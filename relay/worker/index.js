// The "mayak-relay" Worker: squad rooms. Players who share a squad code join
// the same room over a WebSocket and see each other's last position on
// MAYAK's squad map.
//
// The app derives the room from the code (a SHA-256, 64 hex digits) and
// encrypts every message with a key derived from the code as well, so the
// relay never sees the code, the names or the positions: it forwards opaque
// strings between the members of a room and keeps each member's last one
// for whoever joins later. Nothing is written to storage; a room is gone
// once its last member leaves.
//
// The room uses the WebSocket Hibernation API: while nobody sends anything
// the Durable Object sleeps and costs no duration, and the clients' "ping"
// is answered by the runtime without waking it.

const MAX_MEMBERS = 10;
const MAX_MESSAGE = 4096;
// A member may send this many messages per window before it is dropped.
const RATE_WINDOW_MS = 10_000;
const RATE_MAX = 30;

function text(body, status) {
  return new Response(body, { status, headers: { 'content-type': 'text/plain', 'cache-control': 'no-store' } });
}

function randomID() {
  const bytes = crypto.getRandomValues(new Uint8Array(6));
  return Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('');
}

export class SquadRoom {
  constructor(state) {
    this.state = state;
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
    if (this.state.getWebSockets().length >= MAX_MEMBERS) return text('full', 409);
    const [client, server] = Object.values(new WebSocketPair());
    const member = { id: randomID(), last: '', since: Date.now(), count: 0 };
    this.state.acceptWebSocket(server);
    server.serializeAttachment(member);
    const others = this.members(server).map(({ member: m }) => ({ id: m.id, last: m.last }));
    server.send(JSON.stringify({ t: 'welcome', id: member.id, members: others }));
    this.broadcast(server, { t: 'join', id: member.id });
    return new Response(null, { status: 101, webSocket: client });
  }

  async webSocketMessage(ws, message) {
    const member = ws.deserializeAttachment();
    if (!member) return;
    if (typeof message !== 'string' || message.length > MAX_MESSAGE) {
      ws.close(1009, 'message too big');
      return;
    }
    const now = Date.now();
    if (now - member.since > RATE_WINDOW_MS) {
      member.since = now;
      member.count = 0;
    }
    member.count++;
    if (member.count > RATE_MAX) {
      ws.close(1008, 'too many messages');
      return;
    }
    member.last = message;
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

export default {
  async fetch(request, env) {
    const url = new URL(request.url);
    const match = url.pathname.match(/^\/squad\/([0-9a-f]{64})$/);
    if (!match) return text('not found', 404);
    const room = env.SQUAD.get(env.SQUAD.idFromName(match[1]));
    return room.fetch(request);
  },
};
