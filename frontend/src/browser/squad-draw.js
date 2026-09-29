import { state, render, afterRenderHooks } from './shell-core.js'
import { validLine } from './map-geo.js'
import { colorOf } from './squad-colors.js'

// The squad pen's lines (docs/squad-sharing.md): what every member of the
// squad draws on the map, sent through the squad's room as messages of the
// shell's (app_squad.go SquadSend, squad:message), sealed with the squad's
// code like the positions. map-draw.js draws them and holds the pen; this
// module keeps them in step with the others.
//
// A line is {id, by: its drawer's member key, name, c: colour, map, floor, w,
// p: [[x, z], …], gen}. Only its drawer changes or removes it. A clear-all
// raises the map's generation (gens): a line of an older one no longer shows.
// The relay keeps nothing, so the members hand the lines over themselves:
// when someone joins, each member sends its own lines, and the member with
// the lowest relay ID also those of members who left. A member back after a
// drop (or a restart: its own lines are kept on this PC for a day, per
// squad) first waits for the others' lines and generations, then sends its
// own again, without those cleared meanwhile.
//
// Messages ({t, …}): b (a line begins), p (its new points; droppable), l
// (whole lines, with the generations), e (lines erased), x (a map cleared),
// u (the clear undone), c (the pen's position; droppable), part (a piece of
// a message too long for one). Messages of other kinds (squad-share.js) go
// to the handlers given with onShell.

const backend = () => window.mayakDesktop?.backend
// The longest message sent as one (the relay takes 4096 characters of the
// sealed text, about 4/3 of this plus a little).
const LIMIT = 2800
const PART = 2600
// How long a clear can be undone, a line being drawn waits for its next
// points, the pen's position shows unmoved, a message in parts waits.
const UNDO_MS = 8000
const LIVE_MS = 5000
const CURSOR_MS = 4000
const PART_MS = 30000
// The most lines kept for a squad (what others may fill it with).
const MAX_LINES = 3000

export const sq = {
  lines: new Map(),
  gens: /** @type {Record<string, number>} */ ({}),
  // Lines being drawn by others: id → {line, at}.
  live: new Map(),
  // The squad pen's position of others: member key → {name, c, map, floor, x, z, at}.
  cursors: new Map(),
  // The last clear-all, while it can be undone: {map, gen, prev, name, at, mine}.
  cleared: null,
  // Bumped when the lines change, and when what is being drawn moves.
  revision: 0,
  liveRevision: 0,
}
let myKey = ''
let code = ''
let session = ''
let known = new Set()
let helloTimer = 0
let greeted = false
const parts = new Map()
let listeners = []
// The functions map-draw.js gives to redraw the lines (without a render).
export const onLines = (fn) => listeners.push(fn)
const notify = (live = false) => {
  if (live) sq.liveRevision++
  else sq.revision++
  for (const fn of listeners) fn()
}

const me = () => state.squad?.state?.members?.find((m) => m.me)
// canDraw tells whether the squad pen can be used: in a squad, connected
// through a relay that takes its messages, with a member key.
export const canDraw = () => {
  const s = state.squad?.state
  return !!(s && s.phase === 'connected' && s.drawing && myKey)
}
// The pen's colour and name: this member's squad colour and display name.
export const myStyle = () => {
  const m = me()
  return { c: m ? colorOf(m) : state.squadColor || '#ffd166', name: m?.name || state.squadName || '?' }
}
export const myKeyOf = () => myKey
export const genOf = (map) => sq.gens[map] || 0

