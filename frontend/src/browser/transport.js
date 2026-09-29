// The link between a Host and the Clients paired with it (any number): all
// join a room of the relay (relay/worker, /link/<room>) over a WebSocket,
// and everything they say is sealed with AES-GCM under a key only the paired
// PCs have (the pairing's key), so the relay forwards what it cannot read.
// The room is a hash of the key. The Host hears its Clients and each Client
// the Host; the Clients do not hear each other. Runs in the trusted Wails
// shell; external websites use separate native views.
//
// Only what the Host recognizes (tasks, maps, positions, items) goes to the
// Clients, and the squad joined goes both ways (see sendSquad). The link
// reconnects on its own, with a growing pause, until it is stopped.

const LINK_RELAY = 'wss://mayak-relay.ich.sh/link/'
const displayEvents = ['browser:task', 'browser:map', 'browser:position', 'browser:item']
const MAX_MESSAGE = 65536

const utf8 = new TextEncoder()
const b64 = (bytes) =>
  btoa(String.fromCharCode(...bytes))
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=+$/, '')
const unb64 = (text) => Uint8Array.from(atob(text.replace(/-/g, '+').replace(/_/g, '/')), (c) => c.charCodeAt(0))
// A pairing's key: 32 random bytes, base64url.
const newLinkKey = () => b64(crypto.getRandomValues(new Uint8Array(32)))
const validLinkKey = (key) => typeof key === 'string' && /^[A-Za-z0-9_-]{43}$/.test(key)

// linkSecrets derives the room (hex SHA-256 of a label and the key) and the
// AES-GCM key (HKDF-SHA256) from a pairing's key.
async function linkSecrets(key) {
  const raw = unb64(key)
  const label = utf8.encode('mayak-link-room\0')
  const joined = new Uint8Array(label.length + raw.length)
  joined.set(label)
  joined.set(raw, label.length)
  const digest = new Uint8Array(await crypto.subtle.digest('SHA-256', joined))
  const room = Array.from(digest, (b) => b.toString(16).padStart(2, '0')).join('')
  const base = await crypto.subtle.importKey('raw', raw, 'HKDF', false, ['deriveKey'])
  const aes = await crypto.subtle.deriveKey(
    { name: 'HKDF', hash: 'SHA-256', salt: new Uint8Array(0), info: utf8.encode('mayak-link-key v1') },
    base,
    { name: 'AES-GCM', length: 256 },
    false,
    ['encrypt', 'decrypt'],
  )
  return { room, aes }
}
async function seal(aes, value) {
  const iv = crypto.getRandomValues(new Uint8Array(12))
  const sealed = new Uint8Array(await crypto.subtle.encrypt({ name: 'AES-GCM', iv }, aes, utf8.encode(JSON.stringify(value))))
  const out = new Uint8Array(iv.length + sealed.length)
  out.set(iv)
  out.set(sealed, iv.length)
  return b64(out)
}
async function open(aes, text) {
  const bytes = unb64(text)
  const plain = await crypto.subtle.decrypt({ name: 'AES-GCM', iv: bytes.slice(0, 12) }, aes, bytes.slice(12))
  return JSON.parse(new TextDecoder().decode(plain))
}

