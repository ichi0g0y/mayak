import test from 'node:test'
import assert from 'node:assert/strict'
import {
  mapSettingsOf,
  squadColorOf,
  assignColors,
  squadColors,
  autoMap,
  hits,
  segmentDistance,
  validLine,
  findMap,
  floorFor,
  found,
  freshness,
  inBounds,
  markerRotation,
  memberColor,
  players,
  searchTerms,
  shows,
  staleAfter,
  goneAfter,
  tarkovTime,
} from './map-geo.js'

test('the raid clocks run seven times as fast from Moscow time, 12 hours apart', () => {
  assert.equal(tarkovTime(0, true), '03:00:00')
  assert.equal(tarkovTime(0, false), '15:00:00')
  // One real hour is seven raid hours.
  assert.equal(tarkovTime(60 * 60 * 1000, true), '10:00:00')
})

test('the search splits on commas and matches names, items and tasks', () => {
  const terms = searchTerms(' LEDX, bitcoin ,, ')
  assert.deepEqual(terms, ['ledx', 'bitcoin'])
  assert.equal(found({ name: 'Physical Bitcoin' }, terms), true)
  assert.equal(found({ name: 'Loose', detail: { items: [{ name: 'LEDX Skin Transilluminator' }] } }, terms), true)
  assert.equal(found({ name: 'Toolbox', detail: { task: 'Delivery from the Past' } }, ['delivery']), true)
  assert.equal(found({ name: 'Toolbox' }, terms), false)
  assert.equal(found({ name: 'Toolbox' }, []), true)
})

test('a marker shows while one of its layers does; the map clips spawns and loot', () => {
  assert.equal(shows({ layer: 'spawn_pmc' }, ['spawn_pmc']), false)
  assert.equal(shows({ layer: 'loose_ammo', layers: ['loose_ammo', 'loose_keys'] }, ['loose_ammo']), true)
  const map = {
    bounds: [
      [698, -307],
      [-372, 237],
    ],
  }
  assert.equal(inBounds(map, 0, 0), true)
  assert.equal(inBounds(map, 800, 0), false)
})

test("a Host's client is not listed as a player of its own", () => {
  const members = [{ id: '', me: true, viewer: true }, { id: 'a', viewer: true }, { id: 'b' }]
  assert.deepEqual(
    players(members).map((m) => m.id),
    ['b'],
  )
})

