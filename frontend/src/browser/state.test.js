import test from 'node:test'
import assert from 'node:assert/strict'
import {
  liveMapTabID,
  rememberFavicon,
  bookmarkGroup,
  defaults,
  restore,
  receiveTask,
  samePage,
  receiveMap,
  recentSquadsOf,
  rememberSquad,
  linkKind,
  moveTab,
  togglePin,
  pinBookmark,
  bookmarkTab,
  goHome,
  openLocal,
  webURL,
  pageURL,
  resolveAddress,
  searchURL,
  shortcut,
  tabAt,
  cycleTab,
  receivePosition,
  translatedURL,
  originalURL,
  siteOfURL,
  isTranslated,
} from './state.js'
import { encode, decode } from './peer-code.js'
test('curated bookmarks merge once and preserve user choices', () => {
  const existing = { id: 'custom-market', name: 'My prices', url: 'https://tarkov-market.com', group: 'other' }
  const s = restore({ bookmarks: [existing] })
  // The user's bookmark, the two added once since (revision 1) and the two
  // pinned to the sidebar (revision 3).
  assert.equal(s.bookmarks.length, 5)
  assert.deepEqual(
    s.bookmarks.find((b) => b.id === existing.id),
    { ...existing, url: existing.url + '/' },
  )
  assert.equal(
    s.bookmarks.some((b) => b.id === 'maps'),
    false,
  )
  s.bookmarks = s.bookmarks.filter((b) => b.id !== 'eft-ammo')
  assert.deepEqual(restore(JSON.parse(JSON.stringify(s))).bookmarks, s.bookmarks)
  const fresh = defaults()
  fresh.bookmarks = []
  assert.deepEqual(restore(fresh).bookmarks, [])
  const full = Array.from({ length: 100 }, (_, i) => ({
    id: `custom-${i}`,
    name: `Custom ${i}`,
    url: `https://example.com/${i}`,
    group: 'other',
  }))
  assert.deepEqual(restore({ bookmarks: full }).bookmarks, full)
})
const task = (id) => ({
  id,
  name: id,
  site: 'official-wiki',
  urls: {
    'tarkov-dev': `https://tarkov.dev/task/${id}`,
    'official-wiki': `https://escapefromtarkov.fandom.com/wiki/${id}`,
    'japanese-wiki': `https://wikiwiki.jp/eft/Prapor/${id}`,
  },
})
test('a task page translated since is the same task page', () => {
  const s = defaults()
  s.taskMode = 'new'
  s.questSite = 'official-wiki'
  const first = receiveTask(s, task('Debut'))
  // Translated with the toolbar button (Google adds its own parameters).
  first.url =
    'https://escapefromtarkov-fandom-com.translate.goog/wiki/Debut?_x_tr_sl=auto&_x_tr_tl=ja&_x_tr_hl=ja&_x_tr_pto=wapp#Objectives'
  const again = receiveTask(s, task('Debut'))
  assert.equal(again.id, first.id)
  assert.match(again.url, /translate\.goog/)
  // Opened translated (the setting), found again untranslated or not.
  s.translateWiki = true
  const other = receiveTask(s, task('Checking'))
  other.url += '&_x_tr_pto=wapp'
  assert.equal(receiveTask(s, task('Checking')).id, other.id)
  assert.notEqual(receiveTask(s, task('Search')).id, other.id)
})
test('a tab already showing a task page, not opened for the task, becomes its tab', () => {
  const s = defaults()
  s.taskMode = 'new'
  s.questSite = 'official-wiki'
  // Opened from a bookmark, translated, with a trailing slash.
  const opened = {
    id: 'b1',
    kind: 'web',
    url: 'https://escapefromtarkov-fandom-com.translate.goog/wiki/Debut/?_x_tr_sl=auto&_x_tr_tl=ja',
  }
  s.tabs.push(opened)
  const count = s.tabs.length
  const tab = receiveTask(s, task('Debut'))
  assert.equal(tab.id, 'b1')
  assert.equal(tab.task.id, 'Debut')
  assert.match(tab.url, /translate.goog/)
  assert.equal(s.tabs.length, count)
  assert.equal(samePage('https://tarkov.dev/item/ledx/', 'https://tarkov.dev/item/ledx#price'), true)
  assert.equal(samePage('https://tarkov.dev/item/ledx', 'https://tarkov.dev/item/ledx-2'), false)
})
test('reuse preserves pinned tasks and deduplicates repeated detections', () => {
  const s = defaults()
  s.taskMode = 'reuse'
  const first = receiveTask(s, task('Debut'))
  first.pinned = true
  const second = receiveTask(s, task('Checking'))
  assert.notEqual(first.id, second.id)
  assert.equal(receiveTask(s, task('Checking')).id, second.id)
  assert.equal(receiveTask(s, task('Search')).id, second.id)
  assert.equal(first.task.id, 'Debut')
  assert.equal(s.tabs.length, 4)
})
test('new-tab mode and selected wiki survive restoration', () => {
  const s = defaults()
  s.taskMode = 'new'
  s.questSite = 'japanese-wiki'
  const a = receiveTask(s, task('Debut'))
  const b = receiveTask(s, task('Checking'))
  assert.notEqual(a.id, b.id)
  assert.match(b.url, /wikiwiki.jp/)
  assert.deepEqual(
    restore(JSON.parse(JSON.stringify(s))).tabs.map((t) => t.id),
    s.tabs.map((t) => t.id),
  )
})
test('settings can close and reopen without duplication', () => {
  const s = defaults()
  s.tabs = s.tabs.filter((t) => t.kind !== 'settings')
  openLocal(s, 'settings')
  openLocal(s, 'settings')
  assert.equal(s.tabs.filter((t) => t.kind === 'settings').length, 1)
})
test('invalid incoming sites, internal protocols and traversal IDs are rejected', () => {
  for (const u of [
    'javascript:alert(1)',
    'file:///etc/passwd',
    'https://user:pass@example.com',
    'http://wails.localhost/settings.html',
    'wails://wails/',
  ])
    assert.equal(webURL(u), null)
  const s = defaults()
  const bad = task('x')
  bad.urls['japanese-wiki'] = 'file:///tmp/a'
  assert.equal(receiveTask(s, bad), null)
  assert.equal(receiveMap(s, '../settings'), null)
  assert.deepEqual(
    restore({ tabs: [{ id: '../../outside', kind: 'web', url: 'https://tarkov.dev' }] }).tabs.map((t) => t.id),
    [],
  )
})
test('a detected map or position brings the map view forward; the fixed views are gone', () => {
  const s = defaults()
  assert.deepEqual(
    s.tabs.map((t) => t.kind),
    ['livemap', 'settings'],
  )
  assert.equal(s.active, liveMapTabID)
  openLocal(s, 'settings')
  assert.equal(receiveMap(s, 'ground-zero-21').kind, 'livemap')
  assert.equal(s.active, liveMapTabID)
  openLocal(s, 'settings')
  assert.equal(receivePosition(s, 'customs').id, liveMapTabID)
  assert.equal(receivePosition(s, 'Bad Map'), null)
  assert.equal(s.tabs.filter((t) => t.kind === 'livemap').length, 1)
  // A browser saved with the fixed tarkov.dev and TarkovTracker views drops them.
  const restored = restore({
    tabs: [
      { id: 'x', kind: 'blank' },
      { id: 'map', kind: 'web', url: 'https://tarkov.dev/map/customs?connection=AB12' },
      { id: 'tracker', kind: 'web', url: 'https://tarkovtracker.org/' },
      { id: 'old', kind: 'web', role: 'map', url: 'https://tarkov.dev/map/woods' },
    ],
    active: 'map',
  })
  assert.deepEqual(
    restored.tabs.map((t) => t.id),
    ['x', 'old'],
  )
  assert.equal(restored.tabs[1].role, undefined)
  assert.equal(restored.active, 'x')
})
test('incoming traffic cannot grow tabs without a limit', () => {
  const s = defaults()
  s.taskMode = 'new'
  for (let i = 0; i < 100; i++) receiveTask(s, task(String(i)))
  assert.equal(s.tabs.length, 80)
})
test('a pairing code carries a key for ten minutes and nothing malformed', () => {
  const code = {
    version: 2,
    type: 'link',
    id: '01234567-89ab-4cde-8fab-0123456789ab',
    createdAt: Date.now(),
    key: 'A'.repeat(43),
  }
  assert.deepEqual(decode(encode(code)), code)
  assert.throws(() => decode(encode(code), code.createdAt + 600001))
  assert.throws(() => encode({ ...code, key: 'short' }))
  assert.throws(() => encode({ ...code, version: 1 }))
  assert.throws(() => decode('MAYAK1.invalid'))
})