function send(obj, droppable = false) {
  const text = JSON.stringify(obj)
  const go = backend()
  if (!go?.SquadSend) return
  if (text.length <= LIMIT) {
    void go.SquadSend(text, droppable).catch(() => {})
    return
  }
  if (!droppable) void sendParts(text)
}
// sendParts sends a message too long for one in pieces, one after another
// (each waits for room under the relay's limit), telling onProgress how far
// it is (0 to 1). A piece is as long as fits once escaped in its message.
async function sendParts(text, onProgress = (_done) => {}) {
  const go = backend()
  const pieces = []
  for (let at = 0; at < text.length; ) {
    let size = PART
    while (size > 200 && JSON.stringify(text.slice(at, at + size)).length > PART + 100) size -= 200
    pieces.push(text.slice(at, at + size))
    at += size
  }
  const id = Math.random().toString(36).slice(2, 10)
  const n = pieces.length
  for (let i = 0; i < n; i++) {
    if (!(await go.SquadSend(JSON.stringify({ t: 'part', id, i, n, s: pieces[i] }), false).catch(() => false))) return false
    onProgress((i + 1) / n)
  }
  return true
}
// sendShell sends a message of another kind (squad-share.js); a long one
// goes in pieces, with its progress told. It tells whether it went.
export async function sendShell(obj, onProgress) {
  const text = JSON.stringify(obj)
  if (!canDraw()) return false
  if (text.length <= LIMIT) return !!(await backend()?.SquadSend(text, false).catch(() => false))
  return sendParts(text, onProgress)
}
const handlers = {}
export const onShell = (type, fn) => (handlers[type] = fn)
// Functions run when members join (after this member's lines went to them).
const joinHooks = []
export const onJoin = (fn) => joinHooks.push(fn)
// sendLines sends lines in as few messages as fit, each with the generations;
// fresh marks lines just drawn (not handed over).
function sendLines(list, fresh = false) {
  let batch = []
  let size = 0
  const flush = () => {
    if (batch.length) send(fresh ? { t: 'l', gens: sq.gens, lines: batch, new: 1 } : { t: 'l', gens: sq.gens, lines: batch })
    batch = []
    size = 0
  }
  for (const line of list) {
    const n = JSON.stringify(line).length
    if (batch.length && size + n > LIMIT - 200) flush()
    batch.push(line)
    size += n
  }
  flush()
}
const own = () => [...sq.lines.values()].filter((l) => l.by === myKey)
const shown = (l) => (l.gen || 0) >= genOf(l.map)

// The lines this member draws are kept on this PC, per squad, for a day.
let saveTimer = 0
function saveOwn() {
  if (!code) return
  clearTimeout(saveTimer)
  const squadCode = code
  saveTimer = setTimeout(
    () => void backend()?.SquadLinesSave?.(squadCode, JSON.stringify({ gens: sq.gens, lines: own() })).catch(() => {}),
    500,
  )
}
async function loadOwn(squadCode) {
  try {
    const kept = JSON.parse((await backend()?.SquadLinesLoad?.(squadCode)) || '{}')
    if (squadCode !== code) return
    mergeGens(kept.gens)
    for (const l of Array.isArray(kept.lines) ? kept.lines : []) if (valid(l) && l.by === myKey) sq.lines.set(l.id, l)
    notify()
  } catch {}
}

const valid = (l) =>
  validLine(l) &&
  typeof l.by === 'string' &&
  typeof l.name === 'string' &&
  Number.isInteger(l.gen ?? 0) &&
  (l.z === undefined || Number.isFinite(l.z)) &&
  l.p.length <= 20000 &&
  l.p.every((q) => Array.isArray(q) && Number.isFinite(q[0]) && Number.isFinite(q[1]))
function mergeGens(gens) {
  if (!gens || typeof gens !== 'object') return
  for (const [map, g] of Object.entries(gens))
    if (typeof map === 'string' && Number.isInteger(g) && g > genOf(map)) sq.gens[map] = g
}

function reset() {
  sq.lines.clear()
  sq.live.clear()
  sq.cursors.clear()
  sq.gens = {}
  sq.cleared = null
  session = ''
  known = new Set()
  greeted = false
  clearTimeout(helloTimer)
  parts.clear()
  notify()
}

// hello is this member's word after connecting: its own lines, without
// those of a map cleared since they were drawn.
function hello() {
  greeted = true
  for (const l of own()) if (!shown(l)) sq.lines.delete(l.id)
  sendLines(own())
  saveOwn()
  notify()
}
// shareWith hands the lines to members who just joined: this member's own,
// and those of members no longer here when this member has the lowest ID.
function shareWith() {
  const s = state.squad.state
  const ids = s.members.map((m) => m.id).filter(Boolean)
  const lowest = ids.length && ids.every((id) => id >= me().id)
  const keys = new Set(s.members.map((m) => m.key).filter(Boolean))
  const list = [...sq.lines.values()].filter(
    (l) => shown(l) && (l.by === myKey || (lowest && !keys.has(l.by) && l.by !== myKey)),
  )
  sendLines(list)
}

