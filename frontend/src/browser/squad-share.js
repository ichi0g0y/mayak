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
// A member's screenshots are kept apart from the rest: their latest
// SHOTS_EACH, so screenshots coming often do not push pages and notes out.
const SHOTS_EACH = 30
function keep(item) {
  const at = shares.findIndex((s) => s.id === item.id)
  if (at >= 0) shares.splice(at, 1)
  shares.unshift(item)
  let others = 0
  const shots = new Map()
  for (let i = 0; i < shares.length; i++) {
    const s = shares[i]
    const n = s.kind === 'shot' ? (shots.get(s.by) || 0) + 1 : ++others
    if (s.kind === 'shot') shots.set(s.by, n)
    if (n > (s.kind === 'shot' ? SHOTS_EACH : MAX)) shares.splice(i--, 1)
  }
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
// Screenshots (app_squad_shot.go SquadShot): one is named by its file's MD5,
// so the same one is never taken in twice from a member; a small one is
// replaced by its large one when that comes (asked for, or shared by hand).
squad.onShell('shot', (by, d) => {
  if (typeof d.id !== 'string' || !/^[0-9a-f]{32}$/.test(d.hash || '')) return
  if (typeof d.image !== 'string' || d.image.length > MAX_IMAGE || !/^data:image\/jpeg;base64,/.test(d.image)) return
  const had = shares.find((s) => s.kind === 'shot' && s.by === by && s.hash === d.hash)
  if (had) {
    if (d.large && !had.large) {
      Object.assign(had, { image: d.image, large: true })
      if (asking === had.id) asking = ''
      const bubble = bubbles.get(by)
      if (bubble?.hash === d.hash) bubble.large = d.image
      window.dispatchEvent(new CustomEvent('mayak:shot-large', { detail: had.id }))
      render()
    }
    return
  }
  // One on a position shows by its member's arrow on the map (view-map.js).
  if (Number.isFinite(d.x) && Number.isFinite(d.z))
    bubbles.set(by, {
      id: d.id,
      hash: d.hash,
      image: d.image,
      large: d.large ? d.image : '',
      x: d.x,
      z: d.z,
      map: text(d.map, 60),
    })
  keep({
    id: d.id,
    kind: 'shot',
    by,
    name: text(d.name, 24),
    c: text(d.c, 7),
    hash: d.hash,
    map: text(d.map, 60),
    image: d.image,
    large: !!d.large,
    at: Date.now(),
    mine: false,
    seen: false,
  })
})
// A squadmate asks for a large screenshot of this member's: it goes to
// everyone (the relay has no way to one member), once a minute at most.
squad.onShell('shotAsk', (_by, d) => {
  if (d.to !== squad.myKeyOf() || typeof d.hash !== 'string') return
  const sent = sentShots.get(sentKey(d.hash))
  if (sent?.file) void sendShot(sent.file, true)
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

// The screenshots by the arrows on the map: member key (this player's:
// 'me') → {id (its share's), hash, image (small), large (a data URL once
// there), x, z, map, file (this player's: its file)}. One shows while its
// member's arrow is where it was taken (view-map.js drawBubbles).
export const bubbles = new Map()

// The size a large screenshot goes at (squad page): '1080' (in 1920×1080,
// at first), '720' or 'full' (as taken).
const largeSize = () => (['720', 'full'].includes(state.squadShotSize) ? state.squadShotSize : '1080')

// The screenshots this PC sent: the squad and their file's MD5 (sentKey)
// → {id, file, at, largeAt}
// (when the small one and the large one went), kept across restarts so the
// same screenshot is not sent twice (the latest SENT_KEPT).
const SENT_KEPT = 500
const SENT_KEY = 'mayak-squad-shots'
const sentShots = new Map()
const sentKey = (hash) => `${state.squadCode || code}|${hash}`
try {
  for (const [hash, v] of JSON.parse(localStorage.getItem(SENT_KEY) || '[]'))
    if (typeof hash === 'string' && v && typeof v === 'object') sentShots.set(hash, v)
} catch {}
function rememberSent(hash, v) {
  sentShots.delete(hash)
  sentShots.set(hash, v)
  while (sentShots.size > SENT_KEPT) sentShots.delete(sentShots.keys().next().value)
  try {
    localStorage.setItem(SENT_KEY, JSON.stringify([...sentShots]))
  } catch {}
}
// sendShot sends a screenshot of the screenshot folder (its file name),
// small or large, unless that one went already. It tells whether it went.
async function sendShot(file, large, onProgress, made) {
  const go = window.mayakDesktop?.backend
  if (!canShare() || !go?.SquadShot) return false
  let shot = made
  try {
    shot ||= await go.SquadShot(file, large ? largeSize() : 'small')
  } catch {
    return false
  }
  // A small one goes once; a large one again only a minute after it went
  // (for a squadmate who asks and missed it).
  const sent = sentShots.get(sentKey(shot.hash))
  if (!large && sent) return false
  if (large && Date.now() - (sent?.largeAt || 0) < 60_000) return false
  const style = squad.myStyle()
  const id = sent?.id || shot.hash.slice(0, 12)
  const ok = await squad.sendShell(
    {
      t: 'shot',
      id,
      hash: shot.hash,
      map: shot.map || '',
      image: shot.image,
      ...(large ? { large: 1 } : {}),
      ...(!large && shot.positioned ? { x: round(shot.x), z: round(shot.z) } : {}),
      ...style,
    },
    onProgress,
  )
  if (!ok) return false
  rememberSent(sentKey(shot.hash), {
    ...sent,
    id,
    file,
    at: sent?.at || Date.now(),
    ...(large ? { largeAt: Date.now() } : {}),
  })
  const had = shares.find((s) => s.mine && s.hash === shot.hash)
  if (had) Object.assign(had, large ? { image: shot.image, large: true } : {})
  else
    keep({
      id,
      kind: 'shot',
      by: squad.myKeyOf(),
      ...style,
      hash: shot.hash,
      map: shot.map || '',
      image: shot.image,
      large,
      at: Date.now(),
      mine: true,
      seen: true,
    })
  return true
}
// shareShot shares a screenshot by hand, large; one already sent large is
// not sent again. It tells whether it went, and 'dup' for one sent already.
export async function shareShot(file, onProgress) {
  const go = window.mayakDesktop?.backend
  try {
    const { hash } = await go.SquadShotHash(file)
    if (sentShots.get(sentKey(hash))?.largeAt) return 'dup'
  } catch {}
  return sendShot(file, true, onProgress)
}

// askShot asks the member who shared a screenshot for its large picture.
let asking = ''
export const askingShot = () => asking
export async function askShot(id) {
  const s = shares.find((x) => x.id === id && x.kind === 'shot')
  if (!s || s.large || s.mine || !canShare()) return false
  asking = id
  render()
  setTimeout(() => {
    if (asking === id) {
      asking = ''
      render()
    }
  }, 20_000)
  return squad.sendShell({ t: 'shotAsk', hash: s.hash, to: s.by })
}

// A screenshot of the position (its name has one: in a raid every
// screenshot's does, and the analysis found no screen in it, no task or
// item) goes with the position when the player chose so (the map's setting
// shotBubble): small, it shows by their arrow on the map, alone too, and in
// a squad it goes to the squad, at most one every SEND_GAP (the latest taken
// meanwhile in place of those between).
const SEND_GAP = 20_000
const round = (n) => Math.round(n * 100) / 100
window.addEventListener('mayak:screenshot', (event) => void positionShot(/** @type {CustomEvent} */ (event).detail))
let lastFile = ''
async function positionShot(file) {
  const go = window.mayakDesktop?.backend
  if (!state.mapSettings?.shotBubble || !file || file === lastFile || !go?.SquadShotHash) return
  lastFile = file
  let shot
  try {
    shot = await go.SquadShotHash(file)
    // The analysis may take a while.
    for (let i = 0; i < 5 && shot.positioned && !shot.kind; i++) {
      await new Promise((done) => setTimeout(done, 2000))
      shot = await go.SquadShotHash(file)
    }
    if (!shot.positioned || (shot.kind && shot.kind !== 'position')) return
    shot = await go.SquadShot(file, 'small')
  } catch {
    return
  }
  const old = bubbles.get('me')
  bubbles.set('me', {
    id: '',
    hash: shot.hash,
    image: shot.image,
    large: old?.hash === shot.hash ? old.large : '',
    x: shot.x,
    z: shot.z,
    map: shot.map || '',
    file,
  })
  render()
  queueSend(file, shot)
}
let sendLast = 0
let sendNext = null
let sendTimer = 0
function queueSend(file, shot) {
  if (!canShare() || sentShots.has(sentKey(shot.hash))) return
  sendNext = { file, shot }
  if (sendTimer) return
  sendTimer = setTimeout(
    async () => {
      sendTimer = 0
      const next = sendNext
      sendNext = null
      if (!next || !state.mapSettings?.shotBubble) return
      sendLast = Date.now()
      await sendShot(next.file, false, undefined, next.shot)
      if (sendNext) queueSend(sendNext.file, sendNext.shot)
    },
    Math.max(0, sendLast + SEND_GAP - Date.now()),
  )
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
  if (shares.length || pins.size || bubbles.size > (bubbles.has('me') ? 1 : 0)) {
    shares.length = 0
    pins.clear()
    for (const key of bubbles.keys()) if (key !== 'me') bubbles.delete(key)
    render()
  }
})
