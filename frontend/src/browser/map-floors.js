import { floorOrder } from './map-geo.js'

// floorChoices are a map's floors as the floors panel lists them (view-map.js):
// by height, the top floor first, as on a lift's buttons, so the floors under
// the ground (Reserve's Bunkers) come right below it. Each is {id, label}; the
// ground ('' id) is labelled ground.
export function floorChoices(map, ground) {
  return floorOrder(map)
    .reverse()
    .map((id) => ({ id, label: id ? map.layers.find((l) => l.id === id)?.name || id : ground }))
}
