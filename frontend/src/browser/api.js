import * as AppService from '../../bindings/github.com/local/mayak/internal/app/app'
import { Events, Clipboard, Browser } from '@wailsio/runtime'
import { itemInfo, clampItemPanel, clampItemPanelHeight, historyPoints, names } from './item.js'
import {
  browserSections,
  hostSections,
  randomUUID,
  bookmarkGroup,
  clampSidebar,
  rememberFavicon,
  hostname,
  defaults,
  restore,
  webURL,
  pageURL,
  receiveTask,
  receiveMap,
  receivePosition,
  rememberSquad,
  receiveOf,
  linkKind,
  translatedURL,
  originalURL,
  isTranslated,
  moveTab,
  togglePin,
  pinBookmark,
  bookmarkTab,
  goHome,
  openLocal,
  tabAt,
  cycleTab,
} from './state.js'
import { encode, decode, MAX_AGE, PAIR_RELAY } from './peer-code.js'
import { hiddenOf, mapSettingsOf, squadColors } from './map-geo.js'
import { t } from './words.js'
import './transport.js'

// onMenu gets the choice of a menu opened in the menu window ("" for none).
let onMenu = /** @type {(choice:string)=>void} */ (() => {})
let state,
  go,
  platform,
  returnTo = '',
  host = null,
  hostQuestSite = 'tarkov-dev',
  updateChannel = 'stable',
  popup = null,
  item = null,
  restoredItem = null,
  itemOpen = false,
  itemSearch = { query: '', results: [] },
  searchSeq = 0,
  itemBusy = false,
  itemHistory = null,
  notify = /** @type {(state:any)=>void} */ (() => {}),
  onKey = /** @type {(key:any)=>void} */ (() => {}),
  section = 'appearance',
  error = '',
  peerState = { phase: 'idle' }
// Tabs closed in this session, newest last, for Ctrl+Shift+T (not saved).
const closedTabs = []
let queue = Promise.resolve(),
  nativeQueue = Promise.resolve(),
  expiry
// While the shell shows an overlay (the tutorial), the native page views stay
// hidden whatever else asks to show them; closing it shows the active tab again.
let overlay = false
// The updater's state (model.UpdateStatus) drives the update bar: a status
// strip along the bottom of the whole window while a newer version is found,
// downloading or ready. The page views are native windows above the shell,
// so the bar takes its own row (bounds() lifts the pages by it) rather than
// floating over them, where it would be covered.
let updateStatus = null,
  updateDismissed = ''
const updateBarHeight = 32
function updateBarVisible() {
  const u = updateStatus
  return !!(u && ['available', 'downloading', 'ready'].includes(u.state) && u.latest && u.latest !== updateDismissed)
}
const views = new Map()
// Tabs whose page is loading, by view ID (not saved).
const loadingViews = new Set()
// The game's screenshots (on the Host): newest first, their thumbnails and
// the full image being viewed, as data URLs from the Go side.
const shots = { list: [], thumbs: {}, full: {}, viewing: '' }
// Snap notes (internal/snapnote, app_snapnote.go): the list (latest changed
// first), their thumbnails by ID, the note open for drawing (its image as a
// data URL and its strokes), the list's filter and a capture under way.
const snaps = { list: [], thumbs: {}, open: null, filter: 'all', busy: false, fonts: null }
const snapThumbKeys = {}
const snapsAvailable = () => platform === 'windows'
const thumbQueue = []
let thumbLoading = false
// Bosses of each map and the Goons reports (json.tarkov.dev, through the Go
// side), asked again every few minutes: unchanged data costs a small request.
let bosses = null,
  bossesLoading = false,
  bossesAgain = false
// The Goons report being written: what it can be about (the raid and the
// accounts seen in the logs), and how sending it went.
let goonReport = null
const bossesEvery = 5 * 60 * 1000
// Site icons as data URLs from the Host's icon cache, by icon URL: shown at
// once and offline, and replaced when a page reports a changed icon.
const faviconData = {}
function loadFavicon(url, refresh = false) {
  if (!webURL(url)) return
  go.BrowserFavicon(url, refresh)
    .then((data) => {
      if (typeof data === 'string' && data.startsWith('data:image/') && faviconData[url] !== data) {
        faviconData[url] = data
        update()
      }
    })
    .catch(() => {})
}

// The map view and its squad (app_squad.go, view-map.js): whether this build
// offers it, the squad as the Go side last reported it, the maps' geometry,
// the map pictures loaded (by "map|floor") and the markers (by
// "map|language").
const squad = { available: false, state: null, maps: null, mapsError: false, images: {}, markers: {} }
async function loadSquadMaps() {
  if (!squad.available || squad.maps) return
  try {
    squad.maps = (await go.BrowserSquadMaps()) || []
    squad.mapsError = false
  } catch {
    squad.mapsError = true
  }
  update()
}
// squadJoin joins a squad as name; here (not for the other PC's word) it
// also tells the other PC. The squad's page stays, with the code to share.
async function squadJoin(code, name, here = true) {
  name = String(name || '')
    .trim()
    .slice(0, 24)
  if (!name && here) throw new Error(t(state.language, 'squadNeedName'))
  if (name) state.squadName = name
  await go.SquadSetColor(state.squadColor)
  state.squadCode = await go.SquadJoin(String(code || ''), name || host?.player || 'Player')
  state.squadRecent = rememberSquad(state.squadRecent, state.squadCode)
  squad.state = await go.SquadState()
  if (here) shareSquad()
  void loadSquadMaps()
}

const snapshot = () => ({
  ...state,
  squad: squad.available
    ? {
        state: squad.state,
        maps: squad.maps,
        mapsError: squad.mapsError,
        images: squad.images,
        markers: squad.markers,
      }
    : null,
  snapNotes: snapsAvailable()
    ? {
        list: snaps.list,
        thumbs: snaps.thumbs,
        open: snaps.open,
        filter: snaps.filter,
        busy: snaps.busy,
        fonts: snaps.fonts,
      }
    : null,
  update: updateStatus,
  updateChannel,
  updateBar: updateBarVisible(),
  statusRows: statusRows(),
  goonReport,
  bosses: bosses && { ...bosses, current: host?.map || '' },
  screenshots: shotsAvailable()
    ? { list: shots.list, thumbs: shots.thumbs, viewing: shots.viewing, full: shots.full[shots.viewing] || '' }
    : null,
  loadingTabs: [...loadingViews],
  popup: popup && { key: popup.key },
  faviconData,
  host: state.connection.mode === 'local' ? host : null,
  hostQuestSite,
  item,
  itemOpen,
  itemSearch,
  itemBusy,
  itemHistory,
  settingsSection: section,
  localHost: platform === 'windows',
  platform,
  connectionStatus:
    state.connection.mode === 'local'
      ? 'connected'
      : state.connection.mode === 'off'
        ? 'off'
        : peerState.phase === 'connected'
          ? 'connected'
          : 'disconnected',
  // paired: a pairing is kept for this mode.
  peer: { ...peerState, paired: !!state.connection.link && state.connection.link.role === linkRole() },
  error,
})
const update = () => notify(snapshot())
// An error shows as a strip along the bottom, in a row of its own like the
// update bar (see there): a toast over the page would be under it. The row
// appears when the first error arrives; a failure to show the pages after
// that is not retried, or it would loop.
const messageError = (e) => {
  const shown = !!error
  error = t(state.language, 'actionFailed') + String(e?.message || e)
  update()
  if (!shown) void show().catch(() => {})
}
const native = (command, o) => {
  const result = nativeQueue.then(() => go.BrowserView(command, o))
  nativeQueue = result.catch(() => {})
  return result
}
// Every page starts below the toolbar row; fixed views show it without an
// address bar (reload and "open in browser" only).
// The item sidebar keeps its width free on the right while it is open.
// The page area: the window less the tab sidebar (left, right, or the top
// strip) and the item panel (left, right or bottom); both may share a side.
function bounds() {
  const nav = state.layout === 'horizontal' ? 0 : state.sidebarCollapsed ? 52 : state.sidebarWidth
  const dock = itemOpen ? state.itemDock : ''
  const item = dock === 'bottom' ? 0 : state.itemPanelWidth
  return {
    left: (state.sidebarSide === 'left' ? nav : 0) + (dock === 'left' ? item : 0),
    top: state.layout === 'horizontal' ? 96 : 48,
    right: (state.sidebarSide === 'right' ? nav : 0) + (dock === 'right' ? item : 0),
    bottom: (dock === 'bottom' ? state.itemPanelHeight : 0) + statusRows() * updateBarHeight,
  }
}
// The rows along the bottom that the pages make room for: the update bar and
// the error strip. The shell lays them out from the same count.
const statusRows = () => (updateBarVisible() ? 1 : 0) + (error ? 1 : 0)
// A page view opens at its tab's address. (The fixed tarkov.dev map view,
// which added the Host's Remote Control ID, is gone: map detections show on
// the map view, view-map.js.)
const viewURL = (tab) => tab.url
// The theme background fills a tab until its page paints, instead of white.
const pageBackground = () => getComputedStyle(document.documentElement).getPropertyValue('--bg').trim()
// Pages opened from the item sidebar show in a popup window of their own
// (see app_popup.go), which opens beside the sidebar, level with the row
// clicked (anchor, a y in this window).
const popupID = 'popup',
  popupGap = 8,
  popupMargin = 12
