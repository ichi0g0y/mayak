// The "mayak" Worker: the landing page (static assets from site/dist) plus a
// small pairing relay under /api/pair for MAYAK's "other computers" feature.
//
// Pairing two MAYAK instances over WebRTC needs one exchange of an offer and
// an answer. Instead of copying two long codes between PCs, the Host posts
// its offer here and gets an 8-digit code; the other PC fetches the offer by
// that code and posts its answer, which the Host polls for. A code lives in
// its own Durable Object for ten minutes, holds only the two opaque codes
// the app already produces (validated by the app, size-limited here), and
// is deleted once the connection is made or when it expires. The relay
// never sees the connection itself: after the exchange the PCs talk
// directly, encrypted.
//
// /api/release hands the page the latest GitHub release, cached for five
// minutes, so visitors do not each call api.github.com, whose anonymous
// limit (60 requests an hour per IP) a shared or busy address exhausts.
// An optional GITHUB_TOKEN secret raises that limit for the Worker itself.

const TTL_MS = 10 * 60 * 1000;
const MAX_BODY = 100000;
const CODE_PREFIX = 'MAYAK1.';
const RELEASE_API = 'https://api.github.com/repos/ichi0g0y/mayak/releases/latest';
const RELEASE_TTL = 300;

async function latestRelease(env) {
  const headers = { accept: 'application/vnd.github+json', 'user-agent': 'mayak-site (https://mayak.ich.sh)' };
  if (env.GITHUB_TOKEN) headers.authorization = `Bearer ${env.GITHUB_TOKEN}`;
  const upstream = await fetch(RELEASE_API, { headers, cf: { cacheTtl: RELEASE_TTL, cacheEverything: true } });
  if (!upstream.ok) return json({ error: 'github', status: upstream.status }, 502);
  const data = await upstream.json();
  const body = {
    tag_name: data.tag_name,
    name: data.name,
    html_url: data.html_url,
    published_at: data.published_at,
    assets: (data.assets || []).map((a) => ({ name: a.name, browser_download_url: a.browser_download_url, size: a.size })),
  };
  return new Response(JSON.stringify(body), { status: 200, headers: { ...cors, 'content-type': 'application/json', 'cache-control': `public, max-age=${RELEASE_TTL}` } });
}

const cors = {
  'access-control-allow-origin': '*',
  'access-control-allow-methods': 'GET, POST, PUT, DELETE, OPTIONS',
  'access-control-allow-headers': 'content-type',
  'access-control-max-age': '86400',
  'cache-control': 'no-store',
};

function json(body, status = 200) {
  return new Response(JSON.stringify(body), { status, headers: { ...cors, 'content-type': 'application/json' } });
}

function empty(status = 204) {
  return new Response(null, { status, headers: cors });
}

/** A pairing code that looks like one the app made; anything else is refused. */
function pairingCode(value) {
  return typeof value === 'string' && value.length <= MAX_BODY && value.startsWith(CODE_PREFIX) && /^[A-Za-z0-9_.-]+$/.test(value);
}

async function readBody(request) {
  const length = Number(request.headers.get('content-length') || 0);
  if (length > MAX_BODY) return null;
  const text = await request.text();
  if (text.length > MAX_BODY) return null;
  try {
    return JSON.parse(text);
  } catch {
    return null;
  }
}

export class PairRoom {
  constructor(state) {
    this.state = state;
  }

  async fetch(request) {
    const url = new URL(request.url);
    const wantsAnswer = url.pathname.endsWith('/answer');
    const now = Date.now();
    const stored = await this.state.storage.get('room');
    const room = stored && stored.expiresAt > now ? stored : null;

    if (request.method === 'POST') {
      if (room) return json({ error: 'taken' }, 409);
      const body = await readBody(request);
      if (!body || !pairingCode(body.invite)) return json({ error: 'bad-invite' }, 400);
      const expiresAt = now + TTL_MS;
      await this.state.storage.put('room', { invite: body.invite, answer: '', expiresAt });
      await this.state.storage.setAlarm(expiresAt);
      return json({ expiresAt }, 201);
    }
    if (!room) return json({ error: 'not-found' }, 404);
    switch (request.method) {
      case 'GET':
        if (wantsAnswer) return room.answer ? json({ answer: room.answer }) : empty(204);
        return json({ invite: room.invite, expiresAt: room.expiresAt });
      case 'PUT': {
        if (room.answer) return json({ error: 'answered' }, 409);
        const body = await readBody(request);
        if (!body || !pairingCode(body.answer)) return json({ error: 'bad-answer' }, 400);
        await this.state.storage.put('room', { ...room, answer: body.answer });
        return empty(204);
      }
      case 'DELETE':
        await this.state.storage.deleteAll();
        return empty(204);
      default:
        return json({ error: 'method' }, 405);
    }
  }

  async alarm() {
    await this.state.storage.deleteAll();
  }
}

function randomCode() {
  const n = crypto.getRandomValues(new Uint32Array(1))[0] % 100000000;
  return String(n).padStart(8, '0');
}

export default {
  async fetch(request, env) {
    const url = new URL(request.url);
    if (!url.pathname.startsWith('/api/')) return env.ASSETS.fetch(request);
    if (request.method === 'OPTIONS') return empty(204);
    if (url.pathname === '/api/release') return request.method === 'GET' ? latestRelease(env) : json({ error: 'method' }, 405);
    const match = url.pathname.match(/^\/api\/pair(?:\/(\d{8})(\/answer)?)?$/);
    if (!match) return json({ error: 'not-found' }, 404);
    const [, code] = match;
    if (!code) {
      if (request.method !== 'POST') return json({ error: 'method' }, 405);
      // A fresh code: the room must be free, so a collision picks another.
      const body = await request.text();
      for (let attempt = 0; attempt < 5; attempt++) {
        const candidate = randomCode();
        const room = env.PAIR.get(env.PAIR.idFromName(candidate));
        const result = await room.fetch(new Request(`${url.origin}/api/pair/${candidate}`, { method: 'POST', headers: { 'content-type': 'application/json', 'content-length': String(body.length) }, body }));
        if (result.status === 409) continue;
        if (result.status !== 201) return result;
        const data = await result.json();
        return json({ code: candidate, ...data }, 201);
      }
      return json({ error: 'busy' }, 503);
    }
    const room = env.PAIR.get(env.PAIR.idFromName(code));
    return room.fetch(request);
  },
};
