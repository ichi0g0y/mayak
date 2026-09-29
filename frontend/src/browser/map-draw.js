import { state, esc, t, icon, render, clickHandlers } from './shell-core.js'
import { hits, validLine } from './map-geo.js'

// Your own pen on the map (docs/squad-sharing.md): lines drawn over the map
// in game coordinates, each on the map and floor it was drawn on, kept by Go
// in map-drawings.json (app_mapdraw.go) record by record. view-map.js hands
// over Leaflet, the map and what shows (attach, drawLines); the lines go in
// a pane of their own, and while the pen is up the pointer draws instead of
// dragging the map: the left button draws (or erases), the right or middle
// button and Space + drag move the map, the wheel zooms.

export const palette = ['#ff3b30', '#ffd60a', '#34c759', '#32ade6', '#ffffff', '#111111']
// Widths in screen pixels: a line keeps its width at every zoom.
export const widths = /** @type {[string, number][]} */ ([
  ['s', 3],
  ['m', 5],
  ['l', 9],
])

// on: the pen is up; tool: 'pen' or 'eraser'; hidden: your lines are hidden
// (on this PC's screen only).
export const pen = { on: false, tool: 'pen', color: palette[1], size: 'm', hidden: false }

/** @typedef {{id: string, map: string, floor: string, c: string, w: number, p: number[][]}} Line */
/** @type {Line[]} */
let lines = []
let loading = null
let saveTimer = 0
// Bumped on every change of the lines, for drawLines to know when to redraw.
let revision = 0
// {add: Line[]} or {remove: Line[]}, for undo and redo (this session only).
const undoStack = []
const redoStack = []

// What shows now (drawLines): the Leaflet module, map, the map's key and the
// floor, and how strong other floors show.
const at = { L: null, map: null, key: '', floor: '', alpha: 0.2 }
const drawn = { map: null, layer: null, key: '' }

const backend = () => window.mayakDesktop?.backend

// The lines kept are read once. Until they are, nothing is saved: a save
// is the whole list, and would drop the lines not read yet. A failed read is
// tried again when the map next draws.
let loaded = false
function load() {
  loading ||= (async () => {
    try {
      const list = JSON.parse((await backend().MapDrawingLoad()) || '[]')
      const kept = Array.isArray(list) ? list.filter(validLine) : []
      const ids = new Set(kept.map((l) => l.id))
      lines = [...kept, ...lines.filter((l) => !ids.has(l.id))]
      loaded = true
      revision++
      // Lines drawn before the read are saved with those read.
      if (lines.length !== kept.length) changed(false)
      render()
    } catch {
      setTimeout(() => (loading = null), 5000)
    }
  })()
  return loading
}
function changed(draw = true) {
  if (draw) revision++
  clearTimeout(saveTimer)
  if (loaded) saveTimer = setTimeout(() => void backend()?.MapDrawingSave(JSON.stringify(lines)).catch(() => {}), 400)
  if (draw) render()
}

const newID = () => Date.now().toString(36) + Math.random().toString(36).slice(2, 8)
// A point in game coordinates, kept to a centimetre.
const round = (v) => Math.round(v * 100) / 100
const here = (l) => l.map === at.key && l.floor === at.floor

function apply(entry, undoing) {
  const add = undoing ? entry.remove : entry.add
  const remove = undoing ? entry.add : entry.remove
  if (remove?.length) {
    const ids = new Set(remove.map((l) => l.id))
    lines = lines.filter((l) => !ids.has(l.id))
  }
  if (add?.length) lines = [...lines, ...add]
  changed()
}
function record(entry) {
  undoStack.push(entry)
  if (undoStack.length > 200) undoStack.shift()
  redoStack.length = 0
}
export function undo() {
  const entry = undoStack.pop()
  if (!entry) return
  redoStack.push(entry)
  apply(entry, true)
}
export function redo() {
  const entry = redoStack.pop()
  if (!entry) return
  undoStack.push(entry)
  apply(entry, false)
}
function clearHere() {
  const removed = lines.filter(here)
  if (!removed.length) return
  lines = lines.filter((l) => !here(l))
  record({ remove: removed })
  changed()
}
const linesHere = () => lines.some(here)

// The pen's button in the map's column of buttons.
export function penButton(disabled) {
  const label = esc(t('mapPen'))
  return `<button class="map-rail-button ${pen.on ? 'selected' : ''}" data-action="mapPen" aria-pressed="${pen.on}" title="${label}" aria-label="${label}" ${disabled ? 'disabled' : ''}>${icon('pencil')}</button>`
}