function popupPlace() {
  const page = bounds(),
    width = innerWidth,
    height = innerHeight
  const w = Math.round(Math.max(320, Math.min(960, width - page.left - page.right - popupGap - popupMargin)))
  const h = Math.round(Math.max(240, Math.min(800, height - page.top - page.bottom - popupMargin * 2)))
  const top = Math.round(
    Math.min(Math.max(page.top + popupMargin, (popup?.anchor || 0) - 160), height - page.bottom - popupMargin - h),
  )
  return {
    x: state.itemDock === 'left' ? page.left + popupGap : width - page.right - popupGap - w,
    y: top,
    width: w,
    height: h,
  }
}
// popupClosed is the popup the window closed by itself (it lost the focus),
// so the click that took the focus does not open it again straight away.
let popupClosed = { key: '', at: 0 }
async function closePopup() {
  popup = null
  update()
  await go.BrowserPopupClose().catch(() => {})
}
// openPopup shows a page in the popup; the same page again closes it.
async function openPopup(next) {
  if (popup && popup.key === next.key) {
    await closePopup()
    return
  }
  if (!popup && popupClosed.key === next.key && Date.now() - popupClosed.at < 500) return
  popup = { ...next, openedAt: Date.now() }
  update()
  await go.BrowserPopupShow(
    {
      url: next.url,
      title: next.title,
      task: !!next.task,
      background: pageBackground(),
      theme: document.documentElement.dataset.theme || '',
      language: state.language,
    },
    popupPlace(),
  )
}
async function show() {
  const tab = state.tabs.find((t) => t.id === state.active)
  if (overlay || tab?.kind !== 'web') {
    await native('hideAll', { id: 'shell' })
    return
  }
  const previous = views.get(tab.id)
  await native('show', { id: tab.id, url: viewURL(tab), ...bounds(), background: pageBackground() })
  if (previous && previous !== tab.url) await native('navigate', { id: tab.id, url: viewURL(tab) })
  views.set(tab.id, tab.url)
}
async function persist() {
  // Construct the persisted representation explicitly. RTC codes and UI errors
  // never enter Go storage. Existing Host credentials stay in their own store.
  // Until a restored item has loaded, the saved one is kept.
  const shown = item || restoredItem
  const itemPanel = { open: itemOpen, id: shown?.id || '', mode: shown?.mode || '' }
  const {
    version,
    bookmarkRevision,
    language,
    tutorialDone,
    clock,
    layout,
    sidebarSide,
    sidebarCollapsed,
    bookmarksCollapsed,
    screenshotsCollapsed,
    snapNotesCollapsed,
    squadCollapsed,
    toolOrder,
    bossesView,
    bossMap,
    bossMode,
    sidebarWidth,
    itemPanelWidth,
    itemPanelHeight,
    itemDock,
    bookmarkView,
    favicons,
    theme,
    adblock,
    taskMode,
    questSite,
    translateWiki,
    mapHidden,
    mapSettings,
    mapCollapsed,
    squadName,
    squadColor,
    squadCode,
    squadRecent,
    bookmarks,
    tabs,
    active,
  } = state
  await go.BrowserSave(
    JSON.stringify({
      version,
      bookmarkRevision,
      language,
      tutorialDone,
      clock,
      layout,
      sidebarSide,
      sidebarCollapsed,
      bookmarksCollapsed,
      screenshotsCollapsed,
      snapNotesCollapsed,
      squadCollapsed,
      toolOrder,
      bossesView,
      bossMap,
      bossMode,
      sidebarWidth,
      itemPanelWidth,
      itemPanelHeight,
      itemDock,
      itemPanel,
      bookmarkView,
      favicons,
      theme,
      adblock,
      taskMode,
      questSite,
      translateWiki,
      mapHidden,
      mapSettings,
      mapCollapsed,
      squadName,
      squadColor,
      squadCode,
      squadRecent,
      bookmarks,
      tabs,
      active,
      connection: { mode: state.connection.mode, link: state.connection.link, receive: state.connection.receive },
    }),
  )
}
async function changed() {
  update()
  await persist()
  void show().catch(messageError)
}
function enqueue(fn) {
  const result = queue.then(fn)
  queue = result.catch(messageError)
  return result.catch(() => snapshot())
}
function display(message, remote = false) {
  return enqueue(async () => {
    if (remote ? state.connection.mode !== 'client' : state.connection.mode !== 'local') return
    // A Client shows only what it takes from its Host.
    if (remote && state.connection.receive?.[linkKind(message.event)] === false) return
    if (message.event === 'browser:item') {
      const next = itemInfo(message.args[0])
      if (!next) return
      item = next
      itemOpen = true
      loadHistory()
      void persist().catch(() => {})
      if (!remote) peer.send(message)
      update()
      await show()
      return
    }
    // The last detection decides the tab shown: a task its page, a position
    // (after a task, say) the map view again.
    const tab =
      message.event === 'browser:task'
        ? receiveTask(state, message.args[0])
        : message.event === 'browser:map'
          ? receiveMap(state, message.args[0])
          : message.event === 'browser:position'
            ? receivePosition(state, message.args[0])
            : null
    if (tab) {
      if (!remote) peer.send(message)
      // The map view follows the map played again (view-map.js).
      if (tab.kind === 'livemap') window.dispatchEvent(new Event('mayak:map-follow'))
      await changed()
    }
  })
}
// hostStatus keeps what the sidebar shows of a (possibly partial) Host status.
function hostStatus(s) {
  const out = {}
  if (!s || typeof s !== 'object') return out
  if ('monitoring' in s) out.monitoring = !!s.monitoring
  if ('currentMap' in s) out.map = String(s.currentMap || '')
  if ('raidActive' in s) out.raid = !!s.raidActive
  // The last position screenshot, for the map view (in game coordinates).
  if ('position' in s) {
    const p = s.position
    out.position =
      p && [p.x, p.y, p.z].every(Number.isFinite)
        ? { x: p.x, y: p.y, z: p.z, rot: Number(p.rotation) || 0, at: String(p.detectedAt || '') }
        : null
  }
  if (s.tracker && typeof s.tracker === 'object') {
    out.tracker = String(s.tracker.connection || '')
    // Keys registered (none hides the TarkovTracker indicator) and the last
    // error, for the indicator's tooltip.
    out.trackerKeys = Array.isArray(s.tracker.keys) ? s.tracker.keys.length : 0
    out.trackerError = String(s.tracker.lastError || '')
    out.mode = String(s.tracker.mode || '')
    // The player's name, the map view's default name for a squad.
    out.player = String(s.tracker.displayName || '').trim().slice(0, 24)
    out.identity = [s.tracker.accountId, s.tracker.profileId, s.tracker.mode].map((v) => String(v || '')).join('|')
  }
  return out
}
// The task site for pages opened from the item sidebar: the Host's current
// setting on the Host, else this browser's choice or the one the item came with.
function itemSite() {
  if (state.connection.mode === 'local') return hostQuestSite
  return state.questSite === 'host' ? item?.questSite || 'tarkov-dev' : state.questSite
}
// Views are placed in whole pixels.
const anchorY = (data) => Math.round(Number.isFinite(data?.anchor) ? data.anchor : innerHeight / 2)
// loadHistory fetches the price history of the current item for its chart,
// once per item unless again is set (the refresh button).
// Screenshots are the Host's: only there are they listed.
const shotsAvailable = () => platform === 'windows' && state?.connection.mode === 'local'
// What the Host recognized a screenshot as (see screenshotRecord in Go).
function shotMeta(m) {
  if (!m || typeof m !== 'object') return null
  const text = (v, max = 200) => (typeof v === 'string' ? v.slice(0, max) : '')
  const num = (v) => (Number.isFinite(v) ? v : 0)
  return {
    type: ['tasks', 'item', 'position'].includes(m.type) ? m.type : 'unknown',
    layout: text(m.layout, 60),
    score: num(m.score),
    map: text(m.map, 60),
    raid: m.raid === true,
    stage: text(m.stage),
    match: text(m.match),
    detail: text(m.detail, 80),
    confidence: num(m.confidence),
    candidates: Array.isArray(m.candidates) ? m.candidates.slice(0, 3).map((c) => text(c)) : [],
    ocr: text(m.ocr, 500),
    position: text(m.position, 80),
    error: text(m.error, 300),
  }
}
async function loadSnaps() {
  if (!snapsAvailable()) return
  try {
    const list = await go.SnapNoteList()
    snaps.list = Array.isArray(list) ? list.filter((n) => n && typeof n.id === 'string') : []
  } catch {
    return
  }
  update()
  for (const note of snaps.list.slice(0, 200)) void loadSnapThumb(note)
}
// A thumbnail is loaded again only after its note changed.
async function loadSnapThumb(note) {
  const key = note.updatedAt || ''
  if (snapThumbKeys[note.id] === key) return
  snapThumbKeys[note.id] = key
  try {
    const data = await go.SnapNoteThumb(note.id)
    if (typeof data === 'string' && data.startsWith('data:image/jpeg;base64,')) snaps.thumbs[note.id] = data
    else delete snaps.thumbs[note.id]
    update()
  } catch {}
}
async function openSnap(id) {
  const data = await go.SnapNoteOpen(id)
  if (!data?.note || typeof data.image !== 'string') throw new Error('snap note')
  snaps.open = { note: { ...data.note, strokes: undefined }, image: data.image, strokes: data.note.strokes ?? [] }
}
async function loadShots() {
  if (!shotsAvailable()) return
  try {
    const list = await go.BrowserScreenshots(300)
    shots.list = (Array.isArray(list) ? list : [])
      .filter((s) => s && typeof s.name === 'string')
      .map((s) => ({ name: s.name, time: String(s.time || ''), meta: shotMeta(s.meta) }))
  } catch {
    return
  }
  if (shots.list[0]) queueThumb(shots.list[0].name, true)
  // The screenshot page (also when restored open) shows them all.
  if (state.tabs.find((t) => t.id === state.active)?.kind === 'screenshots')
    for (const s of shots.list.slice(0, 120)) queueThumb(s.name)
  update()
}
// Thumbnails load one at a time, the sidebar's latest first.
function queueThumb(name, first = false) {
  if (shots.thumbs[name] || thumbQueue.includes(name)) return
  if (first) thumbQueue.unshift(name)
  else thumbQueue.push(name)
  void nextThumb()
}
async function nextThumb() {
  if (thumbLoading || !thumbQueue.length) return
  thumbLoading = true
  const name = thumbQueue.shift()
  try {
    const data = await go.BrowserScreenshotImage(name, true)
    if (typeof data === 'string' && data.startsWith('data:image/jpeg;base64,')) shots.thumbs[name] = data
    update()
  } catch {}
  thumbLoading = false
  void nextThumb()
}
// The full image of the one viewed, and of its neighbours for paging.
async function viewShot(name) {
  shots.viewing = name
  update()
  if (!name) return
  const index = shots.list.findIndex((s) => s.name === name)
  const wanted = [name, shots.list[index + 1]?.name, shots.list[index - 1]?.name].filter(Boolean)
  for (const key of Object.keys(shots.full)) if (!wanted.includes(key)) delete shots.full[key]
  for (const want of wanted) {
    if (shots.full[want]) continue
    try {
      const data = await go.BrowserScreenshotImage(want, false)
      if (typeof data === 'string' && data.startsWith('data:image/jpeg;base64,')) shots.full[want] = data
    } catch {}
    if (want === name) update()
  }
}
// The action queue is serial: everything a click asks for waits behind what
// is already in it. Fetches from the network never go into it, then, or a
// slow or failing connection (a 45 s timeout per item lookup) would hold
// every click for minutes: the sidebar looks dead while the pages, in their
// own processes, still respond. These run on their own and update the item
// when they return, if it is still the one shown.
async function refreshItem(auto) {
  if (!item || itemBusy) return
  itemBusy = true
  update()
  // The spinner stays long enough to be seen: a cached answer comes back at once.
  const started = Date.now(),
    id = item.id
  try {
    const next = itemInfo(await go.BrowserItemInfo(item.mode, id))
    if (next && next.id === item?.id) item = next
  } catch (e) {
    if (!auto) messageError(e)
  } finally {
    const wait = 600 - (Date.now() - started)
    if (wait > 0) await new Promise((resolve) => setTimeout(resolve, wait))
    itemBusy = false
  }
  void loadHistory(true)
  update()
}
async function reloadItem() {
  if (!item) return
  try {
    const next = itemInfo(await go.BrowserItemInfo('', item.id))
    if (next && next.id === item.id) {
      item = next
      void loadHistory()
      void persist().catch(() => {})
      update()
    }
  } catch {}
}
// A reload asked for while one runs follows it: the game mode may have become
// known meanwhile, and the running one fetched the other mode's reports.
async function loadBosses() {
  if (bossesLoading) {
    bossesAgain = true
    return
  }
  bossesLoading = true
  try {
    const info = await go.BrowserBosses(state.bossMode, state.language)
    if (info && Array.isArray(info.maps))
      bosses = {
        mode: String(info.mode || ''),
        fetched: String(info.fetched || ''),
        maps: info.maps,
        goons: Array.isArray(info.goons) ? info.goons : [],
      }
  } catch {}
  bossesLoading = false
  update()
  if (bossesAgain) {
    bossesAgain = false
    void loadBosses()
  }
}
async function loadHistory(again = false) {
  const { id, mode } = item || {}
  if (!id) return
  if (!again && itemHistory?.id === id && itemHistory.mode === mode) return
  if (!itemHistory || itemHistory.id !== id || itemHistory.mode !== mode)
    itemHistory = { id, mode, points: [], loading: true }
  update()
  let next
  try {
    next = { id, mode, points: historyPoints(await go.BrowserItemHistory(mode, id)) }
  } catch {
    next = { id, mode, points: itemHistory?.points || [], failed: true }
  }
  if (item?.id === id && item.mode === mode) {
    itemHistory = next
    update()
  }
}
// Pairing: the Host makes a key for the link (transport.js) and parks the
// invitation under an 8-digit code on the pairing relay (the long code can be
// copied by hand as well); the other PC fetches it by that code and joins the
// link. Once the two have met, both keep the key (connection.link), meet again
// at every start and the code is dropped. The relay is optional: a relay
// error leaves the long code.
async function relay(method, path, body) {
  const res = await fetch(PAIR_RELAY + path, {
    method,
    headers: body ? { 'content-type': 'application/json' } : undefined,
    body: body ? JSON.stringify(body) : undefined,
    cache: 'no-store',
  })
  if (res.status === 204) return null
  if (!res.ok) throw new Error(res.status === 404 ? 'pair-code-not-found' : 'relay-unavailable')
  return res.json()
}
function forgetPairCode() {
  const code = peerState.pairCode
  if (code) void relay('DELETE', '/' + code).catch(() => {})
}
// The pairing being made: kept once the other PC is met.
let pending = null
// linkRole is the side this PC takes in the mode chosen.
const linkRole = () => (state.connection.mode === 'local' ? 'host' : state.connection.mode === 'client' ? 'client' : '')
const peer = new /** @type {any} */ (globalThis).MayakLink({
  onState: (next) => {
    peerState = { ...peerState, ...next, busy: false }
    if (next.joined) {
      // A PC met for the first time by the code: the code has done its work
      // (a Client coming back leaves the Host's code for the next one).
      if (next.newcomer || pending) {
        forgetPairCode()
        peerState.code = ''
        peerState.pairCode = ''
        clearTimeout(expiry)
      }
      if (pending) {
        state.connection.link = pending
        pending = null
        void persist().catch(() => {})
      }
      // The Host says which squad it is in; a client in one the Host is
      // not answers with its own (receiveSquad).
      if (state.connection.mode === 'local' && squad.available)
        peer.sendSquad({ code: state.squadCode, name: state.squadName, color: state.squadColor, initial: true })
    }
    update()
  },
  onMessage: (message) => void display(message, true),
  onSquad: (s) => void enqueue(() => receiveSquad(s)),
  // The Host ended the pairing: this Client forgets it too.
  onUnpair: () => void enqueue(() => unpair(false)),
})
// startLink joins the link of the pairing kept (or being made) when this
// PC's mode is its side, and leaves it otherwise.
function startLink() {
  const link = pending || state.connection.link
  if (!link || link.role !== linkRole()) {
    peer.stop()
    peerState = { phase: 'idle' }
    return
  }
  peerState = { ...peerState, phase: 'connecting', role: link.role, reason: '' }
  void peer.start(link.key, link.role, link === pending && link.role === 'client').catch(messageError)
}
// receiveSquad follows the squad the other PC joined or left, without
// telling it back. On connecting, a client already in a squad keeps it and
// brings the Host in instead when the Host is in none.
async function receiveSquad({ code, name, initial, color }) {
  if (!squad.available) return
  // The squad colour follows the other PC's too (the Host reports it).
  const recolor = color !== undefined && color !== state.squadColor && (color === '' || squadColors.includes(color))
  if (recolor) {
    state.squadColor = color
    await go.SquadSetColor(color)
  }
  if (!code) {
    if (initial && state.squadCode) {
      peer.sendSquad({ code: state.squadCode, name: state.squadName, color: state.squadColor })
      return
    }
    if (!state.squadCode) return
    await go.SquadLeave()
    squad.state = null
    state.squadCode = ''
  } else if (code !== state.squadCode) {
    await squadJoin(code, state.squadName || name, false)
  } else if (!recolor) return
  // The Host passes a Client's word on to its other Clients.
  if (state.connection.mode === 'local') shareSquad()
  await changed()
}
// shareSquad tells the other PC of a squad joined or left here.
const shareSquad = () => peer.sendSquad({ code: state.squadCode, name: state.squadName, color: state.squadColor })
// unpair ends the pairing (or the one being made), telling the other PC
// unless it told this one.
async function unpair(tell = true) {
  forgetPairCode()
  clearTimeout(expiry)
  pending = null
  if (tell) await peer.unpair()
  else peer.stop()
  state.connection.link = null
  peerState = { phase: 'idle' }
  await persist()
  update()
}
// expireIn drops a code not used within its ten minutes, and a pairing not
// made by then.
function expireIn(ms) {
  clearTimeout(expiry)
  expiry = setTimeout(() => void enqueue(() => cancelCode(true)), ms)
}
// cancelCode drops the code handed out (and the pairing being made with it).
function cancelCode(expired = false) {
  clearTimeout(expiry)
  forgetPairCode()
  peerState = { ...peerState, code: '', pairCode: '', relayError: '' }
  if (pending) {
    pending = null
    startLink()
    if (peerState.phase === 'idle' && expired) peerState = { phase: 'failed', reason: 'invite-expired' }
  }
  update()
}
// join takes an invitation (a long code) on the Client.
function join(code, pairCode = '') {
  if (state.connection.mode !== 'client') throw new Error('receiver-required')
  const offer = decode(code)
  pending = { key: offer.key, role: 'client' }
  peerState = { role: 'client', phase: 'connecting', pairCode }
  startLink()
  expireIn(Math.max(0, MAX_AGE - (Date.now() - offer.createdAt)))
}
async function pairing(type, data) {
  if (peerState.busy) return snapshot()
  if (type === 'peerClose') {
    await unpair()
    return snapshot()
  }
  if (type === 'peerCancel') {
    cancelCode()
    return snapshot()
  }
  if (type === 'peerCopy') {
    await Clipboard.SetText(peerState.code || '')
    return snapshot()
  }
  if (type === 'peerCopyPair') {
    await Clipboard.SetText(peerState.pairCode || '')
    return snapshot()
  }
  peerState = { ...peerState, busy: true, code: '', pairCode: '', relayError: '', reason: '' }
  update()
  try {
    if (type === 'peerInvite') {
      if (platform !== 'windows' || state.connection.mode !== 'local') throw new Error('host-required')
      // Another Client joins the pairing kept, with its key.
      const kept = state.connection.link?.role === 'host' ? state.connection.link.key : ''
      const invite = {
        version: 2,
        type: 'link',
        id: randomUUID(),
        createdAt: Date.now(),
        key: kept || /** @type {any} */ (globalThis).newLinkKey(),
      }
      const code = encode(invite)
      if (!kept) {
        pending = { key: invite.key, role: 'host' }
        startLink()
      }
      peerState = { ...peerState, role: 'host', code, busy: false }
      expireIn(MAX_AGE)
      try {
        const posted = await relay('POST', '', { invite: code })
        if (peerState.code === code) peerState.pairCode = posted.code
        else void relay('DELETE', '/' + posted.code).catch(() => {})
      } catch (e) {
        peerState.relayError = String(e?.message || e)
      }
    } else if (type === 'peerJoin') {
      if (state.connection.mode !== 'client') throw new Error('receiver-required')
      const digits = String(data || '').replace(/\D/g, '')
      if (digits.length !== 8) throw new Error('pair-code-invalid')
      const fetched = await relay('GET', '/' + digits)
      join(fetched?.invite, digits)
    } else if (type === 'peerAccept') {
      join(String(data || ''))
    }
  } catch (e) {
    pending = null
    clearTimeout(expiry)
    startLink()
    peerState = { ...peerState, phase: 'failed', busy: false }
    messageError(e)
  }
  peerState.busy = false
  update()
  return snapshot()
}
const ready = (async () => {
  go = AppService
  platform = await go.BrowserPlatform()
  let migrated = false
  try {
    const saved = JSON.parse(await go.BrowserLoad())
    state = restore(saved)
    migrated = saved.bookmarkRevision !== state.bookmarkRevision
  } catch (e) {
    state = defaults()
    error = String(e)
  }
  if (migrated)
    try {
      await persist()
    } catch (e) {
      error = String(e)
    }
  // The item sidebar comes back as it was, with current prices for its item.
  itemOpen = state.itemPanel.open
  if (state.itemPanel.id) {
    restoredItem = state.itemPanel
    go.BrowserItemInfo(state.itemPanel.mode, state.itemPanel.id)
      .then((raw) => {
        const next = itemInfo(raw)
        if (next && !item) {
          item = next
          void loadHistory()
          update()
        }
      })
      .catch(() => {})
  }
  if (!['local', 'client', 'off'].includes(state.connection.mode)) state.connection.mode = 'off'
  if (platform !== 'windows' && state.connection.mode === 'local') state.connection.mode = 'client'
  await go.BrowserSetMode(state.connection.mode)
  // The pairing kept meets its other PC again (transport.js).
  peer.relay = (await go.BrowserLinkRelay()) || peer.relay
  startLink()
  try {
    await go.BrowserSetAdblock(state.adblock)
  } catch (e) {
    error = String(e)
  }
  // The trusted settings document shares only the trusted parent's runtime.
  // External websites are native views and never receive this object.
  window.mayakDesktop = {
    backend: AppService,
    on: (name, callback) => Events.On(name, (event) => callback(event.data)),
    openURL: (url) => void window.mayak.action('open', url),
  }

  window.mayakDesktop.on('browser:menu', (event) => {
    if (event.action === 'settings') void window.mayak.action('settings')
    else if (event.action === 'bookmark') {
      const b = state.bookmarks.find((b) => b.id === event.id)
      if (b) void window.mayak.action('open', b.url)
    }
  })
  window.mayakDesktop.on('browser:task', (task) => void display({ event: 'browser:task', args: [task] }))
  window.mayakDesktop.on('browser:map', (map) => void display({ event: 'browser:map', args: [map] }))
  window.mayakDesktop.on('browser:position', (map) => void display({ event: 'browser:position', args: [map] }))
  // A shortcut pressed inside a page view (Ctrl+T and the like) is handled by
  // the shell, like one pressed in the shell itself.
  window.mayakDesktop.on('browser:key', (key) => {
    if (key && typeof key.key === 'string')
      onKey({ key: key.key, ctrl: !!key.ctrl, shift: !!key.shift, alt: !!key.alt })
  })
  // The update channel is this PC's, whichever mode it runs in.
  try {
    updateChannel = (await go.GetSettings()).updateChannel === 'nightly' ? 'nightly' : 'stable'
  } catch {}
  // The Host's monitoring state for the sidebar's monitoring button.
  if (platform === 'windows') {
    try {
      host = hostStatus(await go.GetStatus())
      const s = await go.GetSettings()
      hostQuestSite = s.questSite || 'tarkov-dev'
    } catch {}
    // On the Host the task site is one setting, the Host's; a site chosen in
    // the browser before becomes that setting once.
    if (state.connection.mode === 'local' && state.questSite !== 'host') {
      try {
        const s = await go.GetSettings()
        s.questSite = state.questSite
        await go.PersistSettings(s)
        hostQuestSite = state.questSite
        state.questSite = 'host'
        await persist()
      } catch {}
    }
    // The game mode comes with TarkovTracker: the boss data follows it.
    // Another EFT account, profile or mode: the item shown is read again in the
    // mode now played, so its prices and progress are that profile's.
    window.mayakDesktop.on('status:update', (next) => {
      const tracker = host?.tracker,
        mode = host?.mode,
        identity = host?.identity
      host = { ...host, ...hostStatus(next) }
      if (host.tracker !== tracker || host.mode !== mode) void loadBosses()
      if (identity !== undefined && host.identity !== identity) void reloadItem()
      update()
    })
  }
  // The popup window closed by itself, or one of its buttons was pressed.
  window.mayakDesktop.on(
    'popup:closed',
    () =>
      void enqueue(async () => {
        if (!popup || Date.now() - popup.openedAt < 300) return
        popupClosed = { key: popup.key, at: Date.now() }
        popup = null
        update()
      }),
  )
  window.mayakDesktop.on(
    'popup:action',
    (type) =>
      void enqueue(async () => {
        if (type === 'toTab') await perform('popupToTab')
        else if (type === 'close') await perform('popupClose')
      }),
  )
  window.mayakDesktop.on('browser:item', (info) => void display({ event: 'browser:item', args: [info] }))
  // The update toast follows the updater; a found, downloading or downloaded
  // version shows until it is applied or put off.
  // Progress arrives once per percent; the bar redraws at most a few times a second.
  let updateRedraw = 0
  const updateChanged = (next) => {
    const shown = updateBarVisible(),
      before = updateStatus
    updateStatus = next
    const minor = before && before.state === next?.state && before.latest === next?.latest
    if (minor) {
      if (!updateRedraw)
        updateRedraw = setTimeout(() => {
          updateRedraw = 0
          update()
        }, 250)
    } else {
      clearTimeout(updateRedraw)
      updateRedraw = 0
      update()
    }
    if (updateBarVisible() !== shown) void show()
  }
  window.mayakDesktop.on('update:status', updateChanged)
  go.GetUpdateStatus?.()
    .then(updateChanged)
    .catch(() => {})
  // A new screenshot shows in the sidebar (and on the screenshot page).
  window.mayakDesktop.on('browser:screenshot', () => void loadShots())
  window.mayakDesktop.on('snapnote:changed', () => void loadSnaps())
  window.mayakDesktop.on('menu:choice', (choice) => onMenu(String(choice?.id || '')))
  // The map view: a nightly feature (SquadAvailable). The squad joined
  // before is joined again; a build without it drops its tab.
  window.mayakDesktop.on('squad:state', (next) => {
    squad.state = next || null
    update()
  })
  try {
    squad.available = !!(await go.SquadAvailable())
  } catch {}
  if (!squad.available) state.tabs = state.tabs.filter((t) => t.kind !== 'livemap' && t.kind !== 'squad')
  else {
    if (state.tabs.find((t) => t.id === state.active)?.kind === 'livemap') void loadSquadMaps()
    if (state.squadCode && state.squadName)
      go.SquadSetColor(state.squadColor)
        .then(() => go.SquadJoin(state.squadCode, state.squadName))
        .then(() => go.SquadState())
        .then((s) => {
          squad.state = s || null
          update()
        })
        .catch(() => {})
  }
  void loadSnaps()
  void loadShots()
  void loadBosses()
  setInterval(() => {
    if (document.visibilityState === 'visible') void loadBosses()
  }, bossesEvery)
  // A page's address and title show at once, not after the actions queued
  // before (an item reload at start-up waits for the network); only saving
  // them waits its turn.
  window.mayakDesktop.on('browser:navigation', (event) => {
    if (event.popup) {
      if (webURL(event.url)) void enqueue(() => perform('open', event.url))
      return
    }
    if (event.id !== popupID && !!event.loading !== loadingViews.has(event.id)) {
      if (event.loading) loadingViews.add(event.id)
      else loadingViews.delete(event.id)
      update()
    }
    // Following links inside the popup keeps its title and address current.
    if (event.id === popupID) {
      if (popup && webURL(event.url)) {
        popup.url = pageURL(event.url)
        popup.title = String(event.title || popup.title).slice(0, 160)
      }
      return
    }
    const tab = state.tabs.find((t) => t.id === event.id)
    if (!tab || !webURL(event.url)) return
    tab.url = pageURL(event.url)
    tab.title = String(event.title || tab.title || tab.url).slice(0, 160)
    // A new page drops the old favicon until its own one is known.
    if (event.favicon) {
      tab.favicon = webURL(event.favicon) || undefined
      rememberFavicon(state, tab.url, tab.favicon)
      loadFavicon(tab.favicon, true)
    } else if (hostname(tab.faviconPage || '') !== hostname(tab.url)) delete tab.favicon
    tab.faviconPage = tab.url
    tab.canBack = !!event.canBack
    tab.canForward = !!event.canForward
    views.set(tab.id, tab.url)
    update()
    void enqueue(persist)
  })
  for (const url of new Set([...Object.values(state.favicons), ...state.tabs.map((t) => t.favicon)])) loadFavicon(url)
  void show().catch(messageError)
  return snapshot()
})()
async function perform(type, data) {
  const tab = state.tabs.find((t) => t.id === state.active)
  switch (type) {
    case 'state':
      return snapshot()
    case 'newTab': {
      if (state.tabs.length >= 80) throw new Error('tab-limit')
      const item = { id: randomUUID(), kind: 'blank' }
      state.tabs.push(item)
      state.active = item.id
      break
    }
    case 'menu':
    case 'settings':
      if (tab?.kind !== 'settings') returnTo = state.active
      openLocal(state, 'settings')
      break
    // Settings opened at a section (an indicator pressed).
    case 'settingsAt':
      if (tab?.kind !== 'settings') returnTo = state.active
      openLocal(state, 'settings')
      if (browserSections.includes(data) || (platform === 'windows' && hostSections.includes(data))) section = data
      break
    // Closing settings returns to the tab it was opened from, or the map.
    case 'closeSettings': {
      const back =
        state.tabs.find((t) => t.id === returnTo && t.kind !== 'settings') ||
        state.tabs.find((t) => t.kind === 'livemap') ||
        state.tabs.find((t) => t.kind !== 'settings')
      if (back) state.active = back.id
      break
    }
    case 'bookmarks':
      openLocal(state, 'bookmarks')
      break
    case 'tabsPage':
      openLocal(state, 'tabs')
      break
    // The screenshot page: all of them (from the sidebar's heading), or one
    // shown large (from its thumbnail); screenshotView pages through them.
    case 'bosses':
      openLocal(state, 'bosses')
      void loadBosses()
      break
    case 'goonReportOpen': {
      // The raid and profile lookup may go to the network; it fills the dialog
      // when it returns rather than holding the queue.
      goonReport = { raid: null, identities: [], busy: true, done: false, error: '' }
      void (async () => {
        let info = null,
          error = ''
        try {
          info = await go.BrowserGoonReportInfo()
        } catch (e) {
          error = String(e?.message || e)
        }
        if (!goonReport || goonReport.done) return
        goonReport = {
          raid: info?.raid || null,
          identities: Array.isArray(info?.identities) ? info.identities : [],
          busy: false,
          done: false,
          error,
        }
        update()
      })()
      return snapshot()
    }
    case 'goonReportClose':
      goonReport = null
      return snapshot()
    case 'goonReportSend': {
      if (!goonReport || goonReport.busy) return snapshot()
      goonReport = { ...goonReport, busy: true, error: '' }
      update()
      const report = {
        map: String(data?.map || ''),
        accountId: String(data?.accountId || ''),
        mode: String(data?.mode || ''),
        startedAt: String(data?.startedAt || ''),
      }
      void (async () => {
        try {
          await go.BrowserReportGoons(report)
          if (goonReport) goonReport = { ...goonReport, busy: false, done: true }
          // tarkov.dev adds reports to its data every ten minutes.
          setTimeout(() => void loadBosses(), 11 * 60 * 1000)
        } catch (e) {
          if (goonReport) goonReport = { ...goonReport, busy: false, error: String(e?.message || e) }
        }
        update()
      })()
      return snapshot()
    }
    // A menu over the page, in the menu window (app_menu.go).
    case 'menuShow': {
      if (platform !== 'windows') return snapshot()
      const theme = document.documentElement.dataset.theme || ''
      await go.BrowserMenuShow({
        id: String(data?.id || ''),
        x: Math.round(data?.x || 0),
        y: Math.round(data?.y || 0),
        width: Math.round(data?.width || 280),
        items: Array.isArray(data?.items) ? data.items : [],
        theme,
        language: state.language,
      })
      return snapshot()
    }
    case 'snapnotes':
      openLocal(state, 'snapnotes')
      snaps.open = null
      void loadSnaps()
      break
    // A capture shows the page first (a menu may have hidden it), then opens
    // the new note for drawing.
    case 'snapCapture': {
      if (!snapsAvailable() || tab?.kind !== 'web' || snaps.busy) return snapshot()
      overlay = false
      await show()
      await new Promise((resolve) => setTimeout(resolve, 150))
      snaps.busy = true
      update()
      let note
      try {
        note = await go.SnapNoteCapture(
          tab.id,
          originalURL(tab.url) || tab.url,
          tab.title || '',
          !!data?.full,
          isTranslated(tab.url),
        )
      } catch (e) {
        snaps.busy = false
        const shown = !!error
        error = t(state.language, 'snapCaptureFailed') + String(e?.message || e)
        update()
        if (!shown) void show().catch(() => {})
        return snapshot()
      }
      snaps.busy = false
      openLocal(state, 'snapnotes')
      await openSnap(note.id)
      void loadSnaps()
      break
    }
    // A screenshot of the game becomes a note of its own, opened for drawing.
    case 'snapFromShot': {
      const name = String(data || ''),
        shot = shots.list.find((s) => s.name === name)
      const when = shot?.time
        ? new Date(shot.time).toLocaleString(state.language === 'ja' ? 'ja-JP' : 'en-US', {
            year: 'numeric',
            month: '2-digit',
            day: '2-digit',
            hour: '2-digit',
            minute: '2-digit',
          })
        : ''
      const note = await go.SnapNoteFromScreenshot(
        name,
        [t(state.language, 'screenshots'), when].filter(Boolean).join(' '),
      )
      shots.viewing = ''
      openLocal(state, 'snapnotes')
      await openSnap(note.id)
      void loadSnaps()
      break
    }
    case 'snapOpen':
      openLocal(state, 'snapnotes')
      await openSnap(String(data || ''))
      break
    case 'snapClose':
      snaps.open = null
      void loadSnaps()
      return snapshot()
    case 'snapFilter':
      snaps.filter = ['all', 'favorite', 'linked', 'single'].includes(data) ? data : 'all'
      return snapshot()
    case 'snapSave': {
      const note = await go.SnapNoteSave(
        String(data?.id || ''),
        String(data?.title || ''),
        JSON.stringify(data?.strokes ?? []),
        String(data?.thumb || ''),
      )
      if (snaps.open?.note.id === note.id) {
        snaps.open.note = { ...snaps.open.note, ...note, strokes: undefined }
        snaps.open.strokes = data.strokes
      }
      return snapshot()
    }
    // Sharing a note's picture (a PNG data URL, the drawing included): onto
    // the clipboard, or into a file the user names (the path comes back as
    // snapExported, "" when cancelled).
    case 'snapCopyImage':
      await go.SnapNoteCopyImage(String(data?.image || ''))
      return snapshot()
    // A new post on X in the browser, with the text given (the picture is
    // pasted there: X takes none from a link).
    case 'snapOpenPost':
      void Browser.OpenURL('https://x.com/intent/post?text=' + encodeURIComponent(String(data || '')))
      return snapshot()
    case 'snapExport': {
      const path = await go.SnapNoteExport(String(data?.image || ''), String(data?.name || ''))
      return { ...snapshot(), snapExported: path }
    }
    // The fonts installed, for the text of snap notes: asked once.
    case 'snapFonts':
      if (!snaps.fonts) snaps.fonts = (await go.SystemFonts()) || []
      return snapshot()
    // The map view could not make its picture (view-map.js snapMap).
    case 'mapSnapFailed':
      error = t(state.language, 'mapSnapFailed') + String(data || '')
      return snapshot()
    case 'snapNew': {
      const note = await go.SnapNoteCreate(String(data?.image || ''), String(data?.title || ''))
      // A note of the map carries your lines as its first layer (view-map.js).
      if (data?.drawing?.layers?.some((l) => l.strokes?.length))
        await go.SnapNoteSave(note.id, note.title, JSON.stringify(data.drawing), '')
      openLocal(state, 'snapnotes')
      await openSnap(note.id)
      void loadSnaps()
      break
    }
    case 'snapLink': {
      const note = await go.SnapNoteLink(String(data?.id || ''), !!data?.linked)
      if (snaps.open?.note.id === note.id) snaps.open.note = { ...snaps.open.note, linked: note.linked }
      void loadSnaps()
      return snapshot()
    }
    case 'snapFavorite': {
      const note = await go.SnapNoteFavorite(String(data?.id || ''), !!data?.favorite)
      if (snaps.open?.note.id === note.id) snaps.open.note = { ...snaps.open.note, favorite: !!note.favorite }
      snaps.list = snaps.list.map((n) => (n.id === note.id ? { ...n, favorite: !!note.favorite } : n))
      void loadSnaps()
      return snapshot()
    }
    // A note's position (from a game screenshot): its map chosen, or the
    // position shown on the tarkov.dev map (app_snapnote.go).
    case 'snapSetMap': {
      const note = await go.SnapNoteSetMap(String(data?.id || ''), String(data?.map || ''))
      if (snaps.open?.note.id === note.id) snaps.open.note = { ...snaps.open.note, spot: note.spot }
      void loadSnaps()
      return snapshot()
    }
    case 'snapShowSpot':
      await go.SnapNoteShowSpot(String(data || ''))
      return snapshot()
    case 'snapDelete': {
      const id = String(data || '')
      await go.SnapNoteDelete(id)
      if (snaps.open?.note.id === id) snaps.open = null
      await loadSnaps()
      return snapshot()
    }
    case 'snapOpenPage': {
      const note = snaps.open?.note.id === data ? snaps.open.note : snaps.list.find((n) => n.id === data)
      if (!note?.url) return snapshot()
      return perform('openOrFocus', note.url)
    }
    case 'livemap':
      if (!squad.available) return snapshot()
      openLocal(state, 'livemap')
      void loadSquadMaps()
      break
    // The squad's page (view-squad.js): its code, name, colour, members.
    case 'squadPage':
      if (!squad.available) return snapshot()
      openLocal(state, 'squad')
      void loadSquadMaps()
      break
    case 'squadColor':
      state.squadColor = squadColors.includes(data) ? data : ''
      await go.SquadSetColor(state.squadColor)
      shareSquad()
      break
    case 'squadCreate':
      await squadJoin(await go.SquadNewCode(), data?.name)
      break
    case 'squadJoin':
      await squadJoin(data?.code, data?.name)
      break
    case 'squadLeave':
      await go.SquadLeave()
      squad.state = null
      state.squadCode = ''
      shareSquad()
      break
    case 'squadRename': {
      const name = String(data || '')
        .trim()
        .slice(0, 24)
      if (!name) return snapshot()
      state.squadName = name
      await go.SquadRename(name)
      break
    }
    case 'squadCopy':
      await Clipboard.SetText(state.squadCode || '')
      return snapshot()
    case 'mapHidden':
      state.mapHidden = hiddenOf(data)
      break
    case 'mapCollapsed':
      state.mapCollapsed = hiddenOf(data)
      break
    case 'mapSettings':
      state.mapSettings = mapSettingsOf({ ...state.mapSettings, ...data })
      break
    case 'mapMarkers': {
      // By map, language and game mode (auto: the one played).
      const mode = state.mapSettings?.mode || 'auto'
      const key = `${data?.map}|${state.language}|${mode}`
      if (!squad.markers[key])
        squad.markers[key] = (await go.BrowserMapMarkers(
          String(data?.map || ''),
          state.language,
          mode === 'auto' ? '' : mode,
        )) || {
          layers: [],
          markers: [],
        }
      update()
      return snapshot()
    }
    case 'squadMapImage': {
      // By map, floor and how faded the ground is under a floor.
      const fade = Math.round(Number(data?.fade) || 0)
      const key = `${data?.map}|${data?.layer || ''}|${fade}`
      if (!squad.images[key])
        squad.images[key] = await go.BrowserSquadMapImage(String(data?.map || ''), String(data?.layer || ''), fade)
      update()
      return snapshot()
    }
    case 'screenshots':
      openLocal(state, 'screenshots')
      shots.viewing = ''
      void loadShots()
      break
    case 'screenshotOpen':
      openLocal(state, 'screenshots')
      await changed()
      void loadShots()
      void viewShot(String(data || ''))
      return snapshot()
    case 'screenshotView':
      void viewShot(String(data || ''))
      return snapshot()
    case 'screenshotFolder':
      try {
        await go.OpenScreenshotDirectory()
      } catch {}
      return snapshot()
    case 'home':
      goHome(state, tab?.id)
      break
    case 'bookmarkTab':
      bookmarkTab(state, data?.id, data?.before ?? null)
      break
    // A shell menu that reaches over the page hides the native page view meanwhile.
    case 'overlay':
      overlay = !!data
      await show()
      return snapshot()
    case 'sidebarWidth':
      state.sidebarWidth = clampSidebar(data)
      await show()
      return snapshot()
    case 'itemPanelWidth':
      state.itemPanelWidth = clampItemPanel(data)
      await show()
      return snapshot()
    case 'itemPanelHeight':
      state.itemPanelHeight = clampItemPanelHeight(data)
      await show()
      return snapshot()
    case 'bookmarkPin': {
      const item = state.bookmarks.find((b) => b.id === (data?.id ?? data))
      if (item) pinBookmark(state, item.id, data?.pin ?? !item.sidebar, data?.before)
      break
    }
    case 'settingsSection':
      if (data === 'tasks' && platform === 'windows')
        try {
          hostQuestSite = (await go.GetSettings()).questSite || hostQuestSite
        } catch {}
      if (browserSections.includes(data) || (platform === 'windows' && hostSections.includes(data))) section = data
      break
    case 'activate':
      if (state.tabs.some((t) => t.id === data)) state.active = data
      break
    // openOrFocus goes to a tab already showing the address, if any, so a
    // link pressed twice (the changelog, About) does not open a second tab.
    case 'open':
    case 'navigate':
    case 'openOrFocus': {
      const url = webURL(data)
      if (!url) throw new Error(t(state.language, 'enterWebURL'))
      if (type === 'openOrFocus') {
        const same = state.tabs.find((t) => t.kind === 'web' && t.url === url)
        if (same) {
          state.active = same.id
          break
        }
      }
      if (type === 'navigate' && tab && ['web', 'blank'].includes(tab.kind)) {
        tab.kind = 'web'
        tab.url = url
        delete tab.task
      } else {
        if (state.tabs.length >= 80) throw new Error('tab-limit')
        const item = { id: randomUUID(), kind: 'web', url }
        state.tabs.push(item)
        state.active = item.id
      }
      break
    }
    case 'close': {
      const index = state.tabs.findIndex((t) => t.id === data && !t.pinned && !t.fixed)
      if (index < 0) break
      await native('close', { id: data })
      views.delete(data)
      loadingViews.delete(data)
      const [closed] = state.tabs.splice(index, 1)
      if (closed.kind === 'web') {
        closedTabs.push({ tab: closed, index })
        if (closedTabs.length > 20) closedTabs.shift()
      }
      if (state.active === data) state.active = state.tabs[Math.min(index, state.tabs.length - 1)]?.id
      if (!state.tabs.length) state.active = ''
      break
    }
    // Ctrl+Shift+T: the last closed page comes back where it was, as a new tab.
    case 'reopenTab': {
      const last = closedTabs.pop()
      if (!last) return snapshot()
      if (state.tabs.length >= 80) throw new Error('tab-limit')
      const next = { ...last.tab, id: randomUUID() }
      delete next.pinned
      const before = state.tabs.slice(last.index).find((t) => !t.fixed)?.id ?? null
      state.tabs.push(next)
      moveTab(state, next.id, before)
      state.active = next.id
      break
    }
    case 'cycleTab':
      if (!cycleTab(state, data < 0 ? -1 : 1)) return snapshot()
      break
    case 'tabAt':
      if (!tabAt(state, Number(data))) return snapshot()
      break
    // Keyboard focus moves to the shell (its address bar) or to the page shown;
    // the page views are native windows, so the shell cannot do it itself.
    case 'focus':
      if (data === 'shell') await native('focus', { id: 'shell' })
      else if (tab?.kind === 'web') await native('focus', { id: tab.id })
      return snapshot()
    case 'pin':
      togglePin(state, data)
      break
    case 'move':
      moveTab(state, data?.id, data?.before ?? null)
      break
    case 'preferences': {
      const next = restore({ ...state, ...data })
      state.language = next.language
      state.tutorialDone = next.tutorialDone
      state.layout = next.layout
      state.sidebarSide = next.sidebarSide
      state.theme = next.theme
      state.clock = next.clock
      state.sidebarCollapsed = next.sidebarCollapsed
      state.bookmarksCollapsed = next.bookmarksCollapsed
      state.screenshotsCollapsed = next.screenshotsCollapsed
      state.snapNotesCollapsed = next.snapNotesCollapsed
      state.squadCollapsed = next.squadCollapsed
      state.toolOrder = next.toolOrder
      state.bossesView = next.bossesView
      state.bossMap = next.bossMap
      state.bossMode = next.bossMode
      if (data && 'bossMode' in data) void loadBosses()
      if (data && 'language' in data) void loadBosses()
      state.sidebarWidth = next.sidebarWidth
      state.itemPanelWidth = next.itemPanelWidth
      state.itemPanelHeight = next.itemPanelHeight
      state.itemDock = next.itemDock
      state.bookmarkView = next.bookmarkView
      state.taskMode = next.taskMode
      state.questSite = next.questSite
      state.translateWiki = next.translateWiki
      // Blocking applies to new requests; reload so the visible page matches the setting.
      if (next.adblock !== state.adblock) {
        state.adblock = next.adblock
        await go.BrowserSetAdblock(state.adblock)
        if (tab?.kind === 'web') await native('reload', { id: tab.id })
      }
      break
    }
    case 'connection': {
      if (data.receive !== undefined) state.connection.receive = receiveOf(data.receive)
      if (data.mode !== undefined) await go.BrowserSetMode(data.mode)
      if (['local', 'client', 'off'].includes(data.mode) && (data.mode !== 'local' || platform === 'windows')) {
        // A pairing being made ends; the one kept runs while the mode is
        // its side.
        forgetPairCode()
        clearTimeout(expiry)
        pending = null
        state.connection.mode = data.mode
        peerState = { phase: 'idle' }
        startLink()
      }
      break
    }
    case 'bookmark': {
      const url = webURL(data.url)
      if (!url) throw new Error('invalid-url')
      const item = {
        id: data.id || randomUUID(),
        name: String(data.name || url).slice(0, 150),
        url,
        group: bookmarkGroup(data.group),
      }
      const index = state.bookmarks.findIndex((b) => b.id === item.id)
      if (index >= 0) {
        if (state.bookmarks[index].sidebar) item.sidebar = true
        state.bookmarks[index] = item
      } else if (state.bookmarks.length < 100) state.bookmarks.push(item)
      break
    }
    case 'deleteBookmark':
      state.bookmarks = state.bookmarks.filter((b) => b.id !== data)
      break
    // A task tab kept from before has the pages it opened with; the task
    // list at hand may know better ones (the Japanese wiki needs the trader,
    // which a task new to the wiki gets later), so they are asked first.
    case 'site':
      if (!tab?.task) break
      try {
        const fresh = await go.QuestSiteURLs(String(tab.task.id || ''), String(tab.task.name || ''))
        for (const [key, url] of Object.entries(fresh || {})) if (webURL(url)) tab.task.urls[key] = url
      } catch {}
      if (webURL(tab.task.urls[data])) tab.url = tab.task.urls[data]
      break
    case 'back':
    case 'forward':
    case 'reload':
      if (tab?.kind === 'web') await native(type, { id: tab.id })
      return snapshot()
    // The page opens through Google Translate's proxy, or back as itself.
    case 'translate': {
      if (tab?.kind !== 'web' || tab.fixed) return snapshot()
      const url = isTranslated(tab.url) ? originalURL(tab.url) : translatedURL(tab.url, state.language)
      if (!url || url === tab.url) return snapshot()
      tab.url = url
      break
    }
    case 'windowTheme':
      try {
        await go.BrowserSetWindowTheme(data.caption, data.text, data.border, !!data.dark)
      } catch {}
      return snapshot()
    // A task marker of the map view opens its task in a tab as a recognized
    // task does: the task site set (the Host's on the Host) and the tab setting.
    case 'mapTask': {
      const id = String(data?.id || '')
      const urls = id ? await go.QuestSiteURLs(id, '') : null
      if (!urls || !Object.keys(urls).length) return snapshot()
      const site = state.connection.mode === 'local' ? hostQuestSite : 'tarkov-dev'
      if (!receiveTask(state, { id, name: String(data?.name || id).slice(0, 200), site, urls })) return snapshot()
      break
    }
    // A task in the item sidebar opens like a recognized task, on the task site
    // set now (on the Host its current setting, not the one when the item came).
    case 'itemTask': {
      const task = item?.tasks.find((t) => t.id === (data?.id ?? data))
      if (!task) return snapshot()
      const site = itemSite(),
        url = webURL(task.urls[site])
      if (!url) return snapshot()
      await openPopup({
        key: 'task:' + task.id,
        url,
        title: task.name,
        anchor: anchorY(data),
        task: { id: task.id, name: task.name, site, urls: task.urls },
      })
      return snapshot()
    }
    case 'itemPage': {
      const url = webURL(data?.url)
      if (!item || !url) return snapshot()
      await openPopup({ key: 'item:' + item.id, url, title: item.name, anchor: anchorY(data) })
      return snapshot()
    }
    case 'popupClose':
      if (popup) await closePopup()
      return snapshot()
    // "Open in a tab" moves the popup's current page into a tab of its own.
    case 'popupToTab': {
      if (!popup) return snapshot()
      if (state.tabs.length >= 80) throw new Error('tab-limit')
      const next = { id: randomUUID(), kind: 'web', url: popup.url, title: popup.title }
      if (popup.task) next.task = structuredClone(popup.task)
      state.tabs.push(next)
      state.active = next.id
      await closePopup()
      break
    }
    case 'monitor': {
      if (platform !== 'windows' || state.connection.mode !== 'local') return snapshot()
      if (host?.monitoring) {
        await go.StopMonitoring()
        return snapshot()
      }
      // Without a Screenshots folder there is nothing to watch: the folder
      // settings open instead, where it is chosen (or found again).
      let folder = ''
      try {
        folder = String((await go.GetSettings()).screenshotDirectory || '')
      } catch {}
      if (!folder) {
        section = 'folders'
        if (tab?.kind !== 'settings') returnTo = state.active
        openLocal(state, 'settings')
        break
      }
      await go.StartMonitoring()
      return snapshot()
    }
    // The task site is the Host's setting (it also decides what goes to
    // tarkov.dev Remote Control); the browser follows it.
    case 'hostQuestSite': {
      if (platform !== 'windows' || !['tarkov-dev', 'official-wiki', 'japanese-wiki'].includes(data)) return snapshot()
      const s = await go.GetSettings()
      s.questSite = data
      await go.PersistSettings(s)
      hostQuestSite = data
      state.questSite = 'host'
      break
    }
    case 'itemClose':
      itemOpen = false
      update()
      await persist()
      await show()
      return snapshot()
    // The item sidebar opens without an item too: it has a search box.
    case 'itemOpen':
      itemOpen = true
      update()
      await persist()
      await show()
      return snapshot()
    case 'itemSearch': {
      const query = String(data ?? '').slice(0, 80),
        seq = ++searchSeq
      itemSearch = { ...itemSearch, query }
      if (!query.trim()) {
        itemSearch.results = []
        update()
        return snapshot()
      }
      void (async () => {
        let hits
        try {
          hits = await go.BrowserItemSearch(query)
        } catch (e) {
          messageError(e)
          return
        }
        // A later keystroke's search wins over this one.
        if (seq === searchSeq)
          itemSearch = {
            query,
            results: (Array.isArray(hits) ? hits : [])
              .slice(0, 20)
              .map((h) => ({
                id: String(h?.id || '').slice(0, 40),
                name: String(h?.name || '').slice(0, 160),
                shortName: String(h?.shortName || '').slice(0, 60),
                names: names(h?.names),
                iconUrl: webURL(h?.iconUrl) || '',
              }))
              .filter((h) => h.id && h.name),
          }
        update()
      })()
      return snapshot()
    }
    case 'itemSelect': {
      const id = String(data || '')
      void (async () => {
        let next
        try {
          next = itemInfo(await go.BrowserItemInfo('', id))
        } catch (e) {
          messageError(e)
          return
        }
        if (next) {
          item = next
          itemOpen = true
          itemSearch = { query: '', results: [] }
          void loadHistory()
          await persist().catch(() => {})
        }
        update()
        await show()
      })()
      return snapshot()
    }
    // data.auto: the periodic refresh, which keeps the shown item quietly when
    // the catalog cannot be reached (the refresh button reports it).
    case 'itemRefresh': {
      if (!item || itemBusy) return snapshot()
      // The fetch runs outside the action queue (see refreshItem).
      void refreshItem(!!data?.auto)
      return snapshot()
    }
    // Applying restarts the app and downloading takes a while: neither holds the queue.
    case 'updateInstall':
      void go.InstallUpdate().catch(messageError)
      return snapshot()
    case 'updateDownload':
      void go.DownloadUpdate().catch(messageError)
      return snapshot()
    case 'openNotices':
      void go.OpenThirdPartyNotices().catch(messageError)
      return snapshot()
    case 'updateCheck':
      void go.CheckForUpdates().catch(messageError)
      return snapshot()
    // The update channel (updateChannel), shown under About: this PC's
    // updater, on every platform and in every mode.
    case 'updateChannel': {
      const channel = typeof data === 'string' ? data : data?.channel
      if (!['stable', 'nightly'].includes(channel)) return snapshot()
      const s = await go.GetSettings()
      s.updateChannel = channel
      await go.PersistSettings(s)
      updateChannel = channel
      return snapshot()
    }
    case 'updateDismiss':
      updateDismissed = updateStatus?.latest || ''
      await show()
      return snapshot()
    case 'dismiss':
      error = ''
      update()
      await show()
      return snapshot()
    default:
      return snapshot()
  }
  await changed()
  return snapshot()
}
window.mayak = {
  onState(fn) {
    notify = fn
  },
  onKey(fn) {
    onKey = fn
  },
  onMenu(fn) {
    onMenu = fn
  },
  async action(type, data) {
    await ready
    if (type.startsWith('peer')) return pairing(type, data)
    return enqueue(() => perform(type, data))
  },
  // request is action for a caller that must know of a failure (a save): the
  // error shows in the error bar and is thrown to the caller too.
  async request(type, data) {
    await ready
    const result = queue.then(() => perform(type, data))
    queue = result.catch(messageError)
    return result
  },
}
window.addEventListener('beforeunload', () => peer.stop())