const customs = {
  key: 'customs',
  layers: [
    {
      name: '2nd Floor',
      id: 'Second_Floor',
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
    { id: 'Underground_Level', name: 'Underground', svgLayer: 'Underground_Level', extents: [{ height: [-1000, -2] }] },
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

test('a marker is on the floor shown as tarkov.dev decides it', async () => {
  const { onFloor } = await import('./map-geo.js')
  const map = {
    bounds: [
      [698, -307],
      [-372, 237],
    ],
    heightRange: [-1000, 1000],
    layers: [
      {
        id: 'Second_Floor',
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
      { id: 'Underground', extents: [{ height: [-1000, -2] }] },
    ],
  }
  const dorms2 = { x: 200, y: 4, z: 150 }
  const ground = { x: 0, y: 1, z: 0 }
  const extract = { x: 0, y: 1, z: 0, top: 5, bottom: -0.7 }
  // The ground: a marker wholly inside the second floor's dorms is not on it.
  assert.equal(onFloor(map, ground, ''), true)
  assert.equal(onFloor(map, dorms2, ''), false)
  // The second floor: the dorms are, the rest of the map not.
  assert.equal(onFloor(map, dorms2, 'Second_Floor'), true)
  assert.equal(onFloor(map, ground, 'Second_Floor'), false)
  assert.equal(onFloor(map, extract, 'Second_Floor'), false)
  assert.equal(onFloor(map, { x: 0, y: -5, z: 0 }, 'Underground'), true)
})

test('a place name with no height fades on a floor, one with a height where it is', async () => {
  const { labelOff } = await import('./map-geo.js')
  const map = {
    heightRange: [-10, 10],
    bounds: [
      [-100, -100],
      [100, 100],
    ],
    layers: [
      { id: 'second', extents: [{ height: [10, 15] }] },
      { id: 'deck', show: true, extents: [{ height: [20, 30] }] },
    ],
  }
  const street = { x: 0, z: 0, y: 0, top: 1000, bottom: -1000, ground: true }
  const office = { x: 0, z: 0, y: 12, top: 15, bottom: 10 }
  assert.equal(labelOff(map, street, ''), false)
  assert.equal(labelOff(map, street, 'second'), true)
  assert.equal(labelOff(map, street, 'deck'), false)
  assert.equal(labelOff(map, office, 'second'), false)
})

test('floors go from the lowest up with the ground among them', async () => {
  const { floorOrder } = await import('./map-geo.js')
  const floor = (id, ...heights) => ({ id, extents: heights.map((height) => ({ height })) })
  const streets = {
    heightRange: [-6, 10],
    layers: [floor('2', [10, 15]), floor('3', [15, 20]), floor('u', [-10000, -6])],
  }
  assert.deepEqual(floorOrder(streets), ['u', '', '2', '3'])
  const customs = {
    heightRange: [-1000, 1000],
    layers: [floor('2', [2.7, 6.5], [5.7, 1000]), floor('4', [11.2, 54.7]), floor('u', [-1000, 0.5])],
  }
  assert.deepEqual(floorOrder(customs), ['u', '', '2', '4'])
  const groundZero = { heightRange: [-1000, 28], layers: [floor('2', [28, 32.3], [26, 31]), floor('g', [-1000, 21])] }
  assert.deepEqual(floorOrder(groundZero), ['g', '', '2'])
  const interchange = { layers: [floor('2', [25, 34]), floor('3', [34, 1000])] }
  assert.deepEqual(floorOrder(interchange), ['', '2', '3'])
  const reserve = {
    heightRange: [-7, 10000],
    layers: [floor('2', [22.1, 25.7], [-4.3, -2.2]), floor('5', [5, 9.5]), floor('b', [-10000, -7.27], [-11, -4.6])],
  }
  assert.deepEqual(floorOrder(reserve), ['b', '', '2', '5'])
})

test('segmentDistance measures to the nearest point of a segment', () => {
  assert.equal(segmentDistance([5, 3], [0, 0], [10, 0]), 3)
  assert.equal(segmentDistance([-4, 3], [0, 0], [10, 0]), 5)
  assert.equal(segmentDistance([3, 4], [0, 0], [0, 0]), 5)
})

test('hits finds the pointer on a line, its width included', () => {
  const line = [
    [0, 0],
    [100, 0],
    [100, 100],
  ]
  assert.equal(hits(line, 4, [50, 7], 6), true)
  assert.equal(hits(line, 4, [50, 9], 6), false)
  assert.equal(hits(line, 4, [106, 50], 6), true)
  assert.equal(hits([[10, 10]], 10, [18, 10], 3), true)
  assert.equal(hits([[10, 10]], 10, [19, 10], 3), false)
})

test('validLine keeps whole lines only', () => {
  const line = { id: 'a', map: 'customs', floor: '', c: '#fff', w: 5, p: [[1, 2]] }
  assert.equal(validLine(line), true)
  assert.equal(validLine({ ...line, p: [] }), false)
  assert.equal(validLine({ ...line, w: 'wide' }), false)
  assert.equal(validLine({ ...line, floor: undefined }), false)
  assert.equal(validLine(null), false)
})

test('assignColors gives every player a colour of their own', () => {
  const [a, b, c] = squadColors
  const members = [
    { id: 'm2', name: 'Bob', color: a },
    { id: 'm1', me: true, name: 'Ann', color: a },
    { id: 'm3', name: 'Cid', color: 'not a colour' },
    { id: 'm4', name: 'Viewer', viewer: true, color: b },
  ]
  const got = assignColors(members)
  // The lower ID keeps the colour both chose; the other gets a free one.
  assert.equal(got.get('me'), a)
  assert.notEqual(got.get('m2'), a)
  assert.equal(got.has('m4'), false)
  assert.equal(new Set(got.values()).size, 3)
  // The same reports give the same colours, in any order.
  assert.deepEqual([...assignColors([...members].reverse())].sort(), [...got].sort())
  // Ten players take the ten colours.
  const ten = Array.from({ length: 10 }, (_, i) => ({ id: 'id' + i, name: 'same', color: c }))
  assert.equal(new Set(assignColors(ten).values()).size, 10)
  // The member key orders them before the relay ID: reconnected (a new ID),
  // the one who had the colour keeps it.
  const keyed = [
    { id: 'z9', key: 'a'.repeat(32), name: 'Ann', color: a },
    { id: 'a1', key: 'b'.repeat(32), name: 'Bob', color: a },
  ]
  assert.equal(assignColors(keyed).get('z9'), a)
  assert.equal(new Set(assignColors(ten).values()).size, 10)
})

test('the arrow effect in the map settings is one known, in a #rrggbb colour', () => {
  assert.deepEqual([mapSettingsOf({}).markerEffect, mapSettingsOf({}).markerColor], ['none', ''])
  const s = mapSettingsOf({ markerEffect: 'pulse', markerColor: '#FF3B30' })
  assert.deepEqual([s.markerEffect, s.markerColor], ['pulse', '#ff3b30'])
  const bad = mapSettingsOf({ markerEffect: 'spin', markerColor: 'red', markerShape: 'toString' })
  assert.deepEqual([bad.markerEffect, bad.markerColor, bad.markerShape], ['none', '', 'arrow'])
  assert.equal(mapSettingsOf({ markerShape: 'dot' }).markerShape, 'dot')
})

test('a squad colour is any #rrggbb, the first to choose it keeps it', () => {
  assert.equal(squadColorOf('#A1B2C3'), '#a1b2c3')
  assert.equal(squadColorOf('red'), '')
  const given = assignColors([
    { id: '1', key: 'a'.repeat(32), name: 'Ann', color: '#123abc' },
    { id: '2', key: 'b'.repeat(32), name: 'Bob', color: '#123ABC' },
  ])
  assert.equal(given.get('1'), '#123abc')
  assert.notEqual(given.get('2'), '#123abc')
})