// The pen's tools, over the top of the map while it is up.
export function penBar() {
  if (!pen.on) return ''
  const tool = (id, iconName, label) =>
    `<button class="map-draw-tool ${pen.tool === id ? 'on' : ''}" data-action="mapPenTool" data-id="${id}" aria-pressed="${pen.tool === id}" title="${esc(t(label))}" aria-label="${esc(t(label))}">${icon(iconName)}</button>`
  const colors = palette
    .map(
      (c) =>
        `<button class="map-draw-color ${pen.color === c && pen.tool === 'pen' ? 'on' : ''}" data-action="mapPenColor" data-id="${c}" title="${esc(t('snapColor'))} ${c}" style="--swatch:${c}"></button>`,
    )
    .join('')
  const sizes = widths
    .map(
      ([k, w]) =>
        `<button class="map-draw-size ${pen.size === k ? 'on' : ''}" data-action="mapPenSize" data-id="${k}" title="${esc(t('snapSize' + k.toUpperCase()))}"><span style="--dot:${w + 2}px"></span></button>`,
    )
    .join('')
  const sep = '<span class="map-draw-sep"></span>'
  const eye = pen.hidden ? 'mapPenShow' : 'mapPenHide'
  return `<div class="map-draw-bar" role="toolbar" aria-label="${esc(t('mapPen'))}" title="${esc(t('mapPenHint'))}">${tool('pen', 'pencil', 'snapPen')}${tool('eraser', 'eraser', 'snapEraser')}${sep}${colors}${sep}${sizes}${sep}<button data-action="mapPenUndo" title="${esc(t('snapUndo'))}" aria-label="${esc(t('snapUndo'))}" ${undoStack.length ? '' : 'disabled'}>${icon('undo')}</button><button data-action="mapPenRedo" title="${esc(t('snapRedo'))}" aria-label="${esc(t('snapRedo'))}" ${redoStack.length ? '' : 'disabled'}>${icon('redo')}</button>${sep}<button class="${pen.hidden ? 'on' : ''}" data-action="mapPenEye" aria-pressed="${pen.hidden}" title="${esc(t(eye))}" aria-label="${esc(t(eye))}">${icon(pen.hidden ? 'eyeOff' : 'eye')}</button><button data-action="mapPenClear" title="${esc(t('mapPenClear'))}" aria-label="${esc(t('mapPenClear'))}" ${linesHere() ? '' : 'disabled'}>${icon('trash')}</button>${sep}<button data-action="mapPen" title="${esc(t('mapPenDone'))}" aria-label="${esc(t('mapPenDone'))}">${icon('x')}</button></div>`
}

clickHandlers.push(async (type, id) => {
  if (!type.startsWith('mapPen')) return false
  if (type === 'mapPen') {
    pen.on = !pen.on
    if (pen.on) void load()
  } else if (type === 'mapPenTool') pen.tool = id === 'eraser' ? 'eraser' : 'pen'
  else if (type === 'mapPenColor') {
    pen.color = id
    pen.tool = 'pen'
  } else if (type === 'mapPenSize') pen.size = id
  else if (type === 'mapPenUndo') undo()
  else if (type === 'mapPenRedo') redo()
  else if (type === 'mapPenEye') pen.hidden = !pen.hidden
  else if (type === 'mapPenClear') clearHere()
  else return false
  render()
  return true
})

// drawLines brings the lines on the map up to date: those of the map shown,
// those of other floors as faint as the markers there. view-map.js calls it
// after each render with what shows.
export function drawLines(L, map, key, floor, alpha) {
  Object.assign(at, { L, map, key, floor, alpha })
  void load()
  const el = map.getContainer()
  el.classList.toggle('map-drawing', pen.on)
  el.classList.toggle('map-erasing', pen.on && pen.tool === 'eraser')
  for (const handler of [map.dragging, map.boxZoom, map.doubleClickZoom])
    if (pen.on && !space) handler.disable()
    else handler.enable()
  if (drawn.map !== map) {
    const pane = map.createPane('mapDraw')
    // Over the markers (600) and the squad, under tooltips (650) and popups (700).
    pane.style.zIndex = '640'
    pane.style.pointerEvents = 'none'
    drawn.map = map
    drawn.layer = L.layerGroup().addTo(map)
    drawn.key = ''
  }
  const drawKey = [revision, key, floor, pen.hidden, alpha].join('|')
  if (drawn.key === drawKey) return
  drawn.key = drawKey
  drawn.layer.clearLayers()
  if (pen.hidden) return
  for (const l of lines) {
    if (l.map !== key) continue
    const opacity = l.floor === floor ? 1 : alpha
    if (!opacity) continue
    shape(L, l, opacity).addTo(drawn.layer)
  }
}
function shape(L, l, opacity) {
  const points = l.p.map(([x, z]) => [z, x])
  return points.length === 1
    ? L.circleMarker(points[0], {
        pane: 'mapDraw',
        radius: l.w / 2,
        stroke: false,
        fillColor: l.c,
        fillOpacity: opacity,
        interactive: false,
      })
    : L.polyline(points, {
        pane: 'mapDraw',
        color: l.c,
        weight: l.w,
        opacity,
        lineCap: 'round',
        lineJoin: 'round',
        smoothFactor: 0.5,
        interactive: false,
      })
}

