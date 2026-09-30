import {
  autoMap,
  findMap,
  floorFor,
  found,
  gameModes,
  iconURL,
  inBounds,
  labelLayer,
  markerFloor,
  onFloor,
  labelOff,
  floorOrder,
  markerGroups,
  markerColors,
  markerEffects,
  markerRotation,
  markerShapes,
  nameColors,
  outlineColor,
  placed,
  squadKey,
  searchTerms,
  shows,
  tarkovTime,
  textScale,
} from './map-geo.js'
import { state, esc, t, icon, action, clickHandlers, afterRenderHooks, render } from './shell-core.js'
import {
  attach,
  clearNotice,
  drawLines,
  pen,
  penBar,
  penButton,
  putPenDown,
  setPinning,
  squadPenButton,
  snapStrokes,
  squadSnapStrokes,
} from './map-draw.js'
import { mapName, ownName } from './view-squad.js'
import { colorOf } from './squad-colors.js'
import { myStyle as squadStyle } from './squad-draw.js'
import { canShare, dropPin, mapFresh, pinsOn, removePin } from './squad-share.js'

// The map view: tarkov.dev's interactive map redrawn by MAYAK (Leaflet over
// its SVG maps; internal/mapdata makes the picture of each floor and the
// markers from the catalog, the way tarkov.dev builds them), with the
// squad's positions on top. The map fills the page. Over it: the zoom
// buttons and under them a column of icon buttons (maps and floors,
// filters, search, snap note, your pen, squad, settings) at the top left, the
// pen's tools at the top while it is up (map-draw.js), their panels
// beside it, the raid's clocks and the map's credit at the top right, the
// pointer's coordinates at the bottom right. Leaflet loads when
// the view first shows, and the map lives in an element the render leaves
// alone (data-keep); after each render, drawMap brings it up to date.

// The map and floor chosen ('auto' follows the players), the open panel and
// the search; focus is a member to centre the map on once their map shows
// ({map, x, z}).
const view = { map: 'auto', floor: 'auto', panel: '', search: '', focus: null }

// word is a shell word, or '' when there is none for key.
const word = (key) => {
  const w = t(key)
  return w === key ? '' : w
}

// The map's entry, at the top of the sidebar.
export function liveMapEntry() {
  if (!state.squad) return ''
  const active = state.tabs.find((t) => t.id === state.active)?.kind === 'livemap'
  // A row like a tab (the whole of it opens the map, and shows it open), with
  // the name first and the map icon at its end as the sections have theirs.
  return `<div class="tab map-entry live-map-entry ${active ? 'active' : ''}"><button data-action="livemap" class="tab-select" title="${esc(t('liveMapHelp'))}" aria-pressed="${active}"><span class="tab-name">${esc(t('liveMap'))}</span>${mapFresh ? `<span class="map-fresh" title="${esc(t('mapFresh'))}"></span>` : ''}${icon('map', 'tab-icon live-map-icon')}</button></div>`
}

// shown is the map and floor to draw now.
function shown() {
  const sq = state.squad
  const maps = sq?.maps || []
  let members = sq?.state?.members || []
  // On the Host, its own last position shows without a squad too.
  const own = state.host?.raid && state.host.map ? state.host.position : null
  if (own && !placed(members).some((m) => m.me))
    members = [
      ...members.filter((m) => !m.me),
      { id: '', me: true, name: ownName() || t('squadYou'), map: state.host.map, pos: own, at: own.at },
    ]
  const key = view.map === 'auto' ? autoMap(members, state.host?.map || '') : view.map
  const map = findMap(maps, key) || (view.map === 'auto' ? maps.find(drawable) : null) || null
  const me = placed(members).find((m) => m.me)
  // Automatically: my floor while I am on this map, else the floor
  // tarkov.dev opens it on (Icebreaker's), else the ground.
  const floor =
    view.floor !== 'auto'
      ? view.floor
      : me && map && findMap(maps, me.map)?.key === map.key
        ? floorFor(map, me.pos)
        : map?.layers?.find((l) => l.show)?.id || ''
  return { map, floor, members }
}
const drawable = (m) => !!(m.svg || m.tilePath)
// styleOf is how a map is drawn: its SVG picture (tarkov.dev's Abstract) or
// its tiles (Satellite), as chosen when it has both.
const styleOf = (map) =>
  !map?.svg ? 'tile' : !map.tilePath ? 'svg' : state.mapSettings?.style === 'tile' ? 'tile' : 'svg'

function currentData() {
  const { map } = shown()
  return map ? state.squad?.markers?.[`${map.key}|${state.language}|${state.mapSettings?.mode || 'auto'}`] : null
}

// The layers of the filters: the data's, and the place names.
function layersOf(map, data) {
  const layers = [...(data?.layers || [])]
  if (map?.labels?.length) {
    const at = layers.findIndex((l) => l.group === 'landmarks' || markerGroups.indexOf(l.group) > 2)
    layers.splice(at < 0 ? layers.length : at, 0, labelLayer)
  }
  return layers
}
const layerLabel = (l) => word('layer_' + l.key) || l.name || l.key

// The base level's name: the ground, or the first floor of a map whose
// floors start at the second (The Lab, Factory, Interchange).
const baseName = (map) => word('floorBase_' + (map?.key || '')) || t('mapGround')
const floorName = (map, floor) => (floor ? map?.layers?.find((l) => l.id === floor)?.name || floor : baseName(map))

// The icon buttons in a column under the zoom buttons: map, filters,
// search and settings (the squad is in the sidebar and on its page), each opening its panel beside the column,
// and under search the one that makes the map a snap note and your pen.
function rail(map, floor) {
  const button = (id, iconName, label, extra = '') =>
    `<button class="map-rail-button ${view.panel === id ? 'selected' : ''}" data-action="mapPanel" data-id="${id}" aria-pressed="${view.panel === id}" title="${esc(label)}" aria-label="${esc(label)}">${icon(iconName)}${extra}</button>`
  const where = map ? `${mapName(map.key)}${map.layers?.length ? ' · ' + floorName(map, floor) : ''}` : ''
  // A floor other than the one the map opens on is named beside the button.
  const opening = map?.layers?.find((l) => l.show)?.id || ''
  const floorBadge = map && floor !== opening ? `<span class="map-rail-floor">${esc(floorName(map, floor))}</span>` : ''
  const snap = state.snapNotes
    ? `<button class="map-rail-button map-rail-snap" data-action="mapSnap" title="${esc(t('mapSnap'))}" aria-label="${esc(t('mapSnap'))}" ${!map || snapping ? 'disabled' : ''}>${icon('snap')}</button>`
    : ''
  return `<div class="map-rails"><div class="map-rail">${button('maps', 'map', `${t('mapPick')}${where ? `: ${where}` : ''}`, floorBadge)}${button('filters', 'list', t('mapFilters'))}${button('search', 'search', t('mapSearch'), view.search ? '<span class="map-rail-dot"></span>' : '')}${snap}${penButton(!map)}${button('settings', 'settings', t('mapSettings'))}</div>${squadRail(map)}</div>`
}