test('a saved Client mode from the WebRTC days and a kept pairing are restored', () => {
  const key = 'b'.repeat(43)
  const s = restore({ connection: { mode: 'webrtc', stun: 'stun:x', link: { key, role: 'client' } } })
  assert.deepEqual(s.connection.mode, 'client')
  assert.deepEqual(s.connection.link, { key, role: 'client' })
  assert.equal(restore({ connection: { mode: 'local', link: { key: 'bad', role: 'host' } } }).connection.link, null)
})

test('legacy home tabs are removed without losing bookmarks or web tabs', () => {
  const s = defaults()
  const web = receiveTask(s, task('Debut'))
  const restored = restore({ ...s, tabs: [{ id: 'home', kind: 'home' }, ...s.tabs], active: 'home' })
  assert.equal(
    restored.tabs.some((t) => t.kind === 'home'),
    false,
  )
  assert.ok(restored.tabs.some((t) => t.id === web.id))
  assert.deepEqual(restored.bookmarks, s.bookmarks)
  assert.deepEqual(restore({ tabs: [], active: '' }).tabs, [])
  assert.equal(restore({}).active, liveMapTabID)
})

test('pageURL strips the injected tarkov.dev connection only', () => {
  assert.equal(pageURL('https://tarkov.dev/map/customs?connection=AB12'), 'https://tarkov.dev/map/customs')
  assert.equal(pageURL('https://tarkov.dev/map/customs?connection=AB12&x=1'), 'https://tarkov.dev/map/customs?x=1')
  assert.equal(pageURL('https://example.com/?connection=AB12'), 'https://example.com/?connection=AB12')
  assert.equal(pageURL('javascript:alert(1)'), null)
})