// The pointer while the pen is up. attach runs once per map element (the
// element outlives the maps drawn in it; signal ends its listeners).
let space = false
let stroke = null // {line, live, last: [x, y]} while drawing
let erased = null // the lines erased by this drag
let panning = null // {x, y} while the right or middle button moves the map
export function attach(el, signal) {
  const spot = (e) => {
    const box = el.getBoundingClientRect()
    return at.L.point(e.clientX - box.left, e.clientY - box.top)
  }
  el.addEventListener(
    'pointerdown',
    (e) => {
      if (!pen.on || space || !at.map || e.target.closest?.('.leaflet-control-container')) return
      if (e.button === 1 || e.button === 2) {
        panning = { x: e.clientX, y: e.clientY }
        el.setPointerCapture(e.pointerId)
        e.preventDefault()
        return
      }
      if (e.button !== 0) return
      e.preventDefault()
      el.setPointerCapture(e.pointerId)
      const p = spot(e)
      if (pen.tool === 'eraser') {
        erased = []
        eraseAt(p)
        return
      }
      const ll = at.map.containerPointToLatLng(p)
      const w = widths.find(([k]) => k === pen.size)?.[1] || 5
      const line = { id: newID(), map: at.key, floor: at.floor, c: pen.color, w, p: [[round(ll.lng), round(ll.lat)]] }
      stroke = { line, live: shape(at.L, line, 1).addTo(at.map), last: [p.x, p.y] }
    },
    { signal },
  )
  el.addEventListener(
    'pointermove',
    (e) => {
      if (panning) {
        at.map?.panBy([panning.x - e.clientX, panning.y - e.clientY], { animate: false })
        panning = { x: e.clientX, y: e.clientY }
        return
      }
      if (erased) return eraseAt(spot(e))
      if (!stroke) return
      const p = spot(e)
      if (Math.hypot(p.x - stroke.last[0], p.y - stroke.last[1]) < 2) return
      stroke.last = [p.x, p.y]
      const ll = at.map.containerPointToLatLng(p)
      stroke.line.p.push([round(ll.lng), round(ll.lat)])
      // A dot becomes a line once it has two points.
      if (stroke.line.p.length === 2) {
        stroke.live.remove()
        stroke.live = shape(at.L, stroke.line, 1).addTo(at.map)
      } else stroke.live.addLatLng(ll)
    },
    { signal },
  )
  const end = () => {
    panning = null
    if (erased) {
      if (erased.length) record({ remove: erased })
      erased = null
      render()
    }
    if (!stroke) return
    stroke.live.remove()
    lines = [...lines, stroke.line]
    record({ add: [stroke.line] })
    stroke = null
    changed()
  }
  el.addEventListener('pointerup', end, { signal })
  el.addEventListener('pointercancel', end, { signal })
  el.addEventListener('contextmenu', (e) => pen.on && e.preventDefault(), { signal })
}
function eraseAt(p) {
  if (pen.hidden) return
  const map = at.map
  const hit = lines.filter(
    (l) =>
      here(l) &&
      hits(
        l.p.map(([x, z]) => {
          const q = map.latLngToContainerPoint([z, x])
          return [q.x, q.y]
        }),
        l.w,
        [p.x, p.y],
        6,
      ),
  )
  if (!hit.length) return
  const ids = new Set(hit.map((l) => l.id))
  lines = lines.filter((l) => !ids.has(l.id))
  erased.push(...hit)
  changed()
}

// snapStrokes is your lines of the floor shown as a snap note's strokes, in
// the pixels of the picture taken of the map (scale: its pixels per screen
// pixel), for the note's first layer.
export function snapStrokes(scale) {
  if (pen.hidden || !at.map) return []
  const map = at.map
  const px = (v) => Math.round(v * scale * 10) / 10
  return lines.filter(here).map((l) => ({
    c: l.c,
    w: px(l.w),
    p: l.p.map(([x, z]) => {
      const q = map.latLngToContainerPoint([z, x])
      return [px(q.x), px(q.y)]
    }),
  }))
}

// Keys while the map shows with the pen up: Ctrl+Z / Ctrl+Y (Ctrl+Shift+Z)
// undo and redo, Escape puts the pen down, Space held moves the map.
const onMap = () => state.tabs.find((tab) => tab.id === state.active)?.kind === 'livemap'
document.addEventListener('keydown', (event) => {
  if (!pen.on || !onMap() || event.target.closest?.('input,textarea,select')) return
  const key = event.key.toLowerCase()
  if (event.ctrlKey && !event.altKey && (key === 'z' || key === 'y')) {
    event.preventDefault()
    if (key === 'y' || event.shiftKey) redo()
    else undo()
    render()
  } else if (event.key === 'Escape') {
    pen.on = false
    render()
  } else if (event.key === ' ' && !space) {
    event.preventDefault()
    space = true
    at.map?.getContainer().classList.add('map-grab')
    render()
  }
})
document.addEventListener('keyup', (event) => {
  if (event.key !== ' ' || !space) return
  space = false
  at.map?.getContainer().classList.remove('map-grab')
  render()
})