// follow keeps up with the squad after each render: a squad joined or left,
// this PC connecting again, members joining.
function follow() {
  const go = backend()
  if (go && !listening) {
    listening = true
    window.mayakDesktop.on('squad:message', (m) => receive(String(m?.from || ''), m?.data))
    void go
      .SquadMemberKey?.()
      .then((k) => {
        myKey = String(k || '')
        if (code) void loadOwn(code)
        render()
      })
      .catch(() => {})
  }
  const s = state.squad?.state
  const next = s ? state.squadCode || s.code : ''
  if (next !== code) {
    reset()
    code = next
    if (code && myKey) void loadOwn(code)
  }
  if (!s || s.phase !== 'connected' || !s.drawing || !myKey) {
    session = ''
    return
  }
  const mine = me()
  if (mine.id !== session) {
    session = mine.id
    known = new Set(s.members.filter((m) => !m.me).map((m) => m.id))
    greeted = false
    clearTimeout(helloTimer)
    // Others send their lines when they see this member; the generations
    // come with them.
    if (known.size) helloTimer = setTimeout(hello, 2500)
    else hello()
    return
  }
  const fresh = s.members.filter((m) => !m.me && !known.has(m.id))
  for (const m of fresh) known.add(m.id)
  if (fresh.length && greeted) {
    shareWith()
    for (const fn of joinHooks) fn()
  }
  // The pen's position of a member who left goes.
  const keys = new Set(s.members.map((m) => m.key))
  for (const key of sq.cursors.keys()) if (!keys.has(key)) sq.cursors.delete(key)
}
let listening = false
afterRenderHooks.push(follow)

function senderKey(from) {
  return state.squad?.state?.members?.find((m) => m.id === from)?.key || ''
}
function receive(from, data) {
  if (!data || typeof data !== 'object' || !code) return
  if (data.t === 'part') {
    if (!Number.isInteger(data.i) || !Number.isInteger(data.n) || data.n > 1000 || typeof data.s !== 'string') return
    const key = from + ':' + data.id
    const got = parts.get(key) || { n: data.n, pieces: [], at: Date.now() }
    got.pieces[data.i] = data.s
    parts.set(key, got)
    if (got.pieces.filter((x) => x !== undefined).length === got.n) {
      parts.delete(key)
      try {
        receive(from, JSON.parse(got.pieces.join('')))
      } catch {}
    }
    return
  }
  const by = senderKey(from)
  switch (data.t) {
    case 'b': {
      if (!by || typeof data.id !== 'string') return
      const line = { id: data.id, by, name: String(data.name || ''), c: String(data.c || ''), map: String(data.map || ''), floor: String(data.floor || ''), w: Number(data.w) || 5, p: [], gen: 0 }
      if (Number.isFinite(data.z)) line.z = data.z
      sq.live.set(data.id, { line, at: Date.now() })
      return notify(true)
    }
    case 'p': {
      const live = sq.live.get(data.id)
      if (!live || live.line.by !== by || !Array.isArray(data.p)) return
      for (const q of data.p) if (Array.isArray(q) && Number.isFinite(q[0]) && Number.isFinite(q[1])) live.line.p.push([q[0], q[1]])
      live.at = Date.now()
      const last = live.line.p[live.line.p.length - 1]
      if (last) sq.cursors.set(by, { name: live.line.name, c: live.line.c, map: live.line.map, floor: live.line.floor, x: last[0], z: last[1], at: Date.now() })
      return notify(true)
    }
    case 'c': {
      if (!by) return
      if (data.off) sq.cursors.delete(by)
      else if (Number.isFinite(data.x) && Number.isFinite(data.z))
        sq.cursors.set(by, { name: String(data.name || ''), c: String(data.c || ''), map: String(data.map || ''), floor: String(data.floor || ''), x: data.x, z: data.z, at: Date.now() })
      return notify(true)
    }
    case 'l': {
      mergeGens(data.gens)
      let changed = false
      // A line just drawn tells what someone is drawing (squad-share.js).
      const first = Array.isArray(data.lines) ? data.lines[0] : null
      if (data.new && first && by) handlers.drew?.(by, { name: first.name, c: first.c, map: first.map })
      for (const l of Array.isArray(data.lines) ? data.lines : []) {
        sq.live.delete(l?.id)
        // This member's own lines are only its own to give.
        if (!valid(l) || l.by === myKey || !shown(l)) continue
        if (!sq.lines.has(l.id) && sq.lines.size >= MAX_LINES) continue
        sq.lines.set(l.id, l)
        changed = true
      }
      // Own lines of a map cleared meanwhile go (not while the clear can be undone).
      if (!clearedNow()) for (const l of own()) if (!shown(l)) sq.lines.delete(l.id)
      if (changed) notify()
      notify(true)
      return
    }
    case 'e': {
      let changed = false
      for (const id of Array.isArray(data.ids) ? data.ids : [])
        if (sq.lines.get(id)?.by === by && by) changed = sq.lines.delete(id) || changed
      if (changed) notify()
      return
    }
    case 'x': {
      if (typeof data.map !== 'string' || !Number.isInteger(data.gen) || data.gen <= genOf(data.map)) return
      sq.cleared = { map: data.map, gen: data.gen, prev: genOf(data.map), name: String(data.name || ''), at: Date.now(), mine: false }
      sq.gens[data.map] = data.gen
      saveOwn()
      notify()
      render()
      return
    }
    case 'u': {
      if (typeof data.map !== 'string' || !Number.isInteger(data.gen) || genOf(data.map) !== data.gen) return
      sq.gens[data.map] = Number(data.prev) || 0
      if (sq.cleared?.map === data.map) sq.cleared = null
      saveOwn()
      notify()
      render()
      return
    }
    default:
      if (by && typeof data.t === 'string') handlers[data.t]?.(by, data)
  }
}