// The squad's column, under the other while in a squad, framed in this
// member's squad colour under the squad's mark: the squad pen and the pin.
function squadRail(map) {
  if (!state.squad?.state) return ''
  const off = !map || !canShare()
  const pin = `<button class="map-rail-button ${pinning ? 'selected' : ''}" data-action="mapPin" aria-pressed="${pinning}" title="${esc(t(pinning ? 'mapPinCancel' : 'mapPin'))}" aria-label="${esc(t('mapPin'))}" ${off && !pinning ? 'disabled' : ''}>${icon('mapPin')}</button>`
  const fitOn = !!state.mapSettings?.squadFit
  const fit = `<button class="map-rail-button ${fitOn ? 'selected' : ''}" data-action="mapSquadFit" aria-pressed="${fitOn}" title="${esc(t(fitOn ? 'mapSquadFitOff' : 'mapSquadFitOn'))}" aria-label="${esc(t('mapSquadFit'))}">${icon('squadFit')}</button>`
  return `<div class="map-rail map-rail-squad" style="--c:${esc(squadStyle().c)}" role="group" aria-label="${esc(t('squad'))}"><span class="map-rail-head" title="${esc(t('squad'))}">${icon('squad')}</span>${squadPenButton(!map)}${pin}${fit}</div>`
}

// The map panel: the maps, then the floors of the one shown.
function mapsPanel(map, floor) {
  const maps = (state.squad.maps || []).filter(drawable)
  const choice = (action, value, label, on) =>
    `<button class="map-choice ${on ? 'selected' : ''}" data-action="${action}" data-id="${esc(value)}" aria-pressed="${on}">${esc(label)}</button>`
  const mapList = [
    choice(
      'mapPick',
      'auto',
      `${t('mapAuto')}${view.map === 'auto' && map ? ` (${mapName(map.key)})` : ''}`,
      view.map === 'auto',
    ),
    ...maps.map((m) => choice('mapPick', m.key, mapName(m.key), view.map === m.key)),
  ].join('')
  const floors = map?.layers?.length
    ? `<h3>${esc(t('mapFloor'))}</h3><div class="map-choices">${[
        choice(
          'floorPick',
          'auto',
          `${t('mapAuto')}${view.floor === 'auto' ? ` (${floorName(map, floor)})` : ''}`,
          view.floor === 'auto',
        ),
        choice('floorPick', '', baseName(map), view.floor === ''),
        ...map.layers.map((l) => choice('floorPick', l.id, l.name, view.floor === l.id)),
      ].join('')}</div><p class="hint">${esc(t('mapFloorHelp'))}</p>`
    : ''
  // tarkov.dev's Abstract (the SVG) and Satellite (the tiles), for a map with both.
  const style = styleOf(map)
  const styles =
    map?.svg && map.tilePath
      ? `<h3>${esc(t('mapStyle'))}</h3><div class="map-choices map-choices-row">${choice('mapStyle', 'svg', t('mapStyleSvg'), style === 'svg')}${choice('mapStyle', 'tile', t('mapStyleTile'), style === 'tile')}</div>`
      : ''
  // The game mode of the markers' data (boss chances differ); automatically
  // the one played, named beside it.
  const mode = state.mapSettings?.mode || 'auto'
  const played = currentData()?.mode
  const modes = `<h3>${esc(t('mapMode'))}</h3><div class="map-choices map-choices-row">${gameModes
    .map((m) =>
      choice(
        'mapMode',
        m,
        m === 'auto' && played
          ? `${t('mapAuto')} (${t('gameMode_' + played)})`
          : t(m === 'auto' ? 'mapAuto' : 'gameMode_' + m),
        mode === m,
      ),
    )
    .join('')}</div>`
  return `<aside class="map-panel map-maps" aria-label="${esc(t('mapPick'))}">${panelHead(t('mapPick'))}${modes}<h3>${esc(t('liveMap'))}</h3><div class="map-choices">${mapList}</div>${floors}${styles}</aside>`
}

// The search panel: tarkov.dev's search, by commas.
function searchPanel() {
  return `<aside class="map-panel map-search-panel" aria-label="${esc(t('mapSearch'))}">${panelHead(t('mapSearch'))}<div class="map-search">${icon('search', 'map-search-icon')}<input id="map-search" type="search" autocomplete="off" spellcheck="false" placeholder="${esc(t('mapSearchPlaceholder'))}" aria-label="${esc(t('mapSearch'))}" value="${esc(view.search)}"></div><p class="hint">${esc(t('mapSearchHint'))}</p></aside>`
}

const panelHead = (title) =>
  `<header><h2>${esc(title)}</h2><button data-action="mapPanel" data-id="" title="${esc(t('close'))}" aria-label="${esc(t('close'))}">${icon('x')}</button></header>`

function filtersPanel(map, data) {
  const hidden = state.mapHidden || []
  const folded = state.mapCollapsed || []
  const layers = layersOf(map, data)
  const counts = {}
  for (const m of data?.markers || [])
    for (const l of m.layers?.length ? m.layers : [m.layer]) counts[l] = (counts[l] || 0) + 1
  if (map?.labels?.length) counts.labels = map.labels.length
  const groups = markerGroups
    .map((g) => {
      const list = layers.filter((l) => l.group === g)
      if (!list.length) return ''
      const on = list.filter((l) => !hidden.includes(l.key)).length
      const open = !folded.includes(g)
      return `<fieldset class="map-group"><legend><input type="checkbox" data-map-group="${g}" ${on === list.length ? 'checked' : ''} data-indeterminate="${on > 0 && on < list.length}" aria-label="${esc(t('group_' + g))}"><button class="map-group-fold" data-action="mapFold" data-id="${g}" aria-expanded="${open}">${esc(t('group_' + g))}${icon('chevron', 'map-group-chevron')}</button></legend>${
        open
          ? list
              .map(
                (l) =>
                  `<label class="map-filter"><input type="checkbox" data-map-layer="${esc(l.key)}" ${hidden.includes(l.key) ? '' : 'checked'}>${l.icon || l.iconUrl ? `<img class="map-filter-icon" src="${esc(l.iconUrl || iconURL(l.icon))}" alt="" referrerpolicy="no-referrer">` : `<span class="map-filter-icon map-filter-text">A</span>`}<span>${esc(layerLabel(l))}</span>${counts[l.key] ? `<small>${counts[l.key]}</small>` : ''}</label>`,
              )
              .join('')
          : ''
      }</fieldset>`
    })
    .join('')
  return `<aside class="map-panel map-filters" aria-label="${esc(t('mapFilters'))}">${panelHead(t('mapFilters'))}${data ? groups : `<p class="hint">${esc(t('mapLoading'))}</p>`}<p class="hint">${esc(t('mapDataCredit'))}</p></aside>`
}

