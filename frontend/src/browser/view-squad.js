import { age } from './item.js'
import { autoMap, findMap, floorFor, freshness, markerRotation, memberColor, placed } from './squad-geo.js'
import { state, esc, t, icon, option, action, clickHandlers, afterRenderHooks, render } from './shell-core.js'

// The squad map tab: the squad (join by code, the members) and a map with
// everyone's last position. The map is Leaflet over tarkov.dev's SVG maps
// (internal/mapdata makes the picture, one per floor). Leaflet is loaded
// when the tab first shows, and the map lives in an element the render
// leaves alone (data-keep); after each render, drawSquadMap brings it up to
// date with the state.

// What is typed in the forms, kept across renders.
const drafts = { code: '', name: null, notice: '' }
// The map and floor chosen ('auto' follows the players).
const view = { map: 'auto', floor: 'auto' }

// The squad sidebar entry, under the fixed views.
export function squadEntry() {
  if (!state.squad) return ''
  const active = state.tabs.find((t) => t.id === state.active)?.kind === 'squadmap'
  const count = state.squad.state?.members.length || 0
  return `<div class="tab map-entry squad-entry ${active ? 'active' : ''}"><button data-action="squadmap" class="tab-select" title="${esc(t('squadMapHelp'))}">${icon('squad', 'tab-icon')}<span class="tab-name">${esc(t('squadMap'))}</span>${count > 1 ? `<span class="squad-count">${count}</span>` : ''}</button></div>`
}

const mapName = (key) =>
  state.bosses?.maps?.find((m) => m.key === key)?.name ||
  String(key || '')
    .split('-')
    .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
    .join(' ')

// shown is the map and floor to draw now.
function shown() {
  const sq = state.squad
  const maps = sq?.maps || []
  const members = sq?.state?.members || []
  const key = view.map === 'auto' ? autoMap(members, state.host?.map || '') : view.map
  const map = findMap(maps, key) || (view.map === 'auto' ? maps.find((m) => m.svg) : null) || null
  const me = placed(members).find((m) => m.me)
  const floor =
    view.floor !== 'auto'
      ? view.floor
      : me && map && findMap(maps, me.map)?.key === map.key
        ? floorFor(map, me.pos)
        : ''
  return { map, floor, members }
}

function memberLine(m, maps) {
  const where = m.map
    ? m.pos
      ? `${esc(mapName(findMap(maps, m.map)?.key || m.map))} · ${esc(age(m.at, state.language))}`
      : esc(mapName(m.map))
    : esc(t('squadOutOfRaid'))
  const fade = m.map && m.pos ? freshness(m.at) : 'gone'
  return `<li class="squad-member ${fade}"><span class="squad-dot" style="--c:${memberColor(m.name)}"></span><span class="squad-member-name">${esc(m.name || '?')}${m.me ? ` <small>(${esc(t('squadYou'))})</small>` : ''}</span><span class="squad-member-where">${where}</span></li>`
}

function joinForm() {
  const name = drafts.name ?? state.squadName ?? ''
  return `<form id="squad-form" class="squad-form"><p class="hint">${esc(t('squadIntro'))}</p><label class="field"><span>${esc(t('squadName'))}</span><input name="squadName" maxlength="24" autocomplete="off" spellcheck="false" placeholder="${esc(t('squadNamePlaceholder'))}" value="${esc(name)}"></label><div class="squad-actions"><button class="primary" type="submit" value="create">${esc(t('squadCreate'))}</button><span class="squad-or">${esc(t('squadOr'))}</span><input name="squadCode" maxlength="12" autocomplete="off" spellcheck="false" placeholder="ABCD-1234" aria-label="${esc(t('squadCode'))}" value="${esc(drafts.code)}"><button type="submit" value="join">${esc(t('squadJoin'))}</button></div>${drafts.notice ? `<p class="squad-notice" role="alert">${esc(t(drafts.notice))}</p>` : ''}<p class="hint">${esc(t('squadPrivacy'))}</p></form>`
}