test('ad blocking defaults on and keeps an explicit off', () => {
  assert.equal(defaults().adblock, true)
  assert.equal(restore({}).adblock, true)
  assert.equal(restore({ adblock: false }).adblock, false)
  assert.equal(restore(JSON.parse(JSON.stringify(restore({ adblock: false })))).adblock, false)
})

test('recognized tasks open in a new tab by default, but a saved reuse choice is kept', () => {
  assert.equal(defaults().taskMode, 'new')
  assert.equal(restore({}).taskMode, 'new')
  assert.equal(restore({ taskMode: 'reuse' }).taskMode, 'reuse')
  const s = defaults()
  const a = receiveTask(s, task('Debut'))
  const b = receiveTask(s, task('Checking'))
  assert.notEqual(a.id, b.id)
})

test('theme defaults to MAYAK Dark, maps the earlier names and rejects unknown themes', () => {
  assert.equal(defaults().theme, 'mayak-dark')
  assert.equal(restore({ theme: 'catppuccin-mocha' }).theme, 'catppuccin-mocha')
  assert.equal(restore({ theme: 'system' }).theme, 'system')
  assert.equal(restore({ theme: 'neon' }).theme, 'mayak-dark')
  assert.equal(restore({ theme: 'claude-light' }).theme, 'mayak-light')
})

test('tabs reorder before a tab or to the end', () => {
  const s = defaults()
  s.tabs.push({ id: 'a', kind: 'blank' }, { id: 'b', kind: 'blank' })
  const ids = () => s.tabs.map((t) => t.id)
  assert.equal(moveTab(s, 'b', 'settings'), true)
  assert.deepEqual(ids(), [liveMapTabID, 'b', 'settings', 'a'])
  assert.equal(moveTab(s, 'b', null), true)
  assert.deepEqual(ids(), [liveMapTabID, 'settings', 'a', 'b'])
  assert.equal(moveTab(s, 'a', 'missing'), false)
  assert.deepEqual(ids(), [liveMapTabID, 'settings', 'a', 'b'])
})

test('pinned tabs gather at the top and reorder only among themselves', () => {
  const s = defaults()
  s.tabs = ['a', 'b', 'c', 'd'].map((id) => ({ id, kind: 'web', url: 'https://example.com/' + id }))
  const ids = () => s.tabs.map((t) => t.id).join(',')
  togglePin(s, 'c')
  togglePin(s, 'd')
  assert.equal(ids(), 'c,d,a,b')
  assert.equal(moveTab(s, 'd', 'c'), true)
  assert.equal(ids(), 'd,c,a,b')
  moveTab(s, 'a', 'd')
  assert.equal(ids(), 'd,c,a,b') // cannot jump into the pinned group
  moveTab(s, 'd', null)
  assert.equal(ids(), 'c,d,a,b') // stays at the end of its group
  togglePin(s, 'c')
  assert.equal(ids(), 'd,c,a,b') // unpinned: top of the others
  assert.equal(togglePin(s, 'settings'), false)
  const restored = restore({
    tabs: [
      { id: 'x', kind: 'web', url: 'https://example.com/x' },
      { id: 'y', kind: 'web', url: 'https://example.com/y', pinned: true },
    ],
  })
  assert.equal(restored.tabs.map((t) => t.id).join(','), 'y,x')
})