function settingsPanel() {
  const s = state.mapSettings || {}
  const row = (key, label, hint = '') =>
    `<label class="map-filter map-setting"><input type="checkbox" data-map-setting="${key}" ${s[key] ? 'checked' : ''}><span>${esc(label)}${hint ? `<small class="map-setting-hint">${esc(hint)}</small>` : ''}</span></label>`
  const slider = (key, label, min = textScale.min, max = textScale.max) =>
    `<label class="map-slider"><span>${esc(label)}<output data-map-scale-out="${key}">${s[key]}%</output></span><input type="range" min="${min}" max="${max}" step="5" value="${s[key]}" data-map-scale="${key}"></label>`
  // The arrows: their shape, their effect, then (with one) its colour.
  const shape = (id) =>
    `<button class="map-marker-shape ${s.markerShape === id ? 'selected' : ''}" data-action="mapMarkerShape" data-id="${id}" aria-pressed="${s.markerShape === id}" title="${esc(t('settingShape_' + id))}" aria-label="${esc(t('settingShape_' + id))}"><svg viewBox="0 0 24 24" style="transform:rotate(35deg)">${markerShapes[id]}</svg></button>`
  const effect = (id) =>
    `<button data-action="mapMarkerEffect" data-id="${id}" class="${s.markerEffect === id ? 'selected' : ''}" aria-pressed="${s.markerEffect === id}">${esc(t('settingMarker_' + id))}</button>`
  const color = (c) =>
    `<button class="map-marker-color ${s.markerColor === c ? 'selected' : ''} ${c ? '' : 'auto'}" data-action="mapMarkerColor" data-id="${c}" style="--swatch:${c || 'transparent'}" title="${esc(c ? c : t('settingMarkerAuto'))}" aria-pressed="${s.markerColor === c}"></button>`
  const marker = `<div class="map-marker-setting"><span class="map-marker-title">${esc(t('settingShape'))}</span><div class="map-marker-shapes" role="group" aria-label="${esc(t('settingShape'))}">${Object.keys(markerShapes).map(shape).join('')}</div><span class="map-marker-title">${esc(t('settingMarker'))}</span><div class="segmented map-marker-effects" role="group">${markerEffects.map(effect).join('')}</div>${s.markerEffect !== 'none' ? `<div class="map-marker-colors" role="group" aria-label="${esc(t('settingMarkerColor'))}">${markerColors.map(color).join('')}</div>` : ''}</div>`
  return `<aside class="map-panel map-settings" aria-label="${esc(t('mapSettings'))}">${panelHead(t('mapSettings'))}${row('snipers', t('settingSnipers'), t('settingSnipersHint'))}${row('extracts', t('settingExtracts'), t('settingExtractsHint'))}${row('activeTasks', t('settingActiveTasks'), t('settingActiveTasksHint'))}${row('subtleLabels', t('settingSubtleLabels'))}${slider('fade', t('settingFade'), 0, 60)}${slider('extractText', t('settingExtractText'))}${slider('labelText', t('settingLabelText'))}${marker}</aside>`
}

// applyTextScale sizes the extracts' and places' names (CSS variables on
// the map, so the markers are not made again).
function applyTextScale(settings = state.mapSettings || {}) {
  const el = document.getElementById('live-map')
  if (!el) return
  el.style.setProperty('--extract-text', String((settings.extractText || 100) / 100))
  el.style.setProperty('--label-text', String((settings.labelText || 100) / 100))
  el.classList.toggle('subtle-labels', !!settings.subtleLabels)
}

// The raid's facts at the top right: the two clocks (as tarkov.dev keeps
// them), the raid's length and players, and who drew the map (its credit,
// which the map's licence asks for; the name opens their page). The clocks
// tick on their own.
function raidInfo(map, data) {
  if (!map) return ''
  const facts = [
    data?.raidDuration ? `${t('raidDuration')} ${data.raidDuration} ${t('minutes')}` : '',
    data?.players ? `${t('raidPlayers')} ${data.players}` : '',
  ].filter(Boolean)
  const author = map.author || 'Shebuka'
  const link = map.authorLink || 'https://github.com/the-hideout/tarkov-dev-svg-maps'
  const credit = `<div class="map-credit">${esc(t('mapBy'))} <button data-action="open" data-id="${esc(link)}" title="${esc(link)}">${esc(author)}</button><br>CC BY-NC-SA 4.0 · tarkov.dev</div>`
  return `<div class="map-raid" data-keep="map-raid-${esc(map.key)}|${esc(facts.join())}|${esc(author)}">${map.key === 'the-lab' ? '' : `<div class="map-clocks"><span data-clock="left"></span><span data-clock="right"></span></div>`}${facts.length ? `<div class="map-facts">${facts.map(esc).join(' · ')}</div>` : ''}${credit}</div>`
}
function tickClocks() {
  const key = shown().map?.key
  for (const el of document.querySelectorAll('.map-raid [data-clock]')) {
    const left = /** @type {HTMLElement} */ (el).dataset.clock === 'left'
    el.textContent =
      key === 'factory' || key === 'night-factory' ? (left ? '15:28:00' : '03:28:00') : tarkovTime(Date.now(), left)
  }
}

export function liveMapPage() {
  const sq = state.squad
  if (!sq) return `<div class="page"><p class="empty-tabs">${esc(t('squadUnavailable'))}</p></div>`
  const { map, floor } = shown()
  const data = currentData()
  const note = !sq.maps ? t(sq.mapsError ? 'mapDataFailed' : 'mapLoading') : !map ? t('mapNone') : ''
  const panel =
    view.panel === 'maps'
      ? mapsPanel(map, floor)
      : view.panel === 'filters'
        ? filtersPanel(map, data)
        : view.panel === 'settings'
          ? settingsPanel()
          : view.panel === 'search'
            ? searchPanel()
            : ''
  return `<div class="live-map-page"><div id="live-map" data-keep="livemap"></div>${note ? `<p class="map-note">${esc(note)}</p>` : ''}${rail(map, floor)}${penBar()}${clearNotice()}${panel ? `<div class="map-panel-host">${panel}</div>` : ''}${raidInfo(map, data)}<div class="map-coords" data-keep="map-coords"></div></div>`
}

