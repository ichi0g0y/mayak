import test from 'node:test'
import assert from 'node:assert/strict'
import { hostInfoOf, limitsOf, paceWait } from './transport.js'

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
    map: 'the-lab',
    raid: true,
    position: { x: 1, y: 2, z: 3, rot: 90, at: 'now' },
  })
  assert.deepEqual(h, {
    mode: 'pve',
    questSite: 'japanese-wiki',
    map: 'the-lab',
    raid: true,
    position: { x: 1, y: 2, z: 3, rot: 90, at: 'now' },
  })
  const bad = hostInfoOf({ mode: 'pvp', questSite: 'host', map: 'The Lab!', raid: 'yes', position: { x: 'a' } })
  assert.deepEqual(bad, { mode: 'pvp', questSite: '', map: '', raid: false, position: null })
})
