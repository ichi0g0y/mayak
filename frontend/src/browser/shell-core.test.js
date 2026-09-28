import test from 'node:test'
import assert from 'node:assert/strict'

// In a build, the shell's modules can run before api.js has set window.mayak
// (a chunk shared with the menu window ran first and the shell stayed blank).
// Loading them without it must not throw, and callbacks registered then must
// reach the API once it is there.
test('the shell core and the views load before the shell API exists', async () => {
  const registered = []
  globalThis.window = { addEventListener() {}, matchMedia: () => ({ matches: false, addEventListener() {} }) }
  globalThis.document = {
    addEventListener() {},
    querySelector: () => null,
    querySelectorAll: () => [],
    documentElement: { dataset: {}, lang: 'ja' },
    body: { classList: { add() {}, remove() {} } },
  }
  globalThis.localStorage = { getItem: () => null, setItem() {} }
  await import('./shell-core.js')
  await import('./view-snapnotes.js')
  await import('./view-squad.js')
  // api.js sets the API after them, in the same evaluation.
  window.mayak = { action: async () => null, onState() {}, onKey() {}, onMenu: (fn) => registered.push(fn) }
  await new Promise((resolve) => setTimeout(resolve, 40))
  assert.equal(registered.length, 1)
})
