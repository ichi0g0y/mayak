import { state } from './shell-core.js'
import { assignColors, memberColor, squadKey } from './map-geo.js'

// The squad's colours as this PC shows them (map-geo.js assignColors), for
// the map, the lists and the squad pen. A module of its own, imported by
// view-squad.js, view-map.js and squad-draw.js, so that none of them has to
// import another (squad-draw.js and squad-share.js load in either order).

let colors = { members: null, map: new Map() }
// squadColorMap is the squad's colours by squadKey, worked out again when
// its members change.
export const squadColorMap = () => {
  const members = state.squad?.state?.members || null
  if (colors.members !== members) colors = { members, map: assignColors(members) }
  return colors.map
}
// colorOf is a member's colour: the squad's for them, else (outside a squad,
// or a PC that only watches) the one chosen here or their name's.
export const colorOf = (m) =>
  squadColorMap().get(squadKey(m)) || (m.me && state.squadColor) || memberColor(m.name)