// Leaflet and the map drawn with it.
let L = null
let loading = null
const fresh = () => ({
  map: null,
  el: null,
  key: '',
  // The picture under the markers (drawBase): the SVG image, the base tiles
  // and a floor's tiles, drawn for baseKey.
  overlay: null,
  tiles: null,
  floorTiles: null,
  baseKey: '',
  squad: new Map(),
  pins: new Map(),
  things: null,
  thingsKey: '',
  resize: null,
  tick: 0,
  clock: 0,
  // Ends the listeners build put on the map's element (it outlives the map).
  listeners: null,
  // When Ctrl + wheel last stepped a floor.
  stepped: 0,
})
let lm = fresh()
const requested = new Set()

function teardown() {
  lm.resize?.disconnect()
  lm.listeners?.abort()
  clearInterval(lm.tick)
  clearInterval(lm.clock)
  lm.map?.remove()
  lm = fresh()
}

// The CRS of tarkov.dev's maps (its getCRS): game (x, z) rotated by the
// map's coordinateRotation, then scaled and moved by its transform.
function crs(map) {
  const [sx, mx, sz, mz] = map.transform
  const rotate = (latLng, deg) => {
    if (!deg) return latLng
    const a = (deg * Math.PI) / 180
    const x = latLng.lng,
      y = latLng.lat
    return L.latLng(x * Math.sin(a) + y * Math.cos(a), x * Math.cos(a) - y * Math.sin(a))
  }
  return L.extend({}, L.CRS.Simple, {
    transformation: new L.Transformation(sx, mx, -sz, mz),
    projection: L.extend({}, L.Projection.LonLat, {
      project: (latLng) => L.Projection.LonLat.project(rotate(latLng, map.rotation)),
      unproject: (point) => rotate(L.Projection.LonLat.unproject(point), -map.rotation),
    }),
  })
}
const toBounds = (b) => L.latLngBounds([b[0][1], b[0][0]], [b[1][1], b[1][0]])

// setFloor shows a floor ('' for the ground) as chosen.
function setFloor(floor) {
  if (view.floor === floor) return
  view.floor = floor
  render()
}

function build(el, map) {
  teardown()
  lm.el = el
  lm.key = map.key
  const bounds = toBounds(map.bounds)
  lm.map = L.map(el, {
    crs: crs(map),
    zoomSnap: 0.1,
    wheelPxPerZoomLevel: 120,
    minZoom: Math.max(0, (map.minZoom || 2) - 1),
    maxZoom: Math.max(7, map.maxZoom || 6),
    maxBounds: bounds.pad(0.25),
    // The map's credit is in the raid box (raidInfo), as on tarkov.dev.
    attributionControl: false,
    zoomControl: false,
  })
  L.control.zoom({ position: 'topleft' }).addTo(lm.map)
  // A floor's tiles go above the map's picture (Leaflet puts tiles under
  // images), or the faded ground picture would dim the floor too.
  const floorPane = lm.map.createPane('mapFloor')
  floorPane.style.zIndex = '450'
  floorPane.style.pointerEvents = 'none'
  // Markers on another floor go under the map's picture: a floor drawn
  // over them hides them, as the floor above hides what is below it; the
  // picture lets pointers through, so they can still be clicked.
  lm.map.createPane('offLevel').style.zIndex = '350'
  // The squad's pins stand over the pens' lines.
  lm.map.createPane('mapPins').style.zIndex = '660'
  // With a pin being put, a press on the map puts it there.
  lm.map.on('click', (e) => {
    if (!pinning) return
    const { map, floor } = shown()
    if (!map) return
    // Pressed again before the mode ends, the same pin moves there.
    void dropPin({ map: map.key, floor, x: e.latlng.lng, z: e.latlng.lat, zoom: lm.map.getZoom() }, { move: pinPut })
    pinPut = true
  })
  lm.map.fitBounds(bounds)
  lm.resize = new ResizeObserver(() => lm.map?.invalidateSize())
  lm.resize.observe(el)
  // The element stays for the next map: its listeners go with this one.
  lm.listeners = new AbortController()
  const signal = lm.listeners.signal
  // A press on the map closes the open panel and lets go of a picker.
  el.addEventListener(
    'pointerdown',
    () => {
      const focused = /** @type {HTMLElement} */ (document.activeElement)
      if (focused?.closest?.('.map-rail, .map-panel')) focused.blur()
      if (view.panel) {
        view.panel = ''
        render()
      }
    },
    { signal },
  )
  // The pointer's place in game coordinates, as tarkov.dev shows it.
  lm.map.on('mousemove', (e) => {
    const out = document.querySelector('.map-coords')
    if (out) out.textContent = `x: ${e.latlng.lng.toFixed(2)}  z: ${e.latlng.lat.toFixed(2)}`
  })
  // Ctrl + wheel steps through the floors instead of zooming (tarkov.dev):
  // up and down by height, stopping at the top and the bottom.
  // One turn of the wheel can come as several events (a fine wheel, a
  // touchpad): those close together make one step.
  el.addEventListener(
    'wheel',
    (e) => {
      if (!e.ctrlKey) return
      e.preventDefault()
      e.stopPropagation()
      if (!e.deltaY || e.timeStamp - lm.stepped < 150) {
        if (e.deltaY) lm.stepped = e.timeStamp
        return
      }
      lm.stepped = e.timeStamp
      const { map: m, floor } = shown()
      const floors = floorOrder(m)
      const at = floors.indexOf(floor)
      const next = floors[Math.min(floors.length - 1, Math.max(0, at + (e.deltaY < 0 ? 1 : -1)))]
      if (at >= 0 && next !== undefined) setFloor(next)
    },
    { capture: true, passive: false, signal },
  )
  attach(el, signal)
  // Positions age while nothing else changes: redrawn each half minute.
  lm.tick = setInterval(render, 30000)
  lm.clock = setInterval(tickClocks, 1000)
}