test('bookmarks pin to the sidebar at a drop position and survive restoration', () => {
  const s = defaults()
  s.bookmarks = ['a', 'b', 'c', 'd'].map((id) => ({ id, name: id, url: 'https://example.com/' + id, group: 'other' }))
  const pinned = () =>
    s.bookmarks
      .filter((b) => b.sidebar)
      .map((b) => b.id)
      .join(',')
  pinBookmark(s, 'c', true, null)
  pinBookmark(s, 'a', true, null)
  assert.equal(pinned(), 'c,a')
  pinBookmark(s, 'd', true, 'c')
  assert.equal(pinned(), 'd,c,a')
  pinBookmark(s, 'a', true, 'd')
  assert.equal(pinned(), 'a,d,c') // reorder within the sidebar
  pinBookmark(s, 'd', false)
  assert.equal(pinned(), 'a,c')
  assert.equal(pinBookmark(s, 'missing', true, null), false)
  assert.equal(
    restore(JSON.parse(JSON.stringify(s)))
      .bookmarks.filter((b) => b.sidebar)
      .map((b) => b.id)
      .join(','),
    'a,c',
  )
})
test('sidebar and bookmark view preferences restore with safe defaults', () => {
  assert.equal(restore({}).sidebarCollapsed, false)
  assert.equal(restore({ sidebarCollapsed: true }).sidebarCollapsed, true)
  assert.equal(restore({}).bookmarkView, 'grid')
  assert.equal(restore({ bookmarkView: 'list' }).bookmarkView, 'list')
  const r = restore({
    tabs: [
      { id: 'b1', kind: 'bookmarks' },
      { id: 'b2', kind: 'bookmarks' },
    ],
  })
  assert.deepEqual(
    r.tabs.map((t) => t.id),
    ['b1'],
  )
})

test('sidebar width restores within its limits', () => {
  assert.equal(restore({}).sidebarWidth, 224)
  assert.equal(restore({ sidebarWidth: 300.4 }).sidebarWidth, 300)
  assert.equal(restore({ sidebarWidth: 20 }).sidebarWidth, 180)
  assert.equal(restore({ sidebarWidth: 9999 }).sidebarWidth, 420)
  assert.equal(restore({ sidebarWidth: 'wide' }).sidebarWidth, 224)
})

test('favicons are kept per site with a limit and only as web URLs', () => {
  const s = defaults()
  rememberFavicon(s, 'https://www.example.com/a', 'https://example.com/favicon.ico')
  rememberFavicon(s, 'https://evil.example/', 'javascript:alert(1)')
  assert.deepEqual(s.favicons, { 'example.com': 'https://example.com/favicon.ico' })
  for (let i = 0; i < 250; i++) rememberFavicon(s, 'https://s' + i + '.test/', 'https://s' + i + '.test/f.png')
  assert.equal(Object.keys(s.favicons).length, 200)
  assert.equal(s.favicons['s249.test'], 'https://s249.test/f.png')
  const r = restore({
    favicons: { ...s.favicons, 'bad host': 'https://x/' },
    tabs: [{ id: 't', kind: 'web', url: 'https://a.test/', favicon: 'file:///x' }],
  })
  assert.equal(r.favicons['bad host'], undefined)
  assert.equal(r.tabs[0].favicon, undefined)
})

test('bookmark categories accept typed names', () => {
  assert.equal(bookmarkGroup('  Boss   routes '), 'Boss routes')
  assert.equal(bookmarkGroup(''), 'other')
  assert.equal(bookmarkGroup(undefined), 'other')
  assert.equal(bookmarkGroup('x'.repeat(60)).length, 40)
  const r = restore({
    bookmarkRevision: 3,
    bookmarks: [
      { id: 'a', name: 'A', url: 'https://a.test/', group: '弾薬' },
      { id: 'b', name: 'B', url: 'https://b.test/', group: 'maps' },
    ],
  })
  assert.deepEqual(
    r.bookmarks.map((b) => b.group),
    ['弾薬', 'maps'],
  )
})

