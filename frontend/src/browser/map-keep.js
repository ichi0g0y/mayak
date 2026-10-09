// Your own position kept in view (view-map.js): when a new one comes on the
// map shown and the map opens on positions (mapSettings.openOnPosition), the
// map pans just enough to bring your arrow in, the zoom kept. An arrow
// already in view stays where it is.

// keepRoom is the room (px) left between the arrow and the view's edge.
export const keepRoom = 80

// seen is the time of your own position last handled; undefined before the
// map's first draw, which takes in the whole map anyway.
let seen

// keepMeInView pans lmap (a Leaflet map) to your arrow when me (your member:
// pos and at) is a new position on the map shown and enabled says the map
// opens on positions. It tells whether it panned.
export function keepMeInView(lmap, me, onShownMap, enabled) {
  const at = me ? String(me.at || '') : ''
  const fresh = seen !== undefined && at !== '' && at !== seen
  seen = at
  if (!fresh || !onShownMap || !enabled) return false
  const point = [me.pos.z, me.pos.x]
  if (lmap.getBounds().contains(point)) return false
  lmap.panInside(point, { padding: [keepRoom, keepRoom] })
  return true
}

// forgetSeen starts over, as before the map's first draw (tests).
export function forgetSeen() {
  seen = undefined
}