// popupHTML is what a marker tells when clicked, as tarkov.dev's popups.
function popupHTML(m, layers) {
  const d = m.detail || {}
  const title = m.name || layerLabel(layers.find((l) => l.key === m.layer) || { key: m.layer })
  const parts = [`<strong>${esc(title)}</strong>`]
  // The task opens in a tab as a recognized one does (the task site and the
  // tab setting).
  if (d.task)
    parts.push(
      `<div class="map-popup-task">${esc(t('popupTask'))}: ${d.taskId ? `<button class="map-popup-link" data-action="mapTask" data-id="${esc(d.taskId)}" data-name="${esc(d.task)}" title="${esc(t('openTask'))}">${esc(d.task)}</button>` : esc(d.task)}</div>`,
    )
  if (d.bosses?.length)
    parts.push(`<div>${d.bosses.map((b) => `${esc(b.name)} (${Math.round(b.chance * 100)}%)`).join(', ')}</div>`)
  if (d.lockType)
    parts.push(
      `<div>${esc(word('lockType_' + d.lockType) || d.lockType)}${d.needsPower ? ` · <em>${esc(t('lockPower'))}</em>` : ''}</div>`,
    )
  if (d.activatedBy?.length)
    parts.push(`<div>${esc(t('popupActivatedBy'))}: ${d.activatedBy.map(esc).join(', ')}</div>`)
  if (d.activates?.length)
    parts.push(
      `<div>${esc(t('popupActivates'))}: ${d.activates.map((a) => `<span style="color:${nameColors['extract_' + (a.faction || '')] || 'inherit'}">${esc(a.name)}</span>`).join(', ')}</div>`,
    )
  if (d.item && m.layer !== 'quest_item')
    parts.push(
      `<div class="map-popup-item">${d.item.image ? `<img src="${esc(d.item.image)}" alt="" referrerpolicy="no-referrer">` : ''}<span>${m.layer.startsWith('extract') ? `${esc(t('popupRequiredItem'))}: ` : ''}${esc(d.item.name)}${d.item.count > 1 ? ` ×${d.item.count}` : ''}</span></div>`,
    )
  if (m.layer === 'quest_item' && d.item?.image)
    parts.push(`<div class="map-popup-item"><img src="${esc(d.item.image)}" alt="" referrerpolicy="no-referrer"></div>`)
  if (d.items?.length)
    parts.push(
      `<div class="map-popup-items">${d.items.map((i) => `<span title="${esc(i.name)}">${i.image ? `<img src="${esc(i.image)}" alt="" referrerpolicy="no-referrer">` : ''}${d.items.length === 1 ? esc(i.name) : ''}</span>`).join('')}</div>`,
    )
  return `<div class="map-popup">${parts.join('')}</div>`
}

// drawThings draws the markers the filters let through, again when the map,
// the floor, the filters, the settings, the search or the markers change.
function drawThings(map, floor, data) {
  const hidden = state.mapHidden || []
  const settings = state.mapSettings || {}
  const terms = searchTerms(view.search)
  const key = [
    map.key,
    floor,
    hidden.join(','),
    [settings.snipers, settings.extracts, settings.activeTasks, settings.fade].join(),
    terms.join(','),
    data?.markers?.length || 0,
    state.language,
  ].join('|')
  if (lm.thingsKey === key) return
  lm.thingsKey = key
  lm.things?.remove()
  lm.things = L.layerGroup().addTo(lm.map)
  const layers = layersOf(map, data)
  // Place names, faded off the floor shown as the markers are.
  if (!hidden.includes('labels') && !terms.length)
    for (const l of map.labels || []) {
      L.marker([l.z, l.x], {
        interactive: false,
        keyboard: false,
        zIndexOffset: -100000,
        opacity: labelOff(map, l, floor) ? offAlpha() : 1,
        icon: L.divIcon({
          className: 'map-label-icon',
          html: `<span class="map-label" style="font-size:calc(${(20 * (l.size || 100)) / 100}px * var(--label-text, 1));transform:translate(-50%,-50%) rotate(${l.rotation || 0}deg)">${esc(l.text)}</span>`,
          iconSize: [0, 0],
        }),
      }).addTo(lm.things)
    }
  for (const m of data?.markers || []) {
    if (!shows(m, hidden)) continue
    if (settings.activeTasks && m.detail && m.detail.taskId && !m.detail.active) continue
    if ((m.layer.startsWith('spawn') || m.layer.startsWith('loose')) && !inBounds(map, m.x, m.z)) continue
    const hit = terms.length > 0 && found(m, terms)
    if (terms.length && !hit) continue
    // Off the floor shown, a marker fades behind the others (tarkov.dev),
    // snipers and extracts unless kept by the settings.
    const floorOf = markerFloor(map, m)
    const always =
      (settings.snipers && m.layer === 'spawn_sniper_scav') || (settings.extracts && m.layer.startsWith('extract'))
    const off = !always && !onFloor(map, m, floor)
    const named = m.layer.startsWith('extract')
    const size = m.iconSize?.[0] ? m.iconSize : [24, 24]
    // A single loose item shows its own picture, outlined (tarkov.dev).
    const single = m.layer.startsWith('loose') && m.detail?.items?.length === 1
    const cls = `${hit ? 'map-found' : ''} ${single ? 'map-loot-outline' : ''}`
    const icon = named
      ? L.divIcon({
          className: `map-thing-icon ${cls}`,
          html: `<span class="map-extract"><img src="${iconURL(m.icon)}" alt=""><span class="map-extract-name" style="--c:${nameColors[m.layer] || '#fff'}">${esc(m.name || '')}</span></span>`,
          iconSize: [24, 24],
          iconAnchor: [12, 12],
        })
      : L.icon({
          iconUrl: m.iconUrl || iconURL(m.icon),
          iconSize: size,
          iconAnchor: [size[0] / 2, size[1] / 2],
          popupAnchor: [0, -size[1] / 2],
          className: `map-thing-img ${cls}`,
        })
    const zIndexOffset = off
      ? -9999
      : m.layer === 'extract_pmc'
        ? 150
        : m.layer === 'extract_shared'
          ? 125
          : named
            ? 100
            : m.layer.startsWith('hazard')
              ? -100
              : 0
    const marker = L.marker([m.z, m.x], {
      icon,
      keyboard: false,
      riseOnHover: true,
      zIndexOffset,
      opacity: off ? offAlpha() : 1,
      title: named ? '' : m.name || '',
      pane: off ? 'offLevel' : 'markerPane',
    }).addTo(lm.things)
    marker.bindPopup(popupHTML(m, layers), { className: 'map-popup-wrap', maxWidth: 320 })
    // Areas show while pointed at, and stay when clicked (tarkov.dev).
    let outline = null
    let pinned = false
    if (m.outline?.length > 2) {
      outline = L.polygon(
        m.outline.map(([x, z]) => [z, x]),
        {
          color: outlineColor(m.layer),
          weight: 1,
          opacity: off ? offAlpha() : 1,
          fillOpacity: off ? offAlpha() / 10 : 0.2,
          interactive: false,
        },
      )
      marker.on('mouseover', () => outline.addTo(lm.things))
      marker.on('mouseout', () => !pinned && outline.remove())
    }
    marker.on('click', () => {
      if (outline) {
        pinned = !pinned
        if (pinned) outline.addTo(lm.things)
        else outline.remove()
      }
      // A marker on another floor brings its floor up.
      if (floorOf !== floor && !onFloor(map, m, floor)) setFloor(floorOf)
    })
  }
}

