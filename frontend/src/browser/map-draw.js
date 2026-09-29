import { state, esc, t, icon, render, clickHandlers } from './shell-core.js'
import { hits, validLine } from './map-geo.js'
import * as squad from './squad-draw.js'

// Your own pen on the map (docs/squad-sharing.md): lines drawn over the map
// in game coordinates, each on the map and floor it was drawn on, kept by Go
// in map-drawings.json (app_mapdraw.go) record by record. view-map.js hands
// over Leaflet, the map and what shows (attach, drawLines); the lines go in
// a pane of their own, and while the pen is up the pointer draws instead of
// dragging the map: the left button draws (or erases), the right or middle
// button and Space + drag move the map, the wheel zooms.
//
// The squad pen (squad-draw.js) is a second pen with the same tools: its
// lines, in each member's squad colour, show to the whole squad, as they are
// drawn, and only their drawer can erase them; one button clears the map's
// squad lines for everyone (undoable for a few seconds). While it is up, the
// pen's place shows to the others; pointing at a squad line names its drawer.

export const palette = ['#ff3b30', '#ffd60a', '#34c759', '#32ade6', '#ffffff', '#111111']
// Widths in screen pixels at the zoom a line is drawn at (its z): at
// another zoom it is as much wider or thinner as the map is bigger or
// smaller (widthAt), so that writing drawn close up keeps its shape seen
// from afar. A line kept before z has its width at every zoom.
export const widths = /** @type {[string, number][]} */ ([
  ['s', 3],
  ['m', 5],
  ['l', 9],
])

// on: a pen is up; mode: which ('mine' or 'squad'); tool: 'pen' or
// 'eraser'; hidden: your own pen's lines are hidden (on this PC's screen only).
export const pen = { on: false, mode: 'mine', tool: 'pen', color: palette[1], size: 'm', hidden: false }
const squadMode = () => pen.on && pen.mode === 'squad'

/** @typedef {{id: string, map: string, floor: string, c: string, w: number, p: number[][]}} Line */
/** @type {Line[]} */
let lines = []
let loading = null
let saveTimer = 0
// Bumped on every change of the lines, for drawLines to know when to redraw.
let revision = 0
// {mode, add: Line[]} or {mode, remove: Line[]}, for undo and redo (this
// session only); mode is the pen's.
const undoStack = []
const redoStack = []

// What shows now (drawLines): the Leaflet module, map, the map's key and the
// floor, and how strong other floors show.
const at = { L: null, map: null, key: '', floor: '', alpha: 0.2 }
const drawn = { map: null, layer: null, key: '', live: null, liveKey: '', cursors: new Map() }

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
  if (loaded)
    saveTimer = setTimeout(
      () =>
        void backend()
          ?.MapDrawingSave(JSON.stringify(lines))
          .catch(() => {}),
      400,
    )
  if (draw) render()
}

const newID = () => Date.now().toString(36) + Math.random().toString(36).slice(2, 8)
// A point in game coordinates, kept to a centimetre.
const round = (v) => Math.round(v * 100) / 100
const here = (l) => l.map === at.key && l.floor === at.floor

