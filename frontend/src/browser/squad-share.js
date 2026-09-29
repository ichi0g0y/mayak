import { state, render, afterRenderHooks } from './shell-core.js'
import * as squad from './squad-draw.js'

// What is shared with the squad (docs/squad-sharing.md), through the squad's
// room as the squad pen's lines are (squad-draw.js): web pages (a tab's
// address and title), snap notes (the note's picture as drawn, a JPEG) and
// who has just drawn on which map. Each shows in the sidebar's squad section
// and on the squad's page, newest first; one from someone else is unread
// until the page is seen or it is opened. A page shared goes again to those
// who join later; a snap note, being large, reaches only those there when
// it is sent.
//
// A pin ("look here", as on Google Maps) is a point on a map a member
// pressed: it shows on everyone's map in its member's colour, one per
// member (a new one takes the old one's place; its member can put it again
// or drag it, and it moves), until its member takes it out or PIN_MS passes. It also lists as kind 'view'; opening it brings its
// map and floor up around it, at the zoom its member had.
//
// An item: {id, kind: 'tab' | 'snap' | 'draw' | 'view', by (member key), name, c
// (colour), title, url, image, map, at, mine, seen}. The list is this PC's,
// for the squad joined, and goes with it.

export const shares = []
// Someone drew on the map since it last showed: a dot beside "Map" in the
// sidebar (view-map.js liveMapEntry) until it shows.
export let mapFresh = false
const onMap = () => state.tabs.find((tab) => tab.id === state.active)?.kind === 'livemap'
const MAX = 50
// The longest picture taken in (a data URL), and a page's address and title.
const MAX_IMAGE = 1_500_000
let code = ''

const valid = (url) => {
  try {
    const u = new URL(url)
    return (u.protocol === 'https:' || u.protocol === 'http:') && url.length <= 2048
  } catch {
    return false
  }
}
function keep(item) {
  const at = shares.findIndex((s) => s.id === item.id)
  if (at >= 0) shares.splice(at, 1)
  shares.unshift(item)
  shares.splice(MAX)
  render()
}
const text = (v, max) => String(v || '').slice(0, max)

squad.onShell('share', (by, d) => {
  if (typeof d.id !== 'string' || !valid(d.url)) return
  if (shares.some((s) => s.id === d.id)) return
  keep({
    id: d.id,
    kind: 'tab',
    by,
    name: text(d.name, 24),
    c: text(d.c, 7),
    title: text(d.title, 160),
    url: d.url,
    at: Date.now(),
    mine: false,
    seen: false,
  })
})
squad.onShell('snap', (by, d) => {
  if (typeof d.id !== 'string' || typeof d.image !== 'string' || d.image.length > MAX_IMAGE) return
  if (!/^data:image\/(jpeg|png);base64,/.test(d.image)) return
  keep({
    id: d.id,
    kind: 'snap',
    by,
    name: text(d.name, 24),
    c: text(d.c, 7),
    title: text(d.title, 160),
    image: d.image,
    at: Date.now(),
    mine: false,
    seen: false,
  })
})
// The pins on the maps: member key → {id, by, name, c, map, floor, x, z,
// zoom, at, mine, dropAt (when it last came down: it drops in then; a pin
// dragged just moves)}.
export const pins = new Map()
const PIN_MS = 10 * 60 * 1000
function pinIn(by, d) {
  if (!by || typeof d.id !== 'string' || typeof d.map !== 'string') return
  if (![d.x, d.z, d.zoom].every(Number.isFinite)) return
  // A pin handed to someone joining keeps its age.
  const age = Number.isFinite(d.age) ? Math.min(Math.max(0, d.age), PIN_MS) : 0
  if (age >= PIN_MS) return
  const pin = {
    id: d.id,
    by,
    name: text(d.name, 24),
    c: text(d.c, 7),
    map: text(d.map, 60),
    floor: text(d.floor, 60),
    x: d.x,
    z: d.z,
    zoom: d.zoom,
    at: Date.now() - age,
    mine: false,
    dropAt: Date.now(),
  }
  const old = pins.get(by)
  if (d.drag && old?.id === d.id) pin.dropAt = old.dropAt
  pins.set(by, pin)
  // A pin moved moves its row too (read or not as it was).
  const row = shares.find((s) => s.id === d.id)
  if (row) {
    Object.assign(row, { map: pin.map, floor: pin.floor, x: pin.x, z: pin.z, zoom: pin.zoom })
    render()
  } else keep({ ...pin, kind: 'view', seen: false })
}
squad.onShell('pin', pinIn)
// A build before pins shared the view it showed: its centre is the pin.
squad.onShell('view', pinIn)
squad.onShell('unpin', (by, d) => {
  if (pins.get(by)?.id !== d.id) return
  pins.delete(by)
  render()
})
// pinsOn is the pins on a map.
export const pinsOn = (map) => [...pins.values()].filter((p) => p.map === map)
setInterval(() => {
  let gone = false
  for (const [by, p] of pins) if (Date.now() - p.at >= PIN_MS) gone = pins.delete(by) || gone
  if (gone) render()
}, 15000)
// Someone drew a line on a map: one item per member and map, moved up.
squad.onShell('drew', (by, d) => {
  if (!d.map) return
  keep({
    id: `draw:${by}:${d.map}`,
    kind: 'draw',
    by,
    name: text(d.name, 24),
    c: text(d.c, 7),
    map: text(d.map, 60),
    at: Date.now(),
    mine: false,
    seen: true,
  })
  if (!onMap() && !mapFresh) {
    mapFresh = true
    render()
  }
})
afterRenderHooks.push(() => {
  if (mapFresh && onMap()) {
    mapFresh = false
    setTimeout(render)
  }
})
// The shares waiting to be seen (pages and snap notes, not drawings), the
// newest first: counted on the squad's button and beside who shared them.
export const waiting = () => shares.filter((s) => !s.seen && s.kind !== 'draw')
// Those who join get the pages shared here lately, and this member's pin.
squad.onJoin(() => {
  for (const s of shares
    .filter((s) => s.mine && s.kind === 'tab')
    .slice(0, 20)
    .reverse())
    void sendTab(s)
  const pin = pins.get(squad.myKeyOf())
  if (pin) void sendPin(pin)
})

