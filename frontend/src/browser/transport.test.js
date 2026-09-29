import test from 'node:test'
import assert from 'node:assert/strict'
import './transport.js'

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