function squadHead() {
  const sq = state.squad
  const s = sq.state
  if (!s) return joinForm()
  const name = drafts.name ?? state.squadName ?? ''
  return `<div class="squad-joined"><div class="squad-code"><span>${esc(t('squadCode'))}</span><output>${esc(state.squadCode || s.code)}</output><button data-action="squadCopy" title="${esc(t('squadCopy'))}" aria-label="${esc(t('squadCopy'))}">${icon('copy')}<span>${esc(t(drafts.notice === 'squadCopied' ? 'squadCopied' : 'squadCopy'))}</span></button><span class="squad-phase" data-phase="${esc(s.phase)}">${esc(t('squadPhase_' + s.phase))}</span></div><form id="squad-name-form" class="squad-name"><label class="field"><span>${esc(t('squadName'))}</span><input name="squadName" maxlength="24" autocomplete="off" spellcheck="false" value="${esc(name)}"></label></form><button data-action="squadLeave">${esc(t('squadLeave'))}</button></div><ul class="squad-members">${s.members.map((m) => memberLine(m, sq.maps || [])).join('')}</ul>`
}

function mapControls(map, floor) {
  const maps = state.squad.maps || []
  const mapChoices = [['auto', t('squadAuto')], ...maps.filter((m) => m.svg).map((m) => [m.key, mapName(m.key)])]
  const floors = [
    ['auto', t('squadAuto')],
    ['', t('squadGround')],
    ...(map?.layers || []).map((l) => [l.svgLayer, l.name]),
  ]
  return `<div class="squad-map-tools"><label class="field"><span>${esc(t('squadMapLabel'))}</span><select id="squad-map-pick">${mapChoices.map(([v, l]) => option(v, l, view.map)).join('')}</select></label>${map?.layers?.length ? `<label class="field"><span>${esc(t('squadFloor'))}</span><select id="squad-floor-pick">${floors.map(([v, l]) => option(v, l, view.floor)).join('')}</select></label>` : ''}${view.floor === 'auto' && map?.layers?.length ? `<span class="hint">${esc(floor ? map.layers.find((l) => l.svgLayer === floor)?.name || floor : t('squadGround'))}</span>` : ''}</div>`
}

export function squadPage() {
  const sq = state.squad
  if (!sq) return `<div class="page"><p class="empty-tabs">${esc(t('squadUnavailable'))}</p></div>`
  const { map, floor } = shown()
  const note = !sq.maps
    ? t(sq.mapsError ? 'squadMapsFailed' : 'squadMapsLoading')
    : !map
      ? t('squadNoMap')
      : !map.svg
        ? t('squadTilesOnly')
        : ''
  return `<div class="page squad-page"><div class="bookmarks-head"><h1>${esc(t('squadMap'))}</h1></div>${squadHead()}${sq.maps ? mapControls(map, floor) : ''}<div class="squad-map-wrap"><div id="squad-map" data-keep="squadmap"></div>${note ? `<p class="squad-map-note">${esc(note)}</p>` : ''}</div></div>`
}

// Leaflet and the map drawn with it.
let L = null
let loading = null
const lm = { map: null, el: null, key: '', overlay: null, image: '', markers: new Map(), resize: null, tick: 0 }
const requested = new Set()