// The arrows (yours and the squad's) wear the effect chosen in the map's
// settings (an outline, a glow, a pulsing ring, a beacon), in the colour
// chosen, else each in its member's squad colour.
function effectOf(m) {
  const s = state.mapSettings || {}
  if (!s.markerEffect || s.markerEffect === 'none') return { cls: '', style: '' }
  return { cls: ` fx-${s.markerEffect}`, style: `;--fx:${s.markerColor || colorOf(m)}` }
}
function squadHTML(m) {
  const rot = markerRotation(m.pos.rot, shown().map?.rotation)
  const fx = effectOf(m)
  return `<span class="squad-marker ${m.me ? 'me' : ''}${fx.cls}" style="--c:${colorOf(m)}${fx.style}"><svg viewBox="0 0 24 24" style="transform:rotate(${rot}deg)">${markerShapes[state.mapSettings?.markerShape] || markerShapes.arrow}</svg><span class="squad-label">${esc(m.name || '?')}</span></span>`
}

function drawSquad(map, floor, members) {
  const seen = new Set()
  for (const m of placed(members)) {
    if (findMap(state.squad.maps, m.map)?.key !== map.key) continue
    const id = m.me ? 'me' : m.id
    seen.add(id)
    const html = squadHTML(m)
    const at = L.latLng(m.pos.z, m.pos.x)
    let marker = lm.squad.get(id)
    if (!marker) {
      marker = L.marker(at, { interactive: false, keyboard: false, zIndexOffset: m.me ? 2000 : 1000 })
      marker._html = ''
      marker.addTo(lm.map)
      lm.squad.set(id, marker)
    } else marker.setLatLng(at)
    if (marker._html !== html) {
      marker._html = html
      marker.setIcon(L.divIcon({ className: 'squad-marker-icon', html, iconSize: [28, 28], iconAnchor: [14, 14] }))
    }
  }
  for (const [id, marker] of lm.squad)
    if (!seen.has(id)) {
      marker.remove()
      lm.squad.delete(id)
    }
}

// drawPins puts the squad's pins on the map: a pin in its member's colour
// with their name, faint when on another floor. A pin just put drops in.
// Yours can be dragged (not while a pen is up) and taken out with the ×
// beside your name.
function drawPins(map, floor) {
  const seen = new Set()
  for (const p of pinsOn(map.key)) {
    seen.add(p.by)
    const name = p.mine ? t('squadYou') : p.name || '?'
    let marker = lm.pins.get(p.by)
    if (marker && (marker._id !== p.id || marker._drop !== p.dropAt || marker._name !== name)) {
      marker.remove()
      marker = null
    }
    if (!marker) {
      const drop = Date.now() - (p.dropAt || 0) < 1500
      const remove = p.mine
        ? `<button class="map-pin-remove" title="${esc(t('mapPinRemove'))}" aria-label="${esc(t('mapPinRemove'))}">${icon('x')}</button>`
        : ''
      const html = `<span class="map-pin ${p.mine ? 'mine' : ''} ${drop ? 'drop' : ''}" style="--c:${esc(p.c)}"><svg viewBox="0 0 26 34"><path class="map-pin-body" d="M13 33C13 33 2 21.6 2 12.6a11 11 0 0 1 22 0C24 21.6 13 33 13 33z"/><circle class="map-pin-dot" cx="13" cy="12.6" r="4.2"/></svg><span class="map-pin-name">${esc(name)}${remove}</span></span>`
      marker = L.marker(L.latLng(p.z, p.x), {
        pane: 'mapPins',
        interactive: p.mine,
        draggable: p.mine,
        keyboard: false,
        title: p.mine ? t('mapPinDrag') : '',
        icon: L.divIcon({ className: 'map-pin-icon', html, iconSize: [26, 34], iconAnchor: [13, 33] }),
      })
      if (p.mine) {
        marker.on('click', (e) => {
          if (e.originalEvent?.target?.closest?.('.map-pin-remove')) removePin()
        })
        marker.on('dragstart', () => (marker._dragging = true))
        marker.on('dragend', () => {
          marker._dragging = false
          const ll = marker.getLatLng()
          void dropPin(
            { map: p.map, floor: p.floor, x: ll.lng, z: ll.lat, zoom: lm.map.getZoom() },
            { move: true, drag: true },
          )
        })
      }
      marker._id = p.id
      marker._drop = p.dropAt
      marker._name = name
      marker.addTo(lm.map)
      lm.pins.set(p.by, marker)
    } else if (!marker._dragging) marker.setLatLng(L.latLng(p.z, p.x))
    marker.setOpacity((p.floor || '') === (floor || '') ? 1 : 0.45)
    if (p.mine && marker.dragging) pen.on ? marker.dragging.disable() : marker.dragging.enable()
  }
  for (const [by, marker] of lm.pins)
    if (!seen.has(by)) {
      marker.remove()
      lm.pins.delete(by)
    }
}

// Following the squad (the squad's column, mapSquadFit): when a member's
// position comes anew (a later time than seen) on the map of your raid, and
// that map shows, the map takes in everyone there (you too). Not while a pen
// is up or a pin is being put, nor with no one else there (playing alone). seenAt: each member's position time last seen (the first sight of a
// member counts as new, once the map has been drawn).
const seenAt = new Map()
let fitPrimed = false
let fitNow = false
function followSquad(map, members) {
  const on = !!state.mapSettings?.squadFit
  let fresh = false
  for (const m of placed(members)) {
    const id = m.key || squadKey(m)
    const at = String(m.at || '')
    if (seenAt.get(id) !== at) {
      seenAt.set(id, at)
      if (findMap(state.squad?.maps, m.map)?.key === map.key) fresh = true
    }
  }
  const primed = fitPrimed
  fitPrimed = true
  if (!on || pen.on || pinning || !(fitNow || (primed && fresh))) return
  fitNow = false
  const mine = raidMap()
  if (!mine || mine !== map.key) return
  const here = placed(members).filter((m) => findMap(state.squad?.maps, m.map)?.key === map.key)
  if (here.length < 2) return
  lm.map.fitBounds(L.latLngBounds(here.map((m) => L.latLng(m.pos.z, m.pos.x))), {
    padding: [90, 90],
    maxZoom: Math.max(lm.map.getMinZoom() + 2, (map.maxZoom || 6) - 1),
  })
}
// raidMap is the map of your raid: the Host's own while in one, else where
// your last position was.
function raidMap() {
  const maps = state.squad?.maps
  if (state.host?.raid && state.host.map) return findMap(maps, state.host.map)?.key || ''
  const me = placed(state.squad?.state?.members || []).find((m) => m.me)
  return me ? findMap(maps, me.map)?.key || '' : ''
}

