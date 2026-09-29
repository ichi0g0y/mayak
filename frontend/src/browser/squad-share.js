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
// A map view shared ("look here": its map, floor, centre and zoom) comes
// the same way, as kind 'view', and brings that view up when opened.
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
  keep({ id: d.id, kind: 'tab', by, name: text(d.name, 24), c: text(d.c, 7), title: text(d.title, 160), url: d.url, at: Date.now(), mine: false, seen: false })
})
squad.onShell('snap', (by, d) => {
  if (typeof d.id !== 'string' || typeof d.image !== 'string' || d.image.length > MAX_IMAGE) return
  if (!/^data:image\/(jpeg|png);base64,/.test(d.image)) return
  keep({ id: d.id, kind: 'snap', by, name: text(d.name, 24), c: text(d.c, 7), title: text(d.title, 160), image: d.image, at: Date.now(), mine: false, seen: false })
})
squad.onShell('view', (by, d) => {
  if (typeof d.id !== 'string' || typeof d.map !== 'string' || shares.some((s) => s.id === d.id)) return
  if (![d.x, d.z, d.zoom].every(Number.isFinite)) return
  keep({ id: d.id, kind: 'view', by, name: text(d.name, 24), c: text(d.c, 7), map: text(d.map, 60), floor: text(d.floor, 60), x: d.x, z: d.z, zoom: d.zoom, at: Date.now(), mine: false, seen: false })
})
// Someone drew a line on a map: one item per member and map, moved up.
squad.onShell('drew', (by, d) => {
  if (!d.map) return
  keep({ id: `draw:${by}:${d.map}`, kind: 'draw', by, name: text(d.name, 24), c: text(d.c, 7), map: text(d.map, 60), at: Date.now(), mine: false, seen: true })
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
// Those who join get the pages shared here lately.
squad.onJoin(() => {
  for (const s of shares.filter((s) => s.mine && s.kind === 'tab').slice(0, 20).reverse()) void sendTab(s)
})

const sendTab = (s) => squad.sendShell({ t: 'share', id: s.id, url: s.url, title: s.title, ...squad.myStyle() })

// canShare tells whether sharing works now (as the squad pen does).
export const canShare = () => squad.canDraw()

// shareTab shares a web page (its address and title). It tells whether it went.
export async function shareTab(url, title) {
  if (!canShare() || !valid(url)) return false
  const item = { id: Math.random().toString(36).slice(2, 12), kind: 'tab', by: squad.myKeyOf(), ...squad.myStyle(), title: text(title, 160), url, at: Date.now(), mine: true, seen: true }
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
  if (ok) keep({ id, kind: 'snap', by: squad.myKeyOf(), ...style, title: text(title, 160), image, at: Date.now(), mine: true, seen: true })
  return ok
}

// shareView shares the map view shown (map, floor, centre, zoom): "look
// here". It tells whether it went.
export async function shareView(v) {
  if (!canShare()) return false
  const style = squad.myStyle()
  const id = Math.random().toString(36).slice(2, 12)
  const round = (n) => Math.round(n * 100) / 100
  const view = { map: text(v.map, 60), floor: text(v.floor, 60), x: round(v.x), z: round(v.z), zoom: round(v.zoom) }
  const ok = await squad.sendShell({ t: 'view', id, ...view, ...style })
  if (ok) keep({ id, kind: 'view', by: squad.myKeyOf(), ...style, ...view, at: Date.now(), mine: true, seen: true })
  return ok
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
  if (shares.length) {
    shares.length = 0
    render()
  }
})
