import test from 'node:test'
import assert from 'node:assert/strict'
import { floorChoices } from './map-floors.js'

test("a map's floors are listed top first, the ones under the ground right below it", () => {
  // Reserve as tarkov.dev's maps.json has it (2026-10-09): the layers come
  // up from the 2nd floor, the bunkers last.
  const reserve = {
    heightRange: [-7, 10000],
    layers: [
      { id: 'tile-0', name: '2nd Floor', extents: [{ height: [-4.3, -2.2] }] },
      { id: 'tile-1', name: '3rd Floor', extents: [{ height: [-2.2, 2.14] }] },
      { id: 'tile-2', name: '4th Floor', extents: [{ height: [1.6, 4.7] }] },
      { id: 'tile-3', name: '5th Floor', extents: [{ height: [5, 9.5] }] },
      { id: 'Bunkers', name: 'Bunkers', extents: [{ height: [-10000, -7.27] }, { height: [-11, -4.6] }] },
    ],
  }
  assert.deepEqual(
    floorChoices(reserve, 'Ground').map((f) => f.label),
    ['5th Floor', '4th Floor', '3rd Floor', '2nd Floor', 'Ground', 'Bunkers'],
  )
  assert.equal(floorChoices(reserve, 'Ground')[4].id, '')
})