function teardown() {
  lm.resize?.disconnect()
  clearInterval(lm.tick)
  lm.map?.remove()
  Object.assign(lm, {
    map: null,
    el: null,
    key: '',
    overlay: null,
    image: '',
    markers: new Map(),
    resize: null,
    tick: 0,
  })
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

function build(el, map) {
  teardown()
  lm.el = el
  lm.key = map.key
  lm.map = L.map(el, {
    crs: crs(map),
    zoomSnap: 0.25,
    minZoom: Math.max(0, (map.minZoom || 2) - 2),
    maxZoom: (map.maxZoom || 6) + 1,
    attributionControl: true,
  })
  lm.map.attributionControl.setPrefix(false)
  lm.map.fitBounds(toBounds(map.bounds))
  lm.resize = new ResizeObserver(() => lm.map?.invalidateSize())
  lm.resize.observe(el)
  // Positions age while nothing else changes: redrawn each half minute
  // while the map shows.
  lm.tick = setInterval(render, 30000)
}

function markerHTML(m, map, floor) {
  const color = memberColor(m.name)
  const rot = markerRotation(m.pos.rot, map.rotation)
  return `<span class="squad-marker ${m.me ? 'me' : ''}" style="--c:${color}"><svg viewBox="0 0 24 24" style="transform:rotate(${rot}deg)"><path d="M12 2 19 21 12 16.5 5 21z"/></svg><span class="squad-label">${esc(m.name || '?')}</span></span>`
}

function drawMarkers(map, floor, members) {
  const seen = new Set()
  for (const m of placed(members)) {
    if (findMap(state.squad.maps, m.map)?.key !== map.key) continue
    const fresh = freshness(m.at)
    if (fresh === 'gone') continue
    const id = m.me ? 'me' : m.id
    seen.add(id)
    const dim = fresh === 'stale' || floorFor(map, m.pos) !== floor
    const html = markerHTML(m, map, floor)
    const at = L.latLng(m.pos.z, m.pos.x)
    let marker = lm.markers.get(id)
    if (!marker) {
      marker = L.marker(at, { interactive: false, keyboard: false, zIndexOffset: m.me ? 1000 : 0 })
      marker._html = ''
      marker.addTo(lm.map)
      lm.markers.set(id, marker)
    } else marker.setLatLng(at)
    if (marker._html !== html) {
      marker._html = html
      marker.setIcon(L.divIcon({ className: 'squad-marker-icon', html, iconSize: [28, 28], iconAnchor: [14, 14] }))
    }
    marker.setOpacity(dim ? 0.45 : 1)
  }
  for (const [id, marker] of lm.markers)
    if (!seen.has(id)) {
      marker.remove()
      lm.markers.delete(id)
    }
}

function drawSquadMap() {
  const el = document.getElementById('squad-map')
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
  if (!map?.svg) {
    if (lm.map) teardown()
    return
  }
  if (lm.el !== el || lm.key !== map.key) build(el, map)
  const key = map.key + '|' + floor
  const url = state.squad.images?.[key]
  if (!url) {
    if (!requested.has(key)) {
      requested.add(key)
      void action('squadMapImage', { map: map.key, layer: floor }).finally(() => requested.delete(key))
    }
  } else if (lm.image !== key) {
    lm.overlay?.remove()
    lm.overlay = L.imageOverlay(url, toBounds(map.svgBounds || map.bounds), {
      attribution: `Map: ${esc(map.author || 'Shebuka')} / tarkov-dev-svg-maps (CC BY-NC-SA 4.0) · tarkov.dev`,
    }).addTo(lm.map)
    lm.overlay.bringToBack()
    lm.image = key
  }
  drawMarkers(map, floor, members)
}
afterRenderHooks.push(drawSquadMap)

document.addEventListener('change', (event) => {
  const el = /** @type {HTMLSelectElement} */ (event.target)
  if (el.id === 'squad-map-pick') {
    view.map = el.value
    view.floor = 'auto'
    render()
  } else if (el.id === 'squad-floor-pick') {
    view.floor = el.value
    render()
  }
})
document.addEventListener('input', (event) => {
  const el = /** @type {HTMLInputElement} */ (event.target)
  if (el.name === 'squadName' && el.closest('.squad-page')) drafts.name = el.value
  if (el.name === 'squadCode' && el.closest('.squad-page')) drafts.code = el.value
})
document.addEventListener('submit', (event) => {
  const form = /** @type {HTMLFormElement} */ (event.target)
  if (form.id === 'squad-name-form') {
    event.preventDefault()
    const name = String(drafts.name ?? '').trim()
    if (name) void action('squadRename', name).then(() => (drafts.name = null))
    return
  }
  if (form.id !== 'squad-form') return
  event.preventDefault()
  const name = String(drafts.name ?? state.squadName ?? '').trim()
  const join = /** @type {SubmitEvent} */ (event).submitter?.getAttribute('value') === 'join'
  const code = drafts.code.toUpperCase().replace(/[\s-]/g, '')
  drafts.notice = !name ? 'squadNeedName' : join && code.length !== 8 ? 'squadInvalidCode' : ''
  if (drafts.notice) {
    render()
    return
  }
  void action(join ? 'squadJoin' : 'squadCreate', { code: drafts.code, name }).then((next) => {
    if (next?.squad?.state) Object.assign(drafts, { code: '', name: null, notice: '' })
  })
})
// A name changed and left without Enter is kept too.
document.addEventListener(
  'blur',
  (event) => {
    const el = /** @type {HTMLInputElement} */ (event.target)
    if (el?.name === 'squadName' && el.closest?.('#squad-name-form') && drafts.name != null) {
      const name = drafts.name.trim()
      if (name && name !== state.squadName) void action('squadRename', name).then(() => (drafts.name = null))
    }
  },
  true,
)

clickHandlers.push(async (type) => {
  if (type === 'squadCopy') {
    drafts.notice = 'squadCopied'
    await action('squadCopy')
    setTimeout(() => {
      if (drafts.notice === 'squadCopied') {
        drafts.notice = ''
        render()
      }
    }, 2000)
    return true
  }
  if (type === 'squadLeave') {
    Object.assign(drafts, { code: '', name: null, notice: '' })
    view.map = 'auto'
    view.floor = 'auto'
    void action('squadLeave')
    return true
  }
  return false
})