test('the bookmark section fold state is remembered', () => {
  assert.equal(restore({}).bookmarksCollapsed, false)
  assert.equal(restore({ bookmarksCollapsed: true }).bookmarksCollapsed, true)
  assert.equal(restore({}).screenshotsCollapsed, false)
  assert.equal(restore({ screenshotsCollapsed: true }).screenshotsCollapsed, true)
  assert.equal(restore({}).bossesView, 'full')
  assert.equal(restore({ bossesView: 'goons' }).bossesView, 'goons')
  assert.equal(restore({ bossesView: 'half' }).bossesView, 'full')
  assert.equal(restore({}).clock, '24')
  assert.equal(restore({}).bossMap, '')
  assert.equal(restore({ bossMap: 'customs', bossMode: 'pve' }).bossMap, 'customs')
  assert.equal(restore({ bossMap: '../x' }).bossMap, '')
  assert.equal(restore({ bossMode: 'pve' }).bossMode, 'pve')
  assert.equal(restore({ bossMode: 'arena' }).bossMode, '')
  assert.equal(restore({ clock: '12' }).clock, '12')
  assert.equal(restore({ clock: '13' }).clock, '24')
  assert.equal(restore({ bossesCollapsed: true }).bossesView, 'closed')
  assert.equal(
    restore({
      tabs: [
        { id: 'b1', kind: 'bosses' },
        { id: 'b2', kind: 'bosses' },
      ],
    }).tabs.filter((tab) => tab.kind === 'bosses').length,
    1,
  )
})

test('the map view, its filters and the squad come back, one tab at most', () => {
  const restored = restore({
    squadName: '  Alice ',
    squadCode: 'ABCD-1234',
    mapHidden: ['spawn_pmc', 'Bad Key!', 'spawn_pmc', 42],
    mapSettings: { snipers: false, extracts: 'yes' },
    tabs: [
      { id: 's1', kind: 'livemap' },
      { id: 's2', kind: 'livemap' },
    ],
  })
  assert.equal(restored.squadName, 'Alice')
  assert.equal(restored.squadCode, 'ABCD-1234')
  assert.deepEqual(restored.mapHidden, ['spawn_pmc'])
  assert.deepEqual(restored.mapSettings, {
    snipers: false,
    extracts: false,
    activeTasks: false,
    extractText: 100,
    labelText: 100,
    subtleLabels: false,
    fade: 20,
    style: 'svg',
    mode: 'auto',
    markerEffect: 'none',
    markerColor: '',
    markerShape: 'arrow',
    squadFit: true,
  })
  assert.equal(restore({ mapSettings: { extractText: 999, labelText: '75' } }).mapSettings.extractText, 200)
  assert.equal(restore({ mapSettings: { labelText: '75' } }).mapSettings.labelText, 75)
  assert.equal(restored.tabs.filter((tab) => tab.kind === 'livemap').length, 1)
  assert.equal(restore({ squadCode: 'abcd1234' }).squadCode, '')
  assert.equal(restore({ squadName: 42 }).squadName, '')
  assert.deepEqual(restore({}).mapHidden, [])
  assert.deepEqual(restore({}).mapSettings, {
    snipers: true,
    extracts: false,
    activeTasks: false,
    extractText: 100,
    labelText: 100,
    subtleLabels: false,
    fade: 20,
    style: 'svg',
    mode: 'auto',
    markerEffect: 'none',
    markerColor: '',
    markerShape: 'arrow',
    squadFit: true,
  })
})

test('dropping a tab on the pinned bookmarks bookmarks and pins it once', () => {
  const s = defaults()
  s.bookmarks = []
  s.tabs.push({ id: 'w', kind: 'web', url: 'https://a.test/', title: 'A page' }, { id: 'b', kind: 'blank' })
  const first = bookmarkTab(s, 'w', null)
  assert.deepEqual([first.name, first.url, first.group, first.sidebar], ['A page', 'https://a.test/', 'other', true])
  assert.equal(bookmarkTab(s, 'w', null).id, first.id)
  assert.equal(s.bookmarks.length, 1)
  assert.equal(bookmarkTab(s, 'b', null), null)
  assert.equal(bookmarkTab(s, 'settings', null), null)
})