// What this member does with the pen.
export function begin(line) {
  send({ t: 'b', id: line.id, map: line.map, floor: line.floor, w: line.w, z: line.z, c: line.c, name: line.name })
}
export function points(id, list) {
  if (list.length) send({ t: 'p', id, p: list }, true)
}
export function add(lines) {
  for (const l of lines) sq.lines.set(l.id, l)
  sendLines(lines, true)
  saveOwn()
  notify()
}
export function erase(lines) {
  const ids = lines.map((l) => l.id)
  for (const id of ids) sq.lines.delete(id)
  send({ t: 'e', ids })
  saveOwn()
  notify()
}
export function clearAll(map) {
  const prev = genOf(map)
  const gen = prev + 1
  sq.gens[map] = gen
  sq.cleared = { map, gen, prev, name: myStyle().name, at: Date.now(), mine: true }
  send({ t: 'x', map, gen, name: myStyle().name })
  saveOwn()
  notify()
}
export function undoClear() {
  const c = sq.cleared
  if (!c || Date.now() - c.at > UNDO_MS || genOf(c.map) !== c.gen) return
  sq.gens[c.map] = c.prev
  sq.cleared = null
  send({ t: 'u', map: c.map, gen: c.gen, prev: c.prev })
  saveOwn()
  notify()
}
let cursorAt = 0
export function cursor(map, floor, x, z) {
  const now = Date.now()
  if (now - cursorAt < 100) return
  cursorAt = now
  const { c, name } = myStyle()
  send({ t: 'c', map, floor, x: Math.round(x * 100) / 100, z: Math.round(z * 100) / 100, c, name }, true)
}
export function cursorOff() {
  cursorAt = 0
  if (canDraw()) send({ t: 'c', off: 1 }, true)
}

// shownLines is the lines of a map that show (not cleared); liveLines those
// being drawn there.
export const shownLines = (map) => [...sq.lines.values()].filter((l) => l.map === map && shown(l))
export const liveLines = (map) => [...sq.live.values()].map((v) => v.line).filter((l) => l.map === map && l.p.length)
export const cursorsOn = (map) => [...sq.cursors.values()].filter((c) => c.map === map)
export const clearedNow = () => (sq.cleared && Date.now() - sq.cleared.at <= UNDO_MS ? sq.cleared : null)

// Lines being drawn by someone gone quiet, positions unmoved, pieces never
// completed and a clear past undoing go.
setInterval(() => {
  const now = Date.now()
  let live = false
  for (const [id, v] of sq.live) if (now - v.at > LIVE_MS) live = sq.live.delete(id) || live
  for (const [key, c] of sq.cursors) if (now - c.at > CURSOR_MS) live = sq.cursors.delete(key) || live
  for (const [key, p] of parts) if (now - p.at > PART_MS) parts.delete(key)
  if (sq.cleared && now - sq.cleared.at > UNDO_MS) {
    const c = sq.cleared
    sq.cleared = null
    // Past undoing, the cleared lines are dropped.
    for (const [id, l] of sq.lines) if (l.map === c.map && !shown(l)) sq.lines.delete(id)
    saveOwn()
    notify()
    render()
  }
  if (live) notify(true)
}, 1000)