function apply(entry, undoing) {
  const add = undoing ? entry.remove : entry.add
  const remove = undoing ? entry.add : entry.remove
  if (entry.mode === 'squad') {
    if (!squad.canDraw()) return
    if (remove?.length) squad.erase(remove)
    if (add?.length) squad.add(add)
    return
  }
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
// Your squad lines on the floor shown.
const squadHere = () => squad.shownLines(at.key).filter((l) => l.floor === at.floor && l.by === squad.myKeyOf())
function clearHere() {
  if (squadMode()) {
    const removed = squadHere()
    if (!removed.length) return
    squad.erase(removed)
    record({ mode: 'squad', remove: removed })
    return
  }
  const removed = lines.filter(here)
  if (!removed.length) return
  lines = lines.filter((l) => !here(l))
  record({ mode: 'mine', remove: removed })
  changed()
}
const linesHere = () => (squadMode() ? squadHere().length > 0 : lines.some(here))

// The squad pen's mark: the squad's icon on this member's squad colour, over
// the pencil in the column of buttons and at the head of its tools.
const squadBadge = (cls) => `<span class="${cls}" style="--c:${squad.myStyle().c}">${icon('squad')}</span>`

// The pens' buttons in the map's column of buttons: yours, and the squad's
// (in a squad, through a relay that takes it).
export function penButton(disabled) {
  const mine = pen.on && pen.mode === 'mine'
  const label = esc(t('mapPen'))
  const button = `<button class="map-rail-button ${mine ? 'selected' : ''}" data-action="mapPen" aria-pressed="${mine}" title="${label}" aria-label="${label}" ${disabled ? 'disabled' : ''}>${icon('pencil')}</button>`
  if (!state.squad) return button
  const s = state.squad.state
  const on = squadMode()
  const why = !s ? 'mapSquadPenJoin' : !s.drawing && s.phase === 'connected' ? 'mapSquadPenRelay' : 'mapSquadPen'
  // Connected to a relay that did not take the pen, a press asks it again.
  const stale = !!s && s.phase === 'connected' && !s.drawing
  const off = disabled || (!squad.canDraw() && !stale)
  return `${button}<button class="map-rail-button map-rail-squad-pen ${on ? 'selected' : ''}" data-action="mapSquadPen" aria-pressed="${on}" title="${esc(t(why))}" aria-label="${esc(t('mapSquadPen'))}" ${off && !on ? 'disabled' : ''}>${icon('pencil')}${squadBadge('map-rail-pen-badge')}</button>`
}

// The notice of a squad clear-all while it can be undone.
export function clearNotice() {
  const c = squad.clearedNow()
  if (!c || c.map !== at.key) return ''
  return `<div class="map-clear-notice" role="status"><span>${esc(t(c.mine ? 'mapClearedMine' : 'mapClearedBy').replace('{name}', c.name))}</span><button data-action="mapPenUnclear">${esc(t('mapClearedUndo'))}</button></div>`
}

// The pen's tools, over the top of the map while it is up.
export function penBar() {
  if (!pen.on) return ''
  const tool = (id, iconName, label) =>
    `<button class="map-draw-tool ${pen.tool === id ? 'on' : ''}" data-action="mapPenTool" data-id="${id}" aria-pressed="${pen.tool === id}" title="${esc(t(label))}" aria-label="${esc(t(label))}">${icon(iconName)}</button>`
  const colors = squadMode()
    ? `<span class="map-draw-own-color" title="${esc(t('mapSquadPenColor'))}">${squadBadge('map-draw-squad-badge')}${esc(squad.myStyle().name)}</span>`
    : palette
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
  const hide = squadMode()
    ? `<button class="map-draw-clear-all" data-action="mapPenClearAll" title="${esc(t('mapPenClearAll'))}" ${squad.shownLines(at.key).length ? '' : 'disabled'}>${esc(t('mapPenClearAllShort'))}</button>`
    : `<button class="${pen.hidden ? 'on' : ''}" data-action="mapPenEye" aria-pressed="${pen.hidden}" title="${esc(t(eye))}" aria-label="${esc(t(eye))}">${icon(pen.hidden ? 'eyeOff' : 'eye')}</button>`
  const name = squadMode() ? 'mapSquadPen' : 'mapPen'
  return `<div class="map-draw-bar ${squadMode() ? 'squad' : ''}" ${squadMode() ? `style="--c:${squad.myStyle().c}"` : ''} role="toolbar" aria-label="${esc(t(name))}" title="${esc(t(squadMode() ? 'mapSquadPenHint' : 'mapPenHint'))}">${tool('pen', 'pencil', 'snapPen')}${tool('eraser', 'eraser', 'snapEraser')}${sep}${colors}${sep}${sizes}${sep}<button data-action="mapPenUndo" title="${esc(t('snapUndo'))}" aria-label="${esc(t('snapUndo'))}" ${undoStack.length ? '' : 'disabled'}>${icon('undo')}</button><button data-action="mapPenRedo" title="${esc(t('snapRedo'))}" aria-label="${esc(t('snapRedo'))}" ${redoStack.length ? '' : 'disabled'}>${icon('redo')}</button>${sep}<button data-action="mapPenClear" title="${esc(t('mapPenClear'))}" aria-label="${esc(t('mapPenClear'))}" ${linesHere() ? '' : 'disabled'}>${icon('trash')}</button>${hide}${sep}<button data-action="${squadMode() ? 'mapSquadPen' : 'mapPen'}" title="${esc(t('mapPenDone'))}" aria-label="${esc(t('mapPenDone'))}">${icon('x')}</button></div>`
}

// pickPen puts up a pen (or puts it down when it is the one up).
function pickPen(mode) {
  const was = squadMode()
  if (pen.on && pen.mode === mode) pen.on = false
  else Object.assign(pen, { on: true, mode })
  if (was && !squadMode()) squad.cursorOff()
  if (pen.on) void load()
  penUsed = Date.now()
}
// The squad pen is put down after IDLE_MS without use (no drawing, no
// pointer over the map): its position stops going, and the squad's room on
// the relay can sleep (a free service: the relay is paid for by the time its
// rooms are awake).
const IDLE_MS = 3 * 60 * 1000
let penUsed = 0
setInterval(() => {
  if (!squadMode() || stroke || Date.now() - penUsed < IDLE_MS) return
  squad.cursorOff()
  pen.on = false
  render()
}, 15000)
clickHandlers.push(async (type, id) => {
  if (type === 'mapSquadPen') {
    if (squadMode() || squad.canDraw()) pickPen('squad')
    else void window.mayakDesktop?.backend?.SquadRecheck?.()
    render()
    return true
  }
  if (!type.startsWith('mapPen')) return false
  if (type === 'mapPen') pickPen('mine')
  else if (type === 'mapPenClearAll') {
    if (squad.canDraw()) squad.clearAll(at.key)
  } else if (type === 'mapPenUnclear') squad.undoClear()
  else if (type === 'mapPenTool') pen.tool = id === 'eraser' ? 'eraser' : 'pen'
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
    drawn.live = L.layerGroup().addTo(map)
    drawn.key = ''
    drawn.liveKey = ''
    drawn.cursors = new Map()
    // The widths follow the zoom (widthAt): drawn again once it settles.
    map.on('zoomend', () => {
      if (at.map === map) drawLines(at.L, map, at.key, at.floor, at.alpha)
    })
  }
  const drawKey = [revision, squad.sq.revision, key, floor, pen.hidden, alpha, zoomKey()].join('|')
  if (drawn.key !== drawKey) {
    drawn.key = drawKey
    drawn.layer.clearLayers()
    const add = (l) => {
      const opacity = l.floor === floor ? 1 : alpha
      if (opacity) shape(L, l, opacity).addTo(drawn.layer)
    }
    if (!pen.hidden) for (const l of lines) if (l.map === key) add(l)
    for (const l of squad.shownLines(key)) add(l)
  }
  drawLive(L, key, floor, alpha)
}
// drawLive draws what the others are drawing now, and where their squad
// pens are.
function drawLive(L, key, floor, alpha) {
  const liveKey = [squad.sq.liveRevision, key, floor, alpha, zoomKey()].join('|')
  if (drawn.liveKey === liveKey) return
  drawn.liveKey = liveKey
  drawn.live.clearLayers()
  for (const l of squad.liveLines(key)) {
    const opacity = l.floor === floor ? 1 : alpha
    if (opacity) shape(L, l, opacity).addTo(drawn.live)
  }
  // The others' pens stay as markers that move (and glide, style.css) to
  // each new place rather than being drawn again.
  const seen = new Set()
  for (const c of squad.cursorsOn(key)) {
    const id = c.name + '|' + c.c
    seen.add(id)
    let marker = drawn.cursors.get(id)
    if (!marker) {
      marker = L.marker([c.z, c.x], {
        pane: 'mapDraw',
        interactive: false,
        keyboard: false,
        icon: L.divIcon({
          className: 'squad-pen-cursor-icon',
          html: `<span class="squad-pen-cursor" style="--c:${esc(c.c)}">${icon('pencil')}<span>${esc(c.name)}</span></span>`,
          iconSize: [0, 0],
        }),
      }).addTo(drawn.map)
      drawn.cursors.set(id, marker)
    } else marker.setLatLng([c.z, c.x])
    marker.setOpacity(c.floor === floor ? 1 : Math.max(alpha, 0.3))
  }
  for (const [id, marker] of drawn.cursors)
    if (!seen.has(id)) {
      marker.remove()
      drawn.cursors.delete(id)
    }
}
// The lines change without a render when the squad's do.
squad.onLines(() => {
  if (at.map && drawn.map === at.map) drawLines(at.L, at.map, at.key, at.floor, at.alpha)
})
// widthAt is a line's width on screen at the map's zoom now: its width
// scaled with the map from its zoom, a pixel at least and eight times its
// width at most.
const zoomKey = () => (at.map ? Math.round(at.map.getZoom() * 10) : 0)
function widthAt(l) {
  if (!Number.isFinite(l.z) || !at.map) return l.w
  return Math.min(l.w * 8, Math.max(1, l.w * 2 ** (at.map.getZoom() - l.z)))
}
function shape(L, l, opacity) {
  const points = l.p.map(([x, z]) => [z, x])
  const w = widthAt(l)
  return points.length === 1
    ? L.circleMarker(points[0], {
        pane: 'mapDraw',
        radius: w / 2,
        stroke: false,
        fillColor: l.c,
        fillOpacity: opacity,
        interactive: false,
      })
    : L.polyline(points, {
        pane: 'mapDraw',
        color: l.c,
        weight: w,
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
let stroke = null // {line, live, last: [x, y], sent, timer} while drawing
let erased = null // the lines erased by this drag
// While the right or middle button moves the map: where the pointer was, and
// how far it went since the map last moved (applied once a frame).
let panning = null // {x, y, dx, dy, frame}
// startPan and panTo move the map the way Leaflet's own dragging does: the
// map's pane shifts ("move") as the pointer goes, and the map settles once
// ("moveend": tiles, markers) when the button is let go. A panBy per pointer
// event settled it every time, which grew slow with more on the map (zoomed
// in) and stuttered.
function startPan(x, y) {
  const map = at.map
  if (!map) return
  // A pan or zoom still animating ends where it was going (stop() alone
  // would settle the map once more).
  if (map._panAnim?._inProgress) map.stop()
  if (map._animatingZoom) map._onZoomTransitionEnd?.()
  panning = { x, y, dx: 0, dy: 0, frame: 0 }
  map.fire('movestart')
}
function panTo(x, y) {
  panning.dx += panning.x - x
  panning.dy += panning.y - y
  panning.x = x
  panning.y = y
  if (panning.frame) return
  const p = panning
  p.frame = requestAnimationFrame(() => {
    p.frame = 0
    if (!at.map || (!p.dx && !p.dy)) return
    at.map._rawPanBy(at.L.point(Math.round(p.dx), Math.round(p.dy)))
    p.dx -= Math.round(p.dx)
    p.dy -= Math.round(p.dy)
    at.map.fire('move')
  })
}
function endPan() {
  if (!panning) return
  const p = panning
  panning = null
  cancelAnimationFrame(p.frame)
  if (at.map && (Math.round(p.dx) || Math.round(p.dy))) at.map._rawPanBy(at.L.point(Math.round(p.dx), Math.round(p.dy)))
  at.map?.fire('move').fire('moveend')
}
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
        startPan(e.clientX, e.clientY)
        e.preventDefault()
        return
      }
      if (e.button !== 0) return
      e.preventDefault()
      el.setPointerCapture(e.pointerId)
      penUsed = Date.now()
      const p = spot(e)
      if (squadMode() && !squad.canDraw()) return
      if (pen.tool === 'eraser') {
        erased = []
        eraseAt(p)
        return
      }
      const ll = at.map.containerPointToLatLng(p)
      const w = widths.find(([k]) => k === pen.size)?.[1] || 5
      const first = [[round(ll.lng), round(ll.lat)]]
      const z = Math.round(at.map.getZoom() * 10) / 10
      const line = squadMode()
        ? {
            id: newID(),
            by: squad.myKeyOf(),
            ...squad.myStyle(),
            map: at.key,
            floor: at.floor,
            w,
            z,
            p: first,
            gen: squad.genOf(at.key),
          }
        : { id: newID(), map: at.key, floor: at.floor, c: pen.color, w, z, p: first }
      stroke = { line, live: shape(at.L, line, 1).addTo(at.map), last: [p.x, p.y], sent: 0, timer: 0 }
      // The squad sees the line as it is drawn: its start, then its new
      // points every twentieth of a second.
      if (squadMode()) {
        squad.begin(line)
        const s = stroke
        s.timer = setInterval(() => flush(s), 50)
      }
    },
    { signal },
  )
  // The map moves with a right or middle drag wherever the pointer goes, and
  // stops once neither button is down, even if its release was missed (let
  // go outside the window).
  window.addEventListener(
    'pointermove',
    (e) => {
      if (!panning) return
      if (!(e.buttons & 6)) return endPan()
      panTo(e.clientX, e.clientY)
    },
    { signal },
  )
  window.addEventListener('pointerup', endPan, { signal })
  // A middle press would start the browser's autoscroll instead.
  el.addEventListener('mousedown', (e) => pen.on && e.button === 1 && e.preventDefault(), { signal })
  el.addEventListener(
    'pointermove',
    (e) => {
      if (panning) return
      // A stroke or an erasing whose release was missed ends here.
      if ((stroke || erased) && !(e.buttons & 1)) return end()
      if (erased) return eraseAt(spot(e))
      if (!stroke) {
        const p = spot(e)
        if (squadMode() && squad.canDraw() && at.map) {
          penUsed = Date.now()
          const ll = at.map.containerPointToLatLng(p)
          squad.cursor(at.key, at.floor, ll.lng, ll.lat)
        }
        nameTip(el, p)
        return
      }
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
  function end() {
    if (erased) {
      if (erased.length) record({ mode: pen.mode, remove: erased })
      erased = null
      render()
    }
    if (!stroke) return
    const s = stroke
    stroke = null
    s.live.remove()
    if (s.timer) {
      clearInterval(s.timer)
      squad.add([s.line])
      record({ mode: 'squad', add: [s.line] })
      render()
      return
    }
    lines = [...lines, s.line]
    record({ mode: 'mine', add: [s.line] })
    changed()
  }
  el.addEventListener('pointerup', end, { signal })
  el.addEventListener('pointercancel', end, { signal })
  el.addEventListener('contextmenu', (e) => pen.on && e.preventDefault(), { signal })
  el.addEventListener(
    'pointerleave',
    () => {
      if (squadMode()) squad.cursorOff()
      nameTip(el, null)
    },
    { signal },
  )
}
function flush(s) {
  const list = s.line.p.slice(s.sent)
  s.sent = s.line.p.length
  squad.points(s.line.id, list)
}
const onScreen = (l) =>
  l.p.map(([x, z]) => {
    const q = at.map.latLngToContainerPoint([z, x])
    return [q.x, q.y]
  })
// nameTip names the drawer of the squad line under the pointer (p), next to
// it; your own pen's lines have no name.
let tipFrame = 0
function nameTip(el, p) {
  cancelAnimationFrame(tipFrame)
  tipFrame = requestAnimationFrame(() => {
    let tip = el.querySelector('.map-line-name')
    const line =
      p && at.map
        ? squad
            .shownLines(at.key)
            .filter((l) => l.floor === at.floor && l.by !== squad.myKeyOf())
            .reverse()
            .find((l) => hits(onScreen(l), widthAt(l), [p.x, p.y], 4))
        : null
    if (!line) {
      tip?.remove()
      return
    }
    if (!tip) {
      tip = document.createElement('div')
      tip.className = 'map-line-name'
      el.appendChild(tip)
    }
    tip.textContent = line.name || '?'
    tip.style.setProperty('--c', line.c)
    tip.style.left = p.x + 14 + 'px'
    tip.style.top = p.y + 14 + 'px'
  })
}
function eraseAt(p) {
  if (squadMode()) {
    const hit = squadHere().filter((l) => hits(onScreen(l), widthAt(l), [p.x, p.y], 6))
    if (!hit.length) return
    squad.erase(hit)
    erased.push(...hit)
    return
  }
  if (pen.hidden) return
  const hit = lines.filter((l) => here(l) && hits(onScreen(l), widthAt(l), [p.x, p.y], 6))
  if (!hit.length) return
  const ids = new Set(hit.map((l) => l.id))
  lines = lines.filter((l) => !ids.has(l.id))
  erased.push(...hit)
  changed()
}

// snapStrokes is the lines of the floor shown as a snap note's strokes, in
// the pixels of the picture taken of the map (scale: its pixels per screen
// pixel), your own pen's and the squad's, for the note's first layer.
function toStrokes(list, scale) {
  const px = (v) => Math.round(v * scale * 10) / 10
  return list.map((l) => ({ c: l.c, w: px(widthAt(l)), p: onScreen(l).map(([x, y]) => [px(x), px(y)]) }))
}
export const snapStrokes = (scale) => (pen.hidden || !at.map ? [] : toStrokes(lines.filter(here), scale))
export const squadSnapStrokes = (scale) =>
  at.map
    ? toStrokes(
        squad.shownLines(at.key).filter((l) => l.floor === at.floor),
        scale,
      )
    : []

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
    if (squadMode()) squad.cursorOff()
    pen.on = false
    render()
  } else if (event.key === ' ' && !space) {
    event.preventDefault()
    space = true
    at.map?.getContainer().classList.add('map-grab')
    render()
  }
})
function releaseSpace() {
  if (!space) return
  space = false
  at.map?.getContainer().classList.remove('map-grab')
  render()
}
document.addEventListener('keyup', (event) => event.key === ' ' && releaseSpace())
window.addEventListener('blur', releaseSpace)
document.addEventListener('visibilitychange', releaseSpace)