test('tarkov.dev and TarkovTracker are bookmarks pinned to the sidebar, added once', () => {
  const pinned = (b) => b.filter((x) => x.sidebar).map((x) => x.url)
  assert.deepEqual(pinned(defaults().bookmarks), ['https://tarkov.dev/', 'https://tarkovtracker.org/'])
  // Revision 2: the fixed views' bookmarks join, a bookmark already there is pinned.
  const old = restore({
    bookmarkRevision: 2,
    bookmarks: [{ id: 'mine', name: 'Tracker', url: 'https://tarkovtracker.org/', group: 'progress' }],
  })
  assert.deepEqual(pinned(old.bookmarks), ['https://tarkov.dev/', 'https://tarkovtracker.org/'])
  assert.equal(old.bookmarks.length, 2)
  // Unpinned since, it stays unpinned.
  const unpinned = restore({ ...old, bookmarks: old.bookmarks.map((b) => ({ ...b, sidebar: false })) })
  assert.deepEqual(pinned(unpinned.bookmarks), [])
  // The old .io site is replaced by the .org one the tracker API uses.
  const io = restore({
    bookmarkRevision: 1,
    bookmarks: [{ id: 'tracker', name: 'TarkovTracker', url: 'https://tarkovtracker.io/', group: 'progress' }],
  })
  assert.ok(io.bookmarks.some((b) => b.url === 'https://tarkovtracker.org/' && b.sidebar))
})
test('the item sidebar is restored with its item', () => {
  assert.deepEqual(restore({ itemPanel: { open: true, id: '57347ca924597744596b4e71', mode: 'pve' } }).itemPanel, {
    open: true,
    id: '57347ca924597744596b4e71',
    mode: 'pve',
  })
  assert.deepEqual(restore({ itemPanel: { open: 'yes', id: '../x', mode: 'pve' } }).itemPanel, {
    open: false,
    id: '',
    mode: '',
  })
  assert.deepEqual(restore({}).itemPanel, { open: false, id: '', mode: '' })
  assert.equal(restore({}).itemDock, 'right')
  assert.equal(restore({ itemDock: 'bottom' }).itemDock, 'bottom')
  assert.equal(restore({ itemDock: 'left' }).itemDock, 'left')
  assert.equal(restore({ itemDock: 'top' }).itemDock, 'right')
  assert.equal(restore({ itemPanelHeight: 9999 }).itemPanelHeight, 560)
  assert.equal(restore({}).itemPanelHeight, 280)
  assert.equal(restore({}).sidebarSide, 'left')
  assert.equal(restore({ sidebarSide: 'right' }).sidebarSide, 'right')
  const right = restore({ layout: 'horizontal', navPosition: 'right' })
  assert.equal(right.layout, 'vertical')
  assert.equal(right.sidebarSide, 'right')
  const top = restore({ sidebarSide: 'right', navPosition: 'top' })
  assert.equal(top.layout, 'horizontal')
  assert.equal(top.sidebarSide, 'right')
})
test('the address bar opens addresses and searches everything else', () => {
  assert.equal(resolveAddress(''), null)
  assert.equal(resolveAddress('   '), null)
  assert.equal(resolveAddress('https://tarkov.dev/maps/'), 'https://tarkov.dev/maps/')
  assert.equal(resolveAddress(' HTTP://tarkov.dev '), 'http://tarkov.dev/')
  assert.equal(resolveAddress('tarkov.dev/maps'), 'https://tarkov.dev/maps')
  assert.equal(
    resolveAddress('escapefromtarkov.fandom.com/wiki/Quests?x=1#top'),
    'https://escapefromtarkov.fandom.com/wiki/Quests?x=1#top',
  )
  assert.equal(resolveAddress('localhost:5173/page'), 'https://localhost:5173/page')
  assert.equal(resolveAddress('192.168.1.10'), 'https://192.168.1.10/')
  assert.equal(resolveAddress('devbox:8080'), 'https://devbox:8080/')
  assert.equal(resolveAddress('flea market prices'), searchURL('flea market prices'))
  assert.equal(resolveAddress('customs'), searchURL('customs'))
  assert.equal(resolveAddress('タルコフ 攻略'), searchURL('タルコフ 攻略'))
  assert.equal(resolveAddress('3.14'), searchURL('3.14'))
  assert.equal(resolveAddress('v0.1.13 patch notes'), searchURL('v0.1.13 patch notes'))
  assert.equal(resolveAddress('? tarkov.dev'), searchURL('tarkov.dev'))
  assert.equal(resolveAddress('?'), null)
  // Privileged and non-web schemes never navigate.
  assert.equal(resolveAddress('https://user:pw@tarkov.dev/'), searchURL('https://user:pw@tarkov.dev/'))
  assert.equal(resolveAddress('http://wails.localhost/'), searchURL('http://wails.localhost/'))
  assert.equal(resolveAddress('javascript:alert(1)'), searchURL('javascript:alert(1)'))
  assert.equal(resolveAddress('file:///C:/x'), searchURL('file:///C:/x'))
})
test('keyboard shortcuts follow Chrome', () => {
  const k = (key, mods = {}) => shortcut({ key, ctrl: false, shift: false, alt: false, ...mods })
  assert.deepEqual(k('t', { ctrl: true }), { type: 'newTab' })
  assert.deepEqual(k('T', { ctrl: true, shift: true }), { type: 'reopenTab' })
  assert.deepEqual(k('w', { ctrl: true }), { type: 'closeTab' })
  assert.deepEqual(k('F4', { ctrl: true }), { type: 'closeTab' })
  assert.deepEqual(k('Tab', { ctrl: true }), { type: 'nextTab' })
  assert.deepEqual(k('Tab', { ctrl: true, shift: true }), { type: 'prevTab' })
  assert.deepEqual(k('PageDown', { ctrl: true }), { type: 'nextTab' })
  assert.deepEqual(k('PageUp', { ctrl: true }), { type: 'prevTab' })
  assert.deepEqual(k('l', { ctrl: true }), { type: 'address' })
  assert.deepEqual(k('d', { alt: true }), { type: 'address' })
  assert.deepEqual(k('F6'), { type: 'address' })
  assert.deepEqual(k('d', { ctrl: true }), { type: 'bookmark' })
  assert.deepEqual(k('1', { ctrl: true }), { type: 'tabAt', index: 1 })
  assert.deepEqual(k('9', { ctrl: true }), { type: 'tabAt', index: 9 })
  assert.equal(k('t'), null)
  assert.equal(k('t', { ctrl: true, alt: true }), null)
  assert.equal(k('w', { ctrl: true, shift: true }), null)
  assert.equal(k('0', { ctrl: true }), null)
  assert.equal(k('l', { ctrl: true, shift: true }), null)
})
test('tab switching by number and cycling covers every tab', () => {
  const s = defaults()
  s.tabs = [
    { id: 'm', kind: 'livemap' },
    { id: 't', kind: 'blank' },
    { id: 'a', kind: 'web', url: 'https://a.example/' },
    { id: 'b', kind: 'web', url: 'https://b.example/' },
    { id: 'c', kind: 'blank' },
  ]
  s.active = 'a'
  assert.equal(tabAt(s, 1).id, 'm')
  assert.equal(tabAt(s, 2).id, 't')
  assert.equal(tabAt(s, 3).id, 'a')
  assert.equal(tabAt(s, 9).id, 'c')
  assert.equal(tabAt(s, 6), null)
  assert.equal(s.active, 'c')
  assert.equal(cycleTab(s, 1).id, 'm')
  assert.equal(cycleTab(s, -1).id, 'c')
  assert.equal(cycleTab(s, -1).id, 'b')
  s.tabs = []
  assert.equal(cycleTab(s, 1), null)
  assert.equal(tabAt(s, 1), null)
})
test("pages translate through Google Translate's proxy and back", () => {
  const wiki = 'https://escapefromtarkov.fandom.com/wiki/Quests?x=1#top'
  const translated = translatedURL(wiki, 'ja')
  assert.equal(
    translated,
    'https://escapefromtarkov-fandom-com.translate.goog/wiki/Quests?x=1&_x_tr_sl=auto&_x_tr_tl=ja&_x_tr_hl=ja#top',
  )
  assert.equal(isTranslated(translated), true)
  assert.equal(isTranslated(wiki), false)
  assert.equal(originalURL(translated), wiki)
  // Hyphens in the host are doubled and restored.
  assert.equal(
    translatedURL('https://tarkov-market.com/item/x', 'en'),
    'https://tarkov--market-com.translate.goog/item/x?_x_tr_sl=auto&_x_tr_tl=en&_x_tr_hl=en',
  )
  assert.equal(
    originalURL('https://tarkov--market-com.translate.goog/item/x?_x_tr_sl=auto&_x_tr_tl=en'),
    'https://tarkov-market.com/item/x',
  )
  // Already translated stays; the original stays; junk is null.
  assert.equal(translatedURL(translated, 'ja'), translated)
  assert.equal(originalURL(wiki), wiki)
  assert.equal(translatedURL('not a url', 'ja'), null)
})
test('a detected task opens the official wiki translated when asked', () => {
  const s = defaults()
  s.questSite = 'official-wiki'
  s.translateWiki = true
  const tab = receiveTask(s, task('Debut'))
  assert.equal(
    tab.url,
    'https://escapefromtarkov-fandom-com.translate.goog/wiki/Debut?_x_tr_sl=auto&_x_tr_tl=ja&_x_tr_hl=ja',
  )
  // The setting is for pages opened from now on: the tab already open for
  // the task stays as it is, translated.
  s.translateWiki = false
  assert.equal(receiveTask(s, task('Debut')).url, tab.url)
  assert.equal(receiveTask(s, task('Checking')).url, 'https://escapefromtarkov.fandom.com/wiki/Checking')
  s.translateWiki = true
  s.questSite = 'japanese-wiki'
  assert.equal(receiveTask(s, task('Debut')).url, 'https://wikiwiki.jp/eft/Prapor/Debut')
  assert.equal(restore({ translateWiki: true }).translateWiki, true)
  assert.equal(restore({}).translateWiki, false)
})