// offAlpha is how strong what is on another floor shows (the setting).
const offAlpha = () => (state.mapSettings?.fade ?? 20) / 100

// drawBase draws the map itself as tarkov.dev does, for the style and the
// floor shown:
//   SVG: the picture with the floor's group over the faded ground (Go makes
//        that picture); a floor only in tiles is its tiles over the faded
//        ground picture.
//   tiles: the map's tiles, faded under a floor's tiles; a floor only in
//        the SVG draws nothing over them (the markers show it), as on
//        tarkov.dev's Satellite.
// A floor tarkov.dev marks "show" (Icebreaker's decks) leaves the ground as
// it is.
function drawBase(map, floor) {
  const style = styleOf(map)
  const layer = map.layers?.find((l) => l.id === floor) || null
  const fade = !!layer && !layer.show
  let svg = null // the floor's SVG group ('' for the ground), or null for none
  let baseTiles = false
  let floorTiles = ''
  if (style === 'svg') {
    svg = layer?.svgLayer || ''
    if (layer && !layer.svgLayer) floorTiles = layer.tilePath
  } else {
    baseTiles = true
    if (layer?.tilePath) floorTiles = layer.tilePath
  }
  const svgFaded = style === 'svg' && fade && !!layer && !layer.svgLayer
  const pct = state.mapSettings?.fade ?? 20
  // How strong the SVG's ground shows under its floor: faded, or as it is
  // under one that leaves the ground (show).
  const ground = !svg ? 0 : fade ? pct : 100
  const imageKey = svg === null ? '' : `${map.key}|${svg}|${ground}`
  const url = imageKey ? state.squad.images?.[imageKey] : ''
  if (imageKey && !url) {
    fetchOnce('image:' + imageKey, 'squadMapImage', { map: map.key, layer: svg, fade: ground })
    if (style === 'svg') return
  }
  const key = [style, imageKey && url ? imageKey : '', svgFaded, baseTiles, fade, floorTiles, pct].join('|')
  if (lm.baseKey === key) return
  lm.baseKey = key
  for (const k of ['overlay', 'tiles', 'floorTiles']) {
    lm[k]?.remove()
    lm[k] = null
  }
  const bounds = toBounds(map.bounds)
  const tiles = (path, pane) =>
    L.tileLayer(path, {
      tileSize: map.tileSize || 256,
      bounds,
      maxNativeZoom: map.maxZoom || 6,
      maxZoom: lm.map.getMaxZoom(),
      pane,
    }).addTo(lm.map)
  if (baseTiles) {
    lm.tiles = tiles(map.tilePath, 'tilePane')
    if (fade) lm.tiles.setOpacity(offAlpha())
  }
  if (imageKey && url) {
    lm.overlay = L.imageOverlay(url, toBounds(map.svgBounds || map.bounds)).addTo(lm.map)
    if (svgFaded) lm.overlay.setOpacity(offAlpha())
    if (style === 'svg') lm.overlay.bringToBack()
  }
  if (floorTiles) lm.floorTiles = tiles(floorTiles, 'mapFloor')
}

// fetchOnce asks for something the map needs once at a time.
function fetchOnce(key, type, data) {
  if (requested.has(key)) return
  requested.add(key)
  void action(type, data).finally(() => requested.delete(key))
}

function drawMap() {
  // A group checkbox shows "some" (its layers partly on).
  /** @type {HTMLInputElement} */ for (const box of document.querySelectorAll('input[data-map-group]'))
    box.indeterminate = /** @type {HTMLElement} */ (box).dataset.indeterminate === 'true'
  const el = document.getElementById('live-map')
  if (!el) {
    if (lm.map) teardown()
    return
  }
  if (!L) {
    loading ||= Promise.all([import('leaflet'), import('leaflet/dist/leaflet.css')]).then(([mod]) => {
      L = mod.default || mod
      render()
    })
    return
  }
  const { map, floor, members } = shown()
  if (!map || !drawable(map)) {
    if (lm.map) teardown()
    return
  }
  if (lm.el !== el || lm.key !== map.key) build(el, map)
  tickClocks()
  applyTextScale()
  drawBase(map, floor)
  const data = currentData()
  if (!data) fetchOnce('markers:' + map.key, 'mapMarkers', { map: map.key })
  drawThings(map, floor, data)
  drawSquad(map, floor, members)
  drawLines(L, lm.map, map.key, floor, offAlpha())
  if (pen.on) pinning = false
  setPinning(pinning)
  drawPins(map, floor)
  lm.map.getContainer().classList.toggle('map-pinning', pinning)
  followSquad(map, members)
  if (view.focus?.map === map.key) {
    const { x, z, zoom, pulse, keep, rings } = view.focus
    view.focus = null
    const near = Number.isFinite(zoom)
      ? zoom
      : keep
        ? lm.map.getZoom()
        : Math.max(lm.map.getZoom(), ((map.minZoom || 2) + Math.max(7, map.maxZoom || 6)) / 2)
    lm.map.setView(L.latLng(z, x), near)
    // A view shared: a ring in the sharer's colour where they looked.
    if (pulse) {
      const ring = L.marker(L.latLng(z, x), {
        interactive: false,
        keyboard: false,
        zIndexOffset: 3000,
        icon: L.divIcon({
          className: 'map-look-icon',
          html: `<span class="map-look ${rings ? 'rings' : ''}" style="--c:${esc(pulse)}"></span>`,
          iconSize: [0, 0],
        }),
      }).addTo(lm.map)
      setTimeout(() => ring.remove(), 3200)
    }
  }
}
afterRenderHooks.push(drawMap)

