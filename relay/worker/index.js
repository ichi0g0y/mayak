// The "mayak-relay" Worker: squad rooms and links. Players who share a
// squad code join the same room over a WebSocket and see each other's last
// position on MAYAK's squad map (/squad/<room>); a Host and the Clients
// paired with it meet in a link (/link/<room>), which carries
// what the Host recognizes to the Client.
//
// The app derives the room from the code (or the pairing's key: a SHA-256,
// 64 hex digits) and encrypts every message with a key derived from it as
// well, so the relay never sees the code, the names, the positions or what
// is recognized: it forwards opaque strings between the members of a room
// (a squad room keeps each member's last one for whoever joins later, but
// not one marked ephemeral with a leading "~": the squad pen's lines and its
// position, which would push the member's position out).
// A link keeps nothing; a squad room keeps what its members store in it (the
// squad pen's lines, one sealed slot per member: /squad/<room>/store) for a
// week after anyone last joined or stored, then forgets it (an alarm).
//
// The room uses the WebSocket Hibernation API: while nobody sends anything
// the Durable Object sleeps and costs no duration, and the clients' "ping"
// is answered by the runtime without waking it.

// A member may send so many messages per window before it is dropped.
const RATE_WINDOW_MS = 10_000;
// And so many characters per longer window: a picture (a screenshot, a snap
// note) is a hundred messages or more, so heavy things are held to a
// cooldown the message rate alone would not give (told in the welcome; the
// app keeps to five sixths of it).
const BYTES_WINDOW_MS = 60_000;
// A squad: ten players' positions, small and seldom, and the squad pen's
// lines as they are drawn and its position twenty times a second, as
// multiplayer tools send a cursor (the app keeps to five sixths of the rate
// told in the welcome).
const SQUAD = { members: 10, message: 4096, rate: 480, bytes: 4 * 1024 * 1024, storeEvery: 5000, replay: true };
// The relay's version, told in the welcome: 2 takes the squad pen's
// ephemeral messages at the rate above (the app draws only with it).
const VERSION = 2;
// A squad's store: a slot per member (its key's hash), each at most so big,
// so many slots, kept so long after the room was last used.
const STORE = { slot: 512 * 1024, slots: 24, keepMs: 7 * 24 * 60 * 60 * 1000 };
// A link: the Host and its Clients (a few, and room for a dropped socket the
// relay has not noticed yet); an item's details can be large, and a burst of
// recognitions comes quickly.
const LINK = { members: 10, message: 131072, rate: 120, bytes: 16 * 1024 * 1024, replay: false };

// What a member is told of the limits in its welcome, so that the app keeps
// under them (to five sixths) rather than finding them: messages per
// rateWindow, characters per bytesWindow, and for a squad how far apart one
// slot of its store may be written.
const told = (limits) => ({
  rate: limits.rate,
  rateWindow: RATE_WINDOW_MS,
  bytes: limits.bytes,
  bytesWindow: BYTES_WINDOW_MS,
  ...(limits.storeEvery ? { storeEvery: limits.storeEvery } : {}),
});

// A client connects (or reads the store) so often a minute from one address
// at most (the RELAY_CONNECTS rate limit in wrangler.jsonc): a client stuck
// reconnecting is told to wait (429 with Retry-After) before a room wakes.
const RETRY_AFTER_S = 60;

function text(body, status) {
  return new Response(body, { status, headers: { 'content-type': 'text/plain', 'cache-control': 'no-store' } });
}

function randomID() {
  const bytes = crypto.getRandomValues(new Uint8Array(6));
  return Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('');
}

class Room {
  constructor(state, limits) {
    this.state = state;
    this.limits = limits;
    this.state.setWebSocketAutoResponse(new WebSocketRequestResponsePair('ping', 'pong'));
  }

  members(except) {
    const out = [];
    for (const ws of this.state.getWebSockets()) {
      if (ws === except) continue;
      const member = ws.deserializeAttachment();
      if (member) out.push({ ws, member });
    }
    return out;
  }

  broadcast(except, message) {
    const data = JSON.stringify(message);
    for (const { ws } of this.members(except)) {
      try {
        ws.send(data);
      } catch {
        /* A closing socket; its close handler tells the others. */
      }
    }
  }

  async fetch(request) {
    if (request.headers.get('upgrade')?.toLowerCase() !== 'websocket') return text('websocket only', 426);
    if (this.state.getWebSockets().length >= this.limits.members) return text('full', 409);
    const [client, server] = Object.values(new WebSocketPair());
    const member = { id: randomID(), last: '', since: Date.now(), count: 0, bytesSince: Date.now(), bytes: 0 };
    this.state.acceptWebSocket(server);
    server.serializeAttachment(member);
    const others = this.members(server).map(({ member: m }) => ({ id: m.id, last: this.limits.replay ? m.last : '' }));
    server.send(
      JSON.stringify({
        t: 'welcome',
        id: member.id,
        members: others,
        v: VERSION,
        rate: this.limits.rate,
        bytes: this.limits.bytes,
        limits: told(this.limits),
      }),
    );
    this.broadcast(server, { t: 'join', id: member.id });
    return new Response(null, { status: 101, webSocket: client });
  }