import { toolKeys, toolOrderOf, mergeToolOrder } from './state.js'
test('toolbar icons keep a whole order: snap left of the wiki search by default', () => {
  assert.deepEqual(restore({}).toolOrder, ['translate', 'snap', 'squadShare', 'wikiSearch', 'external'])
  assert.deepEqual(toolOrderOf(['external', 'translate']), [
    'external',
    'translate',
    'snap',
    'squadShare',
    'wikiSearch',
  ])
  assert.deepEqual(toolOrderOf(['bogus', 'snap', 'snap', 'external']), [
    'translate',
    'snap',
    'squadShare',
    'wikiSearch',
    'external',
  ])
  // An order saved before the squad share button takes it after the snap note's.
  assert.deepEqual(restore({ toolOrder: ['wikiSearch', 'snap', 'translate', 'external'] }).toolOrder, [
    'wikiSearch',
    'snap',
    'squadShare',
    'translate',
    'external',
  ])
  assert.deepEqual(toolKeys, ['translate', 'snap', 'squadShare', 'wikiSearch', 'external'])
})
test('dragging the shown icons keeps the hidden ones in place', () => {
  // No wiki search on a page without a task: the other three are reordered.
  assert.deepEqual(mergeToolOrder(['translate', 'snap', 'wikiSearch', 'external'], ['snap', 'external', 'translate']), [
    'snap',
    'external',
    'wikiSearch',
    'translate',
  ])
})