const sendTab = (s) => squad.sendShell({ t: 'share', id: s.id, url: s.url, title: s.title, ...squad.myStyle() })
const sendPin = (p, drag = false) =>
  squad.sendShell({
    t: 'pin',
    id: p.id,
    map: p.map,
    floor: p.floor,
    x: p.x,
    z: p.z,
    zoom: p.zoom,
    age: Date.now() - p.at,
    ...(drag ? { drag: 1 } : {}),
    ...squad.myStyle(),
  })

// canShare tells whether sharing works now (as the squad pen does).
export const canShare = () => squad.canDraw()

// shareTab shares a web page (its address and title). It tells whether it went.
export async function shareTab(url, title) {
  if (!canShare() || !valid(url)) return false
  const item = {
    id: Math.random().toString(36).slice(2, 12),
    kind: 'tab',
    by: squad.myKeyOf(),
    ...squad.myStyle(),
    title: text(title, 160),
    url,
    at: Date.now(),
    mine: true,
    seen: true,
  }
  const ok = await sendTab(item)
  if (ok) keep(item)
  return ok
}

// shareSnap sends a snap note's picture (a JPEG data URL) with its title,
// telling onProgress how far it is (0 to 1). It tells whether it went.
export async function shareSnap(title, image, onProgress) {
  if (!canShare() || image.length > MAX_IMAGE) return false
  const style = squad.myStyle()
  const id = Math.random().toString(36).slice(2, 12)
  const ok = await squad.sendShell({ t: 'snap', id, title: text(title, 160), image, ...style }, onProgress)
  if (ok)
    keep({
      id,
      kind: 'snap',
      by: squad.myKeyOf(),
      ...style,
      title: text(title, 160),
      image,
      at: Date.now(),
      mine: true,
      seen: true,
    })
  return ok
}

// dropPin puts this member's pin at a point of a map (map, floor, x, z),
// with the zoom it was put at, in place of its last one: a new pin, or with
// move the same one moved (put again, or with drag dragged: it does not drop
// in again). It tells whether it went.
export async function dropPin(v, { move = false, drag = false } = {}) {
  if (!canShare()) return false
  const round = (n) => Math.round(n * 100) / 100
  const old = pins.get(squad.myKeyOf())
  const pin = {
    id: move && old ? old.id : Math.random().toString(36).slice(2, 12),
    by: squad.myKeyOf(),
    ...squad.myStyle(),
    map: text(v.map, 60),
    floor: text(v.floor, 60),
    x: round(v.x),
    z: round(v.z),
    zoom: round(v.zoom),
    at: Date.now(),
    mine: true,
    dropAt: drag && old ? old.dropAt : Date.now(),
  }
  pins.set(pin.by, pin)
  const row = shares.find((s) => s.id === pin.id)
  if (row) {
    Object.assign(row, { map: pin.map, floor: pin.floor, x: pin.x, z: pin.z, zoom: pin.zoom })
    render()
  } else keep({ ...pin, kind: 'view', seen: true })
  const ok = await sendPin(pin, drag)
  if (!ok && pins.get(pin.by)?.id === pin.id) {
    pins.delete(pin.by)
    render()
  }
  return ok
}
// removePin takes this member's pin out.
export function removePin() {
  const pin = pins.get(squad.myKeyOf())
  if (!pin) return
  pins.delete(pin.by)
  render()
  void squad.sendShell({ t: 'unpin', id: pin.id })
}

export const unseen = () => shares.filter((s) => !s.seen).length
export function markSeen(id) {
  let changed = false
  for (const s of shares)
    if (!s.seen && (!id || s.id === id)) {
      s.seen = true
      changed = true
    }
  if (changed) render()
}

// The list goes with the squad it was shared in.
afterRenderHooks.push(() => {
  const next = state.squad?.state ? state.squadCode || state.squad.state.code : ''
  if (next === code) return
  code = next
  if (shares.length || pins.size) {
    shares.length = 0
    pins.clear()
    render()
  }
})