document.addEventListener('change', (event) => {
  const el = /** @type {HTMLInputElement} */ (event.target)
  if (el.dataset?.mapLayer) {
    const hidden = new Set(state.mapHidden || [])
    if (el.checked) hidden.delete(el.dataset.mapLayer)
    else hidden.add(el.dataset.mapLayer)
    void action('mapHidden', [...hidden])
  } else if (el.dataset?.mapGroup) {
    const { map } = shown()
    const keys = layersOf(map, currentData())
      .filter((l) => l.group === el.dataset.mapGroup)
      .map((l) => l.key)
    const hidden = new Set(state.mapHidden || [])
    for (const k of keys) el.checked ? hidden.delete(k) : hidden.add(k)
    void action('mapHidden', [...hidden])
  } else if (el.dataset?.mapScale) {
    void action('mapSettings', { [el.dataset.mapScale]: Number(el.value) })
  } else if (el.dataset?.mapSetting) {
    void action('mapSettings', { [el.dataset.mapSetting]: el.checked })
  }
})
let searchTimer = 0
document.addEventListener('input', (event) => {
  const el = /** @type {HTMLInputElement} */ (event.target)
  // The search waits for a pause in typing, as tarkov.dev's does.
  // A text size follows the slider at once; it is kept when let go (change).
  if (el.dataset?.mapScale) {
    const key = el.dataset.mapScale
    applyTextScale({ ...state.mapSettings, [key]: Number(el.value) })
    const out = document.querySelector(`[data-map-scale-out="${key}"]`)
    if (out) out.textContent = `${el.value}%`
  }
  if (el.id === 'map-search') {
    view.search = el.value
    clearTimeout(searchTimer)
    searchTimer = setTimeout(render, 300)
  }
})
// Esc stops putting a pin, closes an open panel, then clears the search.
document.addEventListener('keydown', (event) => {
  if (event.key !== 'Escape' || !document.getElementById('live-map')) return
  if (pinning) pinning = false
  else if (view.panel) view.panel = ''
  else if (view.search) view.search = ''
  else return
  render()
})

// snapMap makes the map as it shows (without its buttons) into a snap note,
// opened for drawing. The tiles come through Go: tarkov.dev does not serve
// them for a page to read.
let snapping = false
// A pin is being put: each press on the map puts it there, until the mode
// ends (its button again, Esc, a pen taken up). pinPut: it was put once in
// this mode (the next press moves it).
let pinning = false
let pinPut = false
async function snapMap() {
  const el = document.getElementById('live-map')
  const { map, floor } = shown()
  if (!el || !map || snapping) return
  snapping = true
  render()
  try {
    const { domToPng } = await import('modern-screenshot')
    const scale = Math.min(2, window.devicePixelRatio || 1)
    const image = await domToPng(el, {
      scale,
      backgroundColor: getComputedStyle(el).backgroundColor,
      // The lines (the squad's, yours over them) go in the note's first
      // layer instead, to stay editable; the others are left to write on.
      filter: (node) =>
        !(
          node instanceof Element &&
          (node.classList.contains('leaflet-control-container') || node.classList.contains('leaflet-mapDraw-pane'))
        ),
      fetchFn: async (url) =>
        url.startsWith('https://assets.tarkov.dev/')
          ? (await window.mayakDesktop?.backend?.BrowserMapTile?.(url)) || false
          : false,
    })
    const title = [t('liveMap'), mapName(map.key), map.layers?.length ? floorName(map, floor) : '']
      .filter(Boolean)
      .join(' · ')
    const layer = (strokes) => ({ visible: true, strokes })
    const drawing = {
      v: 2,
      base: { visible: true },
      layers: [layer([...squadSnapStrokes(scale), ...snapStrokes(scale)]), layer([]), layer([])],
    }
    await action('snapNew', { image, title, drawing })
  } catch (e) {
    await action('mapSnapFailed', String(e?.message || e))
  } finally {
    snapping = false
    render()
  }
}

clickHandlers.push(async (type, id, button) => {
  if (type === 'mapPanel') {
    view.panel = view.panel === id ? '' : id || ''
    render()
    if (view.panel === 'search') /** @type {HTMLInputElement} */ (document.getElementById('map-search'))?.focus()
    return true
  }
  // A member clicked in the list: their map and floor, centred on them.
  if (type === 'squadFocus') {
    const { members } = shown()
    const m = placed(members).find((p) => (p.me ? 'me' : p.id) === id)
    const map = m && findMap(state.squad?.maps, m.map)
    if (!map) return true
    view.map = map.key
    view.floor = floorFor(map, m.pos)
    // At the zoom the map has (none when it opens anew), with rings in
    // their colour where they are.
    view.focus = { map: map.key, x: m.pos.x, z: m.pos.z, keep: true, pulse: colorOf(m), rings: true }
    // From the sidebar or the squad's page, the map comes forward.
    if (state.tabs.find((tab) => tab.id === state.active)?.kind !== 'livemap') await action('livemap')
    else render()
    return true
  }
  if (type === 'mapSnap') {
    void snapMap()
    return true
  }
  // "Look here": presses on the map put a pin there (the pens go down
  // meanwhile), until this is pressed again.
  if (type === 'mapPin') {
    pinning = !pinning && !!shown().map && canShare()
    pinPut = false
    if (pinning) putPenDown()
    render()
    return true
  }
  // Following the squad on or off; turned on, it takes the squad in at once.
  if (type === 'mapSquadFit') {
    const on = !state.mapSettings?.squadFit
    fitNow = on
    void action('mapSettings', { squadFit: on })
    return true
  }
  if (type === 'mapMarkerEffect' || type === 'mapMarkerColor' || type === 'mapMarkerShape') {
    const key = { mapMarkerEffect: 'markerEffect', mapMarkerColor: 'markerColor', mapMarkerShape: 'markerShape' }[type]
    void action('mapSettings', { [key]: id || '' })
    return true
  }
  if (type === 'mapPick') {
    view.map = id || 'auto'
    view.floor = 'auto'
    render()
    return true
  }
  if (type === 'mapTask') {
    void action('mapTask', { id, name: button?.dataset.name || '' })
    return true
  }
  if (type === 'mapMode') {
    void action('mapSettings', { mode: id })
    return true
  }
  if (type === 'mapStyle') {
    void action('mapSettings', { style: id === 'tile' ? 'tile' : 'svg' })
    return true
  }
  if (type === 'floorPick') {
    view.floor = id ?? 'auto'
    render()
    return true
  }
  if (type === 'mapFold') {
    const folded = new Set(state.mapCollapsed || [])
    if (folded.has(id)) folded.delete(id)
    else folded.add(id)
    void action('mapCollapsed', [...folded])
    return true
  }
  return false
})

// A map detected (a raid starts, a position screenshot) brings the map view
// back to following the map and floor played.
// A squadmate's drawing pressed in the squad's list shows that map.
window.addEventListener('mayak:map-show', (event) => {
  const d = /** @type {CustomEvent} */ (event).detail || {}
  const key = d.map
  if (!findMap(state.squad?.maps, key)) return
  view.map = key
  view.floor = 'auto'
  // A view shared: its floor, centre and zoom too.
  if (Number.isFinite(d.x) && Number.isFinite(d.z)) {
    view.floor = typeof d.floor === 'string' ? d.floor : 'auto'
    view.focus = { map: key, x: d.x, z: d.z, zoom: d.zoom, pulse: d.c || '#ffd166' }
  }
  if (state.tabs.find((tab) => tab.id === state.active)?.kind !== 'livemap') void action('livemap')
  else render()
})
window.addEventListener('mayak:map-follow', () => {
  view.map = 'auto'
  view.floor = 'auto'
})
