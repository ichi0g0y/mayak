// The squad map's arithmetic, without Leaflet: which map and floor to show,
// where a player lands and which way their marker points. The geometry is
// tarkov.dev's maps.json as internal/mapdata passes it on.

// findMap returns the map named name (a key or an alias: night-factory,
// ground-zero-21) from maps.
export function findMap(maps, name) {
  if (!name || !Array.isArray(maps)) return null
  return maps.find((m) => m.key === name || m.aliases?.includes(name)) || null
}

// floorFor is the floor (a layer's svgLayer) a player at pos is on, or ''
// for the ground level; the same rule as mapdata.Floor.
export function floorFor(map, pos) {
  if (!map || !pos) return ''
  const between = (v, a, b) => v >= Math.min(a, b) && v <= Math.max(a, b)
  for (const layer of map.layers || []) {
    for (const e of layer.extents || []) {
      if (pos.y < e.height[0] || pos.y >= e.height[1]) continue
      if (!e.bounds?.length) return layer.svgLayer
      if (e.bounds.some((b) => between(pos.x, b[0][0], b[1][0]) && between(pos.z, b[0][1], b[1][1])))
        return layer.svgLayer
    }
  }
  return ''
}

// The members in a raid, with a position.
export const placed = (members) => (members || []).filter((m) => m.map && m.pos)

// autoMap is the map to show when none is chosen: mine while I am in a
// raid, else the one of the member who reported last, else the Host's
// current map.
export function autoMap(members, fallback = '') {
  const inRaid = placed(members)
  const me = inRaid.find((m) => m.me)
  if (me) return me.map
  const latest = inRaid.slice().sort((a, b) => Date.parse(b.at || 0) - Date.parse(a.at || 0))[0]
  return latest?.map || fallback
}

// markerRotation turns a player's facing (degrees, the game's) into the
// marker's rotation on the map, as tarkov.dev does for its own marker.
export function markerRotation(rot, mapRotation = 0) {
  let add = mapRotation || 0
  if (add === 90 || add === 270) add += 180
  return ((((rot || 0) + add) % 360) + 360) % 360
}

// A member's colour follows their name, so it stays when they reconnect.
const colors = [
  '#4cc9f0',
  '#f72585',
  '#ffd166',
  '#06d6a0',
  '#f77f00',
  '#9b5de5',
  '#ef476f',
  '#90be6d',
  '#43aa8b',
  '#e9c46a',
]
export function memberColor(name) {
  let h = 0
  for (const ch of String(name || '')) h = (h * 31 + ch.codePointAt(0)) >>> 0
  return colors[h % colors.length]
}

// A position older than this is shown faded, and one older than gone not at
// all: a player who has not taken a screenshot for a while may be anywhere.
export const staleAfter = 5 * 60 * 1000
export const goneAfter = 30 * 60 * 1000
export function freshness(at, now = Date.now()) {
  const age = now - Date.parse(at || '')
  if (!Number.isFinite(age)) return 'stale'
  return age > goneAfter ? 'gone' : age > staleAfter ? 'stale' : 'fresh'
}
