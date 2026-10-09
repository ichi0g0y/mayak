import test from 'node:test'
import assert from 'node:assert/strict'
import { keepMeInView, forgetSeen } from './map-keep.js'

// A Leaflet map's view, from south-west to north-east in [lat, lng] (z, x).
function fakeMap(south, west, north, east) {
  const panned = []
  return {
    panned,
    getBounds: () => ({ contains: ([lat, lng]) => lat >= south && lat <= north && lng >= west && lng <= east }),
    panInside: (point, options) => panned.push({ point, options }),
  }
}
const me = (x, z, at) => ({ me: true, pos: { x, z }, at })

test('a new position outside the view pans to it; one in view, an old one or the setting off do not', () => {
  forgetSeen()
  const map = fakeMap(0, 0, 100, 100)
  // The first draw only takes note of the position.
  assert.equal(keepMeInView(map, me(500, 500, 't1'), true, true), false)
  // The same position again: nothing.
  assert.equal(keepMeInView(map, me(500, 500, 't1'), true, true), false)
  // A new one in view: nothing.
  assert.equal(keepMeInView(map, me(50, 50, 't2'), true, true), false)
  // A new one outside: the map pans to it, with room.
  assert.equal(keepMeInView(map, me(300, 40, 't3'), true, true), true)
  assert.deepEqual(map.panned[0].point, [40, 300])
  assert.ok(map.panned[0].options.padding[0] > 0)
  // The setting off, or on another map: nothing.
  assert.equal(keepMeInView(map, me(300, 400, 't4'), true, false), false)
  assert.equal(keepMeInView(map, me(300, 500, 't5'), false, true), false)
  // No position of your own: nothing.
  assert.equal(keepMeInView(map, undefined, true, true), false)
  assert.equal(map.panned.length, 1)
})
