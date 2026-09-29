// The map view's arithmetic, without Leaflet: which map and floor to show,
// which markers the filters let through and on which floor they are, where
// a squadmate lands and which way their marker points, and whether the
// pointer is on a line drawn with the pen (map-draw.js). The geometry is
// tarkov.dev's maps.json as internal/mapdata passes it on.

// findMap returns the map named name (a key or an alias: night-factory,
// ground-zero-21) from maps.
export function findMap(maps, name) {
  if (!name || !Array.isArray(maps)) return null
  return maps.find((m) => m.key === name || m.aliases?.includes(name)) || null
}

// floorFor is the floor (a layer's id) a player at pos is on, or ''
// for the ground level; the same rule as mapdata.Floor.
export function floorFor(map, pos) {
  if (!map || !pos) return ''
  const between = (v, a, b) => v >= Math.min(a, b) && v <= Math.max(a, b)
  for (const layer of map.layers || []) {
    for (const e of layer.extents || []) {
      if (pos.y < e.height[0] || pos.y >= e.height[1]) continue
      if (!e.bounds?.length) return layer.id
      if (e.bounds.some((b) => between(pos.x, b[0][0], b[1][0]) && between(pos.z, b[0][1], b[1][1]))) return layer.id
    }
  }
  return ''
}

// The players of a squad: its members but the PCs that only watch (a
// Host's client, the same player as the Host), this PC always.
export const players = (members) => (members || []).filter((m) => m.me || !m.viewer)

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

// The map's markers come in layers (mapdata.MarkerLayer), grouped as on
// tarkov.dev; every layer shows unless hidden. The icons are tarkov.dev's
// (public/map-icons, MIT).
export const markerGroups = ['extracts', 'hazards', 'landmarks', 'loose', 'containers', 'spawns', 'tasks', 'usable']
export const iconURL = (icon) => `/map-icons/${encodeURIComponent(icon || 'lock')}.png`
// The place names (maps.json labels) are a layer of their own.
export const labelLayer = { key: 'labels', group: 'landmarks', icon: '' }

const layerKey = /^[a-z0-9_-]{1,80}$/
// hiddenOf keeps the layer keys of a saved list of hidden layers.
export const hiddenOf = (saved) =>
  Array.isArray(saved) ? [...new Set(saved.filter((k) => typeof k === 'string' && layerKey.test(k)))].slice(0, 400) : []
// The map settings, as tarkov.dev has them: snipers' spawns always at full
// strength (even on another floor), extracts too, and only the markers of
// tasks being done (with TarkovTracker); and the size of the extracts'
// names and of the place names, in percent.
export const defaultMapSettings = {
  snipers: true,
  extracts: false,
  activeTasks: false,
  extractText: 100,
  labelText: 100,
  subtleLabels: false,
  fade: 20,
  style: 'svg',
  mode: 'auto',
}
// The catalog's game modes (internal/catalog).
export const gameModes = ['auto', 'regular', 'pve', 'pvp-season']
export const textScale = { min: 50, max: 200 }
export const clampText = (v) =>
  Number.isFinite(Number(v)) ? Math.round(Math.min(textScale.max, Math.max(textScale.min, Number(v)))) : 100
export function mapSettingsOf(saved) {
  const s = saved && typeof saved === 'object' ? saved : {}
  return {
    snipers: typeof s.snipers === 'boolean' ? s.snipers : defaultMapSettings.snipers,
    extracts: s.extracts === true,
    activeTasks: s.activeTasks === true,
    extractText: s.extractText === undefined ? 100 : clampText(s.extractText),
    labelText: s.labelText === undefined ? 100 : clampText(s.labelText),
    // Place names drawn faint, so they do not cover the map.
    subtleLabels: s.subtleLabels === true,
    // How strong what is on another floor shows (percent): the ground under a
    // floor shown, and the markers off it. tarkov.dev's is 20.
    fade:
      Number.isFinite(Number(s.fade)) && s.fade !== null && s.fade !== ''
        ? Math.round(Math.min(60, Math.max(0, Number(s.fade))))
        : 20,
    // A map with both drawn from its SVG (tarkov.dev's Abstract) or its
    // tiles (Satellite).
    style: s.style === 'tile' ? 'tile' : 'svg',
    // The game mode whose data the markers show (boss chances differ), or
    // auto: the one played.
    mode: gameModes.includes(s.mode) ? s.mode : 'auto',
  }
}

// shows reports whether a marker is in a layer not hidden (loose loot is in
// every layer of its items' categories).
export const shows = (m, hidden) => (m.layers?.length ? m.layers : [m.layer]).some((l) => !hidden.includes(l))

// The colours tarkov.dev gives extract names and outlines, by layer.
export const nameColors = {
  extract_pmc: '#00e599',
  extract_scav: '#ff7800',
  extract_shared: '#00e4e5',
  extract_transit: '#e53500',
}
export const outlineColor = (layer) =>
  nameColors[layer] || (layer === 'quest_objective' ? '#e5e200' : layer.startsWith('hazard') ? '#ff0000' : '#ffffff')

// markerFloor is the floor a marker is on: an area (an extract, a hazard)
// by the middle of its height, a point by its own.
export function markerFloor(map, m) {
  const y = m.top || m.bottom ? ((m.top || 0) + (m.bottom || 0)) / 2 : m.y
  return floorFor(map, { x: m.x, y, z: m.z })
}

