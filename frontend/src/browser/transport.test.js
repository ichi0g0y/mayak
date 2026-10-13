import test from 'node:test'
import assert from 'node:assert/strict'
import { gameInputOf, hostInfoOf, limitsOf, paceWait } from './transport.js'

const { linkSecrets, newLinkKey, validLinkKey } = /** @type {any} */ (globalThis)

test('a pairing key gives both PCs the same room and a key the relay does not see', async () => {
  const key = newLinkKey()
  assert.ok(validLinkKey(key))
  const a = await linkSecrets(key)
  const b = await linkSecrets(key)
  assert.match(a.room, /^[0-9a-f]{64}$/)
  assert.equal(a.room, b.room)
  assert.notEqual((await linkSecrets(newLinkKey())).room, a.room)
  const iv = new Uint8Array(12)
  const sealed = await crypto.subtle.encrypt({ name: 'AES-GCM', iv }, a.aes, new TextEncoder().encode('hi'))
  const opened = await crypto.subtle.decrypt({ name: 'AES-GCM', iv }, b.aes, sealed)
  assert.equal(new TextDecoder().decode(opened), 'hi')
  assert.ok(!validLinkKey('x'))
})

test('the link keeps to five sixths of the limits the relay tells', () => {
  const limits = limitsOf({ rate: 12, rateWindow: 1000, bytes: 600, bytesWindow: 5000, junk: 1 })
  assert.deepEqual(limits, { rate: 12, rateWindow: 1000, bytes: 600, bytesWindow: 5000 })
  assert.deepEqual(limitsOf({ rate: -1 }).rate, 120)
  // Ten messages (five sixths of twelve) go in a second; the eleventh waits
  // until the first is a second old.
  const sent = Array.from({ length: 10 }, (_, i) => ({ at: 1000 + i * 10, n: 10 }))
  assert.equal(paceWait(sent.slice(0, 9), 10, limits, 1100), 0)
  assert.equal(paceWait(sent, 10, limits, 1100), 900)
  // Characters: five sixths of 600 is 500; one of 450 after 100 waits for
  // enough of them to be five seconds old.
  const heavy = [
    { at: 0, n: 60 },
    { at: 100, n: 40 },
  ]
  assert.equal(paceWait(heavy, 400, limits, 200), 0)
  assert.equal(paceWait(heavy, 450, limits, 200), 4800)
  assert.equal(paceWait(heavy, 450, limits, 6000), 0)
})

test('the Host info keeps what is sound: mode, task site, map, raid and position', () => {
  const h = hostInfoOf({
    mode: 'pve',
    questSite: 'japanese-wiki',
    questSites: ['japanese-wiki', 'bogus', 'tarkov-dev'],
    map: 'the-lab',
    raid: true,
    position: { x: 1, y: 2, z: 3, rot: 90, at: 'now' },
  })
  assert.deepEqual(h, {
    mode: 'pve',
    questSite: 'japanese-wiki',
    questSites: ['japanese-wiki', 'tarkov-dev'],
    map: 'the-lab',
    raid: true,
    position: { x: 1, y: 2, z: 3, rot: 90, at: 'now' },
  })
  const bad = hostInfoOf({ mode: 'pvp', questSite: 'host', map: 'The Lab!', raid: 'yes', position: { x: 'a' } })
  assert.deepEqual(bad, { mode: 'pvp', questSite: '', questSites: null, map: '', raid: false, position: null })
})

test('the Host keeps its latest detection of each kind, sent or not', () => {
  const { MayakLink } = /** @type {any} */ (globalThis)
  const link = new MayakLink()
  link.role = 'host'
  assert.equal(link.send({ event: 'browser:task', args: [{ id: 'a' }] }), false)
  assert.equal(link.send({ event: 'browser:task', args: [{ id: 'b' }] }), false)
  assert.equal(link.send({ event: 'browser:map', args: ['customs'] }), false)
  assert.equal(link.send({ event: 'squad:state', args: [] }), false)
  assert.deepEqual(
    link.recent.due().map((k) => [k.event, k.args[0]]),
    [
      ['browser:task', { id: 'b' }],
      ['browser:map', 'customs'],
    ],
  )
})

// A socket that keeps what is sent on it.
const fakeSocket = () => ({
  readyState: WebSocket.OPEN,
  out: /** @type {string[]} */ ([]),
  send(text) {
    this.out.push(text)
  },
})

test('a Client missing a detection gets it at the next hello, and one it got does not open again', async () => {
  const { MayakLink, linkSecrets, newLinkKey } = /** @type {any} */ (globalThis)
  const secrets = await linkSecrets(newLinkKey())
  const host = new MayakLink()
  Object.assign(host, { role: 'host', secrets, ws: fakeSocket() })
  host.peers.set('client', 'mac')
  const got = []
  const client = new MayakLink({ onMessage: (m) => got.push(m.args[0].id) })
  Object.assign(client, { role: 'client', secrets, ws: fakeSocket() })
  client.peers.set('host', 'pc')
  // What the Host said since last time reaches the Client.
  const deliver = async () => {
    await host.outbox
    for (const data of host.ws.out.splice(0))
      await client.received(client.ws, JSON.stringify({ t: 'msg', from: 'host', data }))
  }
  host.send({ event: 'browser:task', args: [{ id: 'a' }] })
  await deliver()
  assert.deepEqual(got, ['a'])
  // Said again at a hello: already shown.
  host.sendRecent()
  await deliver()
  assert.deepEqual(got, ['a'])
  // Sent while the Client could not hear it (lost), then said again.
  host.send({ event: 'browser:task', args: [{ id: 'b' }] })
  await host.outbox
  host.ws.out.length = 0
  host.sendRecent()
  await deliver()
  assert.deepEqual(got, ['a', 'b'])
})

test("the Host's wheel and buttons reach a Client only as a tab step, a tab action or a bounded scroll", () => {
  assert.deepEqual(gameInputOf({ kind: 'tab', step: -1 }), { kind: 'tab', step: -1 })
  assert.equal(gameInputOf({ kind: 'tab', step: 3 }), null)
  assert.deepEqual(gameInputOf({ kind: 'action', action: 'pin' }), { kind: 'action', action: 'pin' })
  assert.equal(gameInputOf({ kind: 'action', action: 'quit' }), null)
  assert.deepEqual(gameInputOf({ kind: 'scroll', dy: 100, smooth: true }), { kind: 'scroll', dy: 100, smooth: true })
  assert.deepEqual(gameInputOf({ kind: 'scroll', dy: -1e9 }), { kind: 'scroll', dy: -5000, smooth: false })
  assert.equal(gameInputOf({ kind: 'scroll', dy: Number.NaN }), null)
  assert.equal(gameInputOf({ kind: 'eval', code: 'x' }), null)
  assert.equal(gameInputOf(null), null)
})
