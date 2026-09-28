import test from 'node:test'
import assert from 'node:assert/strict'
import {
  autoMap,
  findMap,
  floorFor,
  freshness,
  markerRotation,
  memberColor,
  staleAfter,
  goneAfter,
} from './squad-geo.js'

const customs = {
  key: 'customs',
  layers: [
    {
      name: '2nd Floor',
      svgLayer: 'Second_Floor',
      extents: [
        {
          height: [2.7, 6.5],
          bounds: [
            [
              [243, 190],
              [165, 125],
            ],
          ],
        },
      ],
    },
    { name: 'Underground', svgLayer: 'Underground_Level', extents: [{ height: [-1000, -2] }] },
  ],
}
const maps = [customs, { key: 'factory', aliases: ['night-factory'] }]

test('a map is found by its key or an alias', () => {
  assert.equal(findMap(maps, 'customs'), customs)
  assert.equal(findMap(maps, 'night-factory')?.key, 'factory')
  assert.equal(findMap(maps, 'woods'), null)
  assert.equal(findMap(null, 'customs'), null)
})

test('the floor follows the height and the floor’s areas, like mapdata.Floor', () => {
  assert.equal(floorFor(customs, { x: 200, y: 4, z: 150 }), 'Second_Floor')
  assert.equal(floorFor(customs, { x: 0, y: 4, z: 0 }), '')
  assert.equal(floorFor(customs, { x: 0, y: -10, z: 0 }), 'Underground_Level')
  assert.equal(floorFor(customs, null), '')
})

test('the map shown follows me in a raid, else whoever reported last, else the Host', () => {
  const pos = { x: 0, y: 0, z: 0, rot: 0 }
  const members = [
    { id: 'a', name: 'A', map: 'woods', pos, at: '2026-09-29T10:00:00Z' },
    { id: 'b', name: 'B', map: 'shoreline', pos, at: '2026-09-29T10:05:00Z' },
  ]
  assert.equal(autoMap(members, 'customs'), 'shoreline')
  assert.equal(
    autoMap([...members, { id: '', me: true, name: 'Me', map: 'factory', pos, at: '2026-09-29T09:00:00Z' }]),
    'factory',
  )
  assert.equal(autoMap([{ id: '', me: true, name: 'Me' }], 'customs'), 'customs')
})

test('markers turn like tarkov.dev’s own', () => {
  assert.equal(markerRotation(10, 180), 190)
  assert.equal(markerRotation(10, 90), 280)
  assert.equal(markerRotation(-30, 0), 330)
})

test('a position fades, then goes', () => {
  const now = Date.parse('2026-09-29T12:00:00Z')
  const ago = (ms) => new Date(now - ms).toISOString()
  assert.equal(freshness(ago(1000), now), 'fresh')
  assert.equal(freshness(ago(staleAfter + 1000), now), 'stale')
  assert.equal(freshness(ago(goneAfter + 1000), now), 'gone')
  assert.equal(memberColor('Alice'), memberColor('Alice'))
})