// onExtents is tarkov.dev's markerIsOnLayer: whether a marker (its height
// span, top to bottom, else its y) is within a floor's extents, "full" when
// wholly inside a height range, "partial" when it crosses one, else false.
function onExtents(m, extents) {
  const top = m.top || m.y
  const bottom = m.bottom || m.y
  const between = (v, a, b) => v >= Math.min(a, b) && v <= Math.max(a, b)
  for (const e of extents || []) {
    if (!(top >= e.height[0] && bottom < e.height[1])) continue
    const contained = bottom >= e.height[0] && top <= e.height[1] ? 'full' : 'partial'
    if (!e.bounds?.length) return contained
    if (e.bounds.some((b) => between(m.x, b[0][0], b[1][0]) && between(m.z, b[0][1], b[1][1]))) return contained
  }
  return false
}

// onFloor is tarkov.dev's markerIsOnActiveLayer: a marker wholly inside an
// area of a floor not shown is not on the floor shown; else it is when it is
// within the floor shown (floor, a layer id), or with no floor shown within
// the ground level (the map's height range and bounds).
export function onFloor(map, m, floor) {
  for (const l of map?.layers || [])
    if (l.id !== floor && l.extents?.some((e) => e.bounds?.length) && onExtents(m, l.extents) === 'full') return false
  const shown = floor ? map?.layers?.find((l) => l.id === floor) : null
  if (shown) return !!onExtents(m, shown.extents)
  const ground = [
    {
      height: map?.heightRange || [Number.MIN_SAFE_INTEGER, Number.MAX_SAFE_INTEGER],
      bounds: map?.bounds ? [map.bounds] : undefined,
    },
  ]
  return !!onExtents(m, ground)
}

// inBounds reports whether a place is on the map's picture (tarkov.dev
// leaves spawns and loot outside it out).
export function inBounds(map, x, z) {
  const [[x1, z1], [x2, z2]] = map.bounds
  return x >= Math.min(x1, x2) && x <= Math.max(x1, x2) && z >= Math.min(z1, z2) && z <= Math.max(z1, z2)
}

// searchTerms splits a search as tarkov.dev does: by commas, lower case.
export const searchTerms = (text) =>
  String(text || '')
    .split(',')
    .map((s) => s.trim().toLowerCase())
    .filter(Boolean)
// found reports whether a marker matches a term: its name, its items' or
// its task's.
export function found(m, terms) {
  if (!terms.length) return true
  const texts = [m.name, m.detail?.task, m.detail?.objective, ...(m.detail?.items || []).map((i) => i.name)]
    .filter(Boolean)
    .map((s) => s.toLowerCase())
  return terms.some((term) => texts.some((s) => s.includes(term)))
}

// tarkovTime is the time in a raid, as tarkov.dev shows it: seven times as
// fast as real time, from Moscow time; the right-hand clock is 12 hours on.
export function tarkovTime(now, left) {
  const day = 24 * 60 * 60 * 1000
  const t = new Date((3 * 60 * 60 * 1000 + (left ? 0 : day / 2) + now * 7) % day)
  return t.toISOString().slice(11, 19)
}

// labelOff reports whether a place name is off the floor shown, to be drawn
// faded as the markers there are. A name given no height (ground) names a
// place on the ground: off any floor shown over a faded ground.
export function labelOff(map, l, floor) {
  const shown = floor ? map?.layers?.find((x) => x.id === floor) : null
  if (shown && l.ground) return !shown.show
  return !onFloor(map, { x: l.x, y: l.y, z: l.z, top: l.top, bottom: l.bottom }, floor)
}

// floorOrder is the map's floors from the lowest up, the ground ('') among
// them, for Ctrl + wheel. tarkov.dev lists the floors above the ground
// first and those under it last; a floor is placed by the lowest of its
// heights, one open downwards (a basement, -10000 to -6) under all others
// by its top.
export function floorOrder(map) {
  const open = (v) => v <= -1000
  const keyOf = (extents) => {
    const lows = (extents || []).map((e) => e.height[0]).filter((v) => !open(v))
    if (lows.length) return Math.min(...lows)
    const tops = (extents || []).map((e) => e.height[1])
    return (tops.length ? Math.max(...tops) : 0) - 100000
  }
  const floors = (map?.layers || []).map((l) => ({ id: l.id, key: keyOf(l.extents) }))
  const lowest = Math.min(...floors.map((f) => f.key).filter((k) => k > -1000))
  const [low, high] = map?.heightRange || [-1000, 1000]
  const ground = !open(low) ? low : high < 1000 ? high - 100000 : Number.isFinite(lowest) ? lowest - 0.001 : 0
  // The ground first among equals: sort keeps the order of ties.
  return [{ id: '', key: ground }, ...floors].sort((a, b) => a.key - b.key).map((f) => f.id)
}

// validLine tells whether a line drawn with the pen (map-draw.js) is whole:
// {id, map, floor, c: colour, w: width, p: [[x, z], …]}.
export const validLine = (l) =>
  !!l &&
  typeof l.id === 'string' &&
  typeof l.map === 'string' &&
  typeof l.floor === 'string' &&
  typeof l.c === 'string' &&
  Number.isFinite(l.w) &&
  Array.isArray(l.p) &&
  l.p.length > 0

// Distance from p to the segment a–b, in the same units.
export function segmentDistance(p, a, b) {
  const dx = b[0] - a[0],
    dy = b[1] - a[1]
  const len = dx * dx + dy * dy
  const k = len ? Math.max(0, Math.min(1, ((p[0] - a[0]) * dx + (p[1] - a[1]) * dy) / len)) : 0
  return Math.hypot(p[0] - (a[0] + k * dx), p[1] - (a[1] + k * dy))
}
// hits tells whether a line (its points on screen) passes within radius of p.
export function hits(points, width, p, radius) {
  const reach = radius + width / 2
  if (points.length === 1) return Math.hypot(p[0] - points[0][0], p[1] - points[0][1]) <= reach
  for (let i = 1; i < points.length; i++) if (segmentDistance(p, points[i - 1], points[i]) <= reach) return true
  return false
}