  async webSocketMessage(ws, message) {
    const member = ws.deserializeAttachment();
    if (!member) return;
    if (typeof message !== 'string' || message.length > this.limits.message) {
      ws.close(1009, 'message too big');
      return;
    }
    const now = Date.now();
    if (now - member.since > RATE_WINDOW_MS) {
      member.since = now;
      member.count = 0;
    }
    member.count++;
    if (member.count > this.limits.rate) {
      ws.close(1008, 'too many messages');
      return;
    }
    if (this.limits.bytes) {
      // A member from before the byte count starts one now.
      if (!(now - member.bytesSince <= BYTES_WINDOW_MS)) {
        member.bytesSince = now;
        member.bytes = 0;
      }
      member.bytes += message.length;
      if (member.bytes > this.limits.bytes) {
        ws.close(1008, 'too much data');
        return;
      }
    }
    if (this.limits.replay && !message.startsWith('~')) member.last = message;
    ws.serializeAttachment(member);
    this.broadcast(ws, { t: 'msg', from: member.id, data: message });
  }

  async webSocketClose(ws, code) {
    this.left(ws, code);
  }

  async webSocketError(ws) {
    this.left(ws, 1011);
  }

  left(ws, code) {
    const member = ws.deserializeAttachment();
    try {
      ws.close(code === 1005 || code === 1006 ? 1000 : code, 'bye');
    } catch {
      /* Already closed. */
    }
    if (member) this.broadcast(ws, { t: 'leave', id: member.id });
  }
}

export class SquadRoom extends Room {
  constructor(state) {
    super(state, SQUAD);
    // When each slot was last written (while the room is awake).
    this.puts = new Map();
  }

  // Joining keeps the store another week; the store itself is plain HTTP
  // (a slot is too big for a message).
  async fetch(request) {
    const url = new URL(request.url);
    const store = url.pathname.match(/\/store(?:\/([0-9a-f]{64}))?$/);
    if (!store) {
      await this.keep();
      return super.fetch(request);
    }
    const slot = store[1];
    if (request.method === 'GET' && !slot) {
      await this.keep();
      const slots = await this.state.storage.list({ prefix: 'slot:' });
      const out = {};
      for (const [key, value] of slots) out[key.slice(5)] = value;
      return new Response(JSON.stringify(out), { headers: { 'content-type': 'application/json', 'cache-control': 'no-store' } });
    }
    if (request.method === 'PUT' && slot) {
      // A slot is written storeEvery apart at most (the app is told so and
      // waits; the pen's lines are kept a while after they change).
      const now = Date.now();
      const last = this.puts.get(slot) || 0;
      if (now - last < SQUAD.storeEvery) {
        const wait = Math.ceil((SQUAD.storeEvery - (now - last)) / 1000);
        return new Response('slow down', { status: 429, headers: { 'retry-after': String(wait), 'cache-control': 'no-store' } });
      }
      this.puts.set(slot, now);
      const body = await request.text();
      if (body.length > STORE.slot || !/^[A-Za-z0-9_-]*$/.test(body)) return text('bad slot', 400);
      const key = 'slot:' + slot;
      if (body === '') {
        await this.state.storage.delete(key);
      } else {
        if ((await this.state.storage.get(key)) === undefined) {
          const count = (await this.state.storage.list({ prefix: 'slot:', limit: STORE.slots })).size;
          if (count >= STORE.slots) return text('store full', 409);
        }
        await this.state.storage.put(key, body);
      }
      await this.keep();
      return text('ok', 200);
    }
    return text('not found', 404);
  }

  // keep puts the store's end a week away.
  async keep() {
    await this.state.storage.setAlarm(Date.now() + STORE.keepMs);
  }

  async alarm() {
    // Members still here keep it (they store again as they draw).
    if (this.state.getWebSockets().length) return this.keep();
    await this.state.storage.deleteAll();
  }
}

export class LinkRoom extends Room {
  constructor(state) {
    super(state, LINK);
  }
}

export default {
  async fetch(request, env) {
    const url = new URL(request.url);
    const match = url.pathname.match(/^\/(squad|link)\/([0-9a-f]{64})(\/store(\/[0-9a-f]{64})?)?$/);
    if (!match || (match[3] && match[1] !== 'squad')) return text('not found', 404);
    // Joining and reading the store count against the address's connects;
    // writing the store has its own pace (storeEvery).
    if (env.RELAY_CONNECTS && request.method === 'GET') {
      const address = request.headers.get('cf-connecting-ip') || '';
      const { success } = await env.RELAY_CONNECTS.limit({ key: address });
      if (!success)
        return new Response('too many connections', {
          status: 429,
          headers: { 'retry-after': String(RETRY_AFTER_S), 'cache-control': 'no-store' },
        });
    }
    const rooms = match[1] === 'squad' ? env.SQUAD : env.LINK;
    const room = rooms.get(rooms.idFromName(match[2]));
    return room.fetch(request);
  },
};
