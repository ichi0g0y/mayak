import test from 'node:test'
import assert from 'node:assert/strict'
import { Recent, Shown, RECENT_MS } from './link-recent.js'

test('the Host says its latest detection of each kind again, oldest first, for half an hour', () => {
  const recent = new Recent()
  const task1 = recent.remember({ event: 'browser:task', args: [{ id: 'a' }] }, 1000)
  const map = recent.remember({ event: 'browser:map', args: [{ map: 'woods' }] }, 2000)
  const task2 = recent.remember({ event: 'browser:task', args: [{ id: 'b' }] }, 3000)
  assert.notEqual(task1.id, task2.id)
  // The later task replaces the first; the map, older, goes before it.
  assert.deepEqual(
    recent.due(4000).map((k) => [k.event, k.args[0]]),
    [
      ['browser:map', { map: 'woods' }],
      ['browser:task', { id: 'b' }],
    ],
  )
  assert.equal(recent.due(2000 + RECENT_MS).length, 1)
  assert.equal(recent.due(3000 + RECENT_MS).length, 0)
  assert.equal(map.at, 2000)
})

test('a Client shows each detection once, and one without an id always', () => {
  const shown = new Shown()
  assert.ok(shown.fresh('browser:task', 'x'))
  assert.ok(!shown.fresh('browser:task', 'x'))
  // The same id of another kind, and a new id, are new.
  assert.ok(shown.fresh('browser:map', 'x'))
  assert.ok(shown.fresh('browser:task', 'y'))
  assert.ok(shown.fresh('browser:task', undefined))
  assert.ok(shown.fresh('browser:task', undefined))
})