class MayakLink {
  constructor({
    onState = /** @type {(state:any)=>void} */ (() => {}),
    onMessage = /** @type {(message:any)=>void} */ (() => {}),
    onSquad = /** @type {(squad:{code:string,name:string,initial:boolean,color?:string})=>void} */ (() => {}),
    onUnpair = () => {},
    relay = LINK_RELAY,
  } = {}) {
    this.onState = onState
    this.onMessage = onMessage
    this.onSquad = onSquad
    this.onUnpair = onUnpair
    this.relay = relay
    this.ws = null
    this.role = ''
    this.key = ''
    this.secrets = null
    // The members of the room that proved they have the key (a hello).
    this.peers = new Set()
    this.retry = 0
    this.timer = 0
    this.ping = 0
    this.seen = 0
    this.run = 0
    this.fresh = false
    // Sealing and opening take a moment each: queued, so what is said and
    // heard keeps its order.
    this.outbox = /** @type {Promise<any>} */ (Promise.resolve())
    this.inbox = /** @type {Promise<any>} */ (Promise.resolve())
  }
  get connected() {
    return this.peers.size > 0 && this.ws?.readyState === WebSocket.OPEN
  }
  // start joins the room of key as role ("host" or "client"), again after
  // every drop until stop; fresh marks a Client joining for the first time
  // (by a pairing code), so the Host can drop the code.
  async start(key, role, fresh = false) {
    if (!validLinkKey(key) || !['host', 'client'].includes(role)) throw new Error('invalid-link')
    if (this.key === key && this.role === role && this.ws) return
    this.stop()
    this.fresh = fresh
    const run = ++this.run
    this.key = key
    this.role = role
    this.secrets = await linkSecrets(key)
    if (run === this.run) this.connect(run)
  }
  stop() {
    this.run++
    clearTimeout(this.timer)
    clearInterval(this.ping)
    this.peers.clear()
    const ws = this.ws
    this.ws = null
    this.key = ''
    this.role = ''
    this.retry = 0
    try {
      ws?.close(1000, 'bye')
    } catch {}
  }
  connect(run) {
    if (run !== this.run) return
    this.onState({ phase: 'connecting' })
    const ws = new WebSocket(this.relay + this.secrets.room)
    this.ws = ws
    this.peers.clear()
    ws.onopen = () => {
      if (this.ws !== ws) return
      this.retry = 0
      this.seen = Date.now()
      clearInterval(this.ping)
      // The runtime answers "ping" without waking the room; a link that
      // hears nothing for a while is gone and is made again.
      this.ping = setInterval(() => {
        if (this.ws !== ws) return
        if (Date.now() - this.seen > 75000) ws.close(4000, 'silent')
        else ws.send('ping')
      }, 30000)
      this.onState({ phase: 'waiting' })
    }
    ws.onmessage = (event) => {
      if (this.ws !== ws) return
      this.seen = Date.now()
      if (typeof event.data !== 'string' || event.data === 'pong') return
      const data = event.data
      this.inbox = this.inbox.then(() => this.received(ws, data)).catch(() => {})
    }
    ws.onclose = () => {
      if (this.ws !== ws) return
      this.ws = null
      clearInterval(this.ping)
      this.peers.clear()
      // Again after 2, 4, 8… seconds, a minute at most.
      const wait = Math.min(60000, 2000 * 2 ** this.retry++)
      this.onState({ phase: 'offline' })
      this.timer = setTimeout(() => this.connect(run), wait)
    }
  }
  async received(ws, text) {
    let frame
    try {
      frame = JSON.parse(text)
    } catch {
      return
    }
    if (frame.t === 'welcome') {
      // Whoever is there already hears who joined by our hello.
      if (frame.members?.length) await this.say(ws, { t: 'hello', fresh: this.fresh })
      return
    }
    if (frame.t === 'leave') {
      if (this.peers.delete(frame.id)) this.counted()
      return
    }
    if (frame.t !== 'msg' || typeof frame.data !== 'string' || frame.data.length > MAX_MESSAGE * 2) return
    let message
    try {
      message = await open(this.secrets.aes, frame.data)
    } catch {
      return // Not sealed with this pairing's key.
    }
    if (this.ws !== ws || !message || message.r === this.role || !['host', 'client'].includes(message.r)) return
    if (message.t === 'hello') {
      const known = this.peers.has(frame.from)
      this.peers.add(frame.from)
      if (!message.reply) await this.say(ws, { t: 'hello', reply: true, fresh: this.fresh })
      if (!known) {
        // A Client met once is paired: the next start is not its first.
        if (this.role === 'client') this.fresh = false
        this.onState({ phase: 'connected', peers: this.peers.size, joined: true, newcomer: message.fresh === true })
      }
      return
    }
    if (!this.peers.has(frame.from)) return
    // The Host ending the pairing ends it for every Client; a Client ending
    // it only leaves.
    if (message.t === 'unpair') {
      if (message.r === 'host') this.onUnpair()
      else if (this.peers.delete(frame.from)) this.counted()
      return
    }
    // The squad joined goes both ways, so that either PC can create, join
    // or leave it for both (see sendSquad).
    if (message.event === 'squad:sync') {
      const s = message.args?.[0]
      if (s && typeof s.code === 'string' && s.code.length <= 16)
        this.onSquad({
          code: s.code,
          name: typeof s.name === 'string' ? s.name.slice(0, 24) : '',
          initial: s.initial === true,
          // The squad colour chosen (checked where it is used); a build
          // before squad colours sends none.
          color: typeof s.color === 'string' ? s.color.slice(0, 7) : undefined,
        })
      return
    }
    if (
      this.role === 'client' &&
      message.r === 'host' &&
      displayEvents.includes(message.event) &&
      Array.isArray(message.args) &&
      message.args.length === 1
    )
      this.onMessage({ event: message.event, args: message.args })
  }
  say(ws, value) {
    const aes = this.secrets?.aes
    const next = this.outbox.then(async () => {
      if (!aes || this.ws !== ws || ws.readyState !== WebSocket.OPEN) return false
      const sealed = await seal(aes, { ...value, r: this.role })
      if (this.ws !== ws || ws.readyState !== WebSocket.OPEN) return false
      ws.send(sealed)
      return true
    })
    this.outbox = next.catch(() => false)
    return next
  }
  // counted tells how many PCs are met after one left.
  counted() {
    this.onState(this.peers.size ? { phase: 'connected', peers: this.peers.size } : { phase: 'waiting', peers: 0 })
  }
  // unpair tells the others the pairing is over (from the Host: for all;
  // from a Client: for it), then leaves.
  async unpair() {
    if (this.connected) await this.say(this.ws, { t: 'unpair' }).catch(() => {})
    this.stop()
  }
  // sendSquad tells the other PC the squad joined here (code "" for none),
  // in either direction; initial marks the Host's word when they connect.
  sendSquad(squad) {
    if (!this.connected) return false
    void this.say(this.ws, { event: 'squad:sync', args: [squad] }).catch(() => {})
    return true
  }
  send(message) {
    if (this.role !== 'host' || !this.connected || !displayEvents.includes(message?.event)) return false
    if (JSON.stringify(message).length > MAX_MESSAGE) return false
    void this.say(this.ws, { event: message.event, args: message.args }).catch(() => {})
    return true
  }
}
Object.assign(globalThis, { MayakLink, newLinkKey, validLinkKey, linkSecrets })