test('the task site of a page is its host, translated or moved on', () => {
  assert.equal(siteOfURL('https://wikiwiki.jp/eft/::cmd/search?word=Batya'), 'japanese-wiki')
  assert.equal(
    siteOfURL('https://wikiwiki.jp/eft/Mechanic/To%20the%20Light%20-%20Clip%20Their%20Wings'),
    'japanese-wiki',
  )
  assert.equal(siteOfURL(translatedURL('https://escapefromtarkov.fandom.com/wiki/Debut', 'ja')), 'official-wiki')
  assert.equal(siteOfURL('https://tarkov.dev/task/debut'), 'tarkov-dev')
  assert.equal(siteOfURL('https://example.com/'), null)
  assert.equal(siteOfURL(''), null)
})

test('recent squads keep the last five, newest first, for a month', () => {
  const day = 24 * 60 * 60 * 1000
  const now = 100 * day
  let list = []
  for (const code of ['AAAA-0001', 'AAAA-0002', 'AAAA-0003', 'AAAA-0004', 'AAAA-0005', 'AAAA-0006'])
    list = rememberSquad(list, code, now)
  assert.deepEqual(
    list.map((r) => r.code),
    ['AAAA-0006', 'AAAA-0005', 'AAAA-0004', 'AAAA-0003', 'AAAA-0002'],
  )
  list = rememberSquad(list, 'AAAA-0003', now + 1)
  assert.deepEqual(
    list.slice(0, 2).map((r) => r.code),
    ['AAAA-0003', 'AAAA-0006'],
  )
  assert.equal(new Set(list.map((r) => r.code)).size, list.length)
  assert.deepEqual(recentSquadsOf([{ code: 'AAAA-0001', at: now - 31 * day }], now), [])
  assert.deepEqual(recentSquadsOf([{ code: 'bad', at: now }, null, 'x'], now), [])
  assert.deepEqual(
    restore({ squadRecent: [{ code: 'ABCD-1234', at: Date.now() }] }).squadRecent.map((r) => r.code),
    ['ABCD-1234'],
  )
})

test('a Client takes all of its Host detections unless it turns some off', () => {
  assert.deepEqual(defaults().connection.receive, { task: true, map: true, item: true })
  const s = restore({ connection: { mode: 'client', receive: { task: false, map: true, extra: false } } })
  assert.deepEqual(s.connection.receive, { task: false, map: true, item: true })
  assert.equal(linkKind('browser:position'), 'map')
  assert.equal(linkKind('browser:item'), 'item')
})
