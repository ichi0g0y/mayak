import { state, api, t, esc, icon, action, request, render, clickHandlers, afterRenderHooks } from './shell-core.js'
import { hostname, originalURL } from './state.js'
import { age } from './item.js'
import { canShare, shareSnap } from './squad-share.js'

// Snap notes: a capture of the page in view (or a pasted picture, an image
// file, a blank sheet) with drawing over it. The notes live on this computer
// (internal/snapnote); the shell has their list in state.snapNotes and the
// note open for drawing in state.snapNotes.open. A note taken from a page is
// linked to it: the page's toolbar button counts them, and the note opens
// the page again. Unlinked notes keep their page on record.

// The page a note belongs to: the original of a translated page, without
// its #fragment or a trailing slash.
export function snapPageKey(url) {
  try {
    const u = new URL(originalURL(url) || url)
    return u.origin + u.pathname.replace(/\/+$/, '') + u.search
  } catch {
    return ''
  }
}
const pageNotes = (url) => {
  const key = snapPageKey(url)
  if (!key) return []
  const notes = (state.snapNotes?.list || []).filter((n) => n.linked && snapPageKey(n.url) === key)
  return [...notes.filter((n) => n.favorite), ...notes.filter((n) => !n.favorite)]
}
const noteTime = (note) => age(note.updatedAt || note.createdAt, state.language)
const thumbHTML = (note) => {
  const src = state.snapNotes?.thumbs?.[note.id]
  return src ? `<img src="${src}" alt="">` : `<span class="shot-placeholder">${icon('image')}</span>`
}
const siteLine = (note) =>
  (note.url ? `${esc(hostname(note.url))}${note.linked ? '' : ` · ${esc(t('snapUnlinked'))}`}` : esc(t('snapSingle'))) +
  (note.spot?.map ? ` · ${esc(mapLabel(note.spot.map))}` : '')

// A note's position, from a game screenshot's file name: {map (tarkov.dev's
// route, "" when not known), x, y, z, rotation}. tarkov.dev's map page takes
// no position in its address, so a position is shown through the map
// connection (Remote Control), and a post carries it as text with the map's
// link.
const mapNames = {
  customs: 'Customs',
  factory: 'Factory',
  'night-factory': 'Night Factory',
  'ground-zero': 'Ground Zero',
  'ground-zero-21': 'Ground Zero 21+',
  interchange: 'Interchange',
  icebreaker: 'Icebreaker',
  'the-lab': 'The Lab',
  'the-labyrinth': 'The Labyrinth',
  lighthouse: 'Lighthouse',
  reserve: 'Reserve',
  shoreline: 'Shoreline',
  'streets-of-tarkov': 'Streets of Tarkov',
  terminal: 'Terminal',
  woods: 'Woods',
}
const mapLabel = (key) => mapNames[key] || key
const mapRoute = (key) => (key === 'ground-zero-21' ? 'ground-zero' : key)
const facing = (rotation) => Math.round(((Number(rotation) % 360) + 360) % 360)
const spotText = (spot) =>
  `X ${Math.round(spot.x)} / Z ${Math.round(spot.z)} · ${t('snapFacing')} ${facing(spot.rotation)}°`
function spotHTML(note) {
  const spot = note.spot
  if (!spot) return ''
  const place = spot.map
    ? `<strong>${esc(mapLabel(spot.map))}</strong> · ${esc(spotText(spot))}<button class="snap-spot-show" data-action="snapShowSpot" data-id="${esc(note.id)}" title="${esc(t('snapShowSpotHint'))}">${icon('map')}<span>${esc(t('snapShowSpot'))}</span></button>`
    : `${esc(spotText(spot))}<select class="snap-spot-map" data-spot-map="${esc(note.id)}" title="${esc(t('snapPickMapHint'))}" aria-label="${esc(t('snapPickMap'))}"><option value="">${esc(t('snapPickMap'))}</option>${Object.entries(
        mapNames,
      )
        .map(([k, name]) => `<option value="${k}">${esc(name)}</option>`)
        .join('')}</select>`
  return `<span class="snap-spot">${icon('map')}<span>${esc(t('snapSpot'))}:</span> ${place}</span>`
}
// A post's text: the title, and where it was taken with the map's link.
function postText(note, title) {
  const spot = note.spot
  if (!spot?.map) return title
  return `${title}\n${mapLabel(spot.map)} · ${spotText(spot)}\nhttps://tarkov.dev/map/${mapRoute(spot.map)}`
}

// The toolbar button of a web page: it opens the snap menu, and counts the
// page's notes.
export function snapButton(tab) {
  if (!state.snapNotes || tab?.kind !== 'web') return ''
  const count = pageNotes(tab.url).length
  return `<button class="snap-page${count ? ' has-notes' : ''}${menu && !menu.share ? ' on' : ''}" data-action="snapMenu" title="${esc(t('snapTake'))}" aria-label="${esc(t('snapTake'))}" aria-haspopup="menu" ${state.snapNotes.busy ? 'disabled' : ''}>${icon('snap')}${count ? `<span class="snap-count">${count}</span>` : ''}</button>`
}

// The menu under the button: the two captures, the page's notes and the
// list. It opens in the menu window (app_menu.go), above the page, which
// stays shown; the choice comes back through api.onMenu.
let menu = null
const menuWidth = 300
function openMenu(button) {
  const tab = state.tabs.find((t) => t.id === state.active)
  const r = button.getBoundingClientRect()
  const notes = tab ? pageNotes(tab.url).slice(0, 6) : []
  const items = [
    { id: 'capture:visible', icon: 'scan', title: t('snapVisible'), hint: t('snapVisibleHint') },
    { id: 'capture:full', icon: 'file', title: t('snapFull'), hint: t('snapFullHint') },
    ...(notes.length
      ? [
          { kind: 'label', title: t('snapThisPage') },
          ...notes.map((n) => ({
            id: 'open:' + n.id,
            title: n.title,
            hint: noteTime(n),
            thumb: state.snapNotes?.thumbs?.[n.id] || '',
            icon: 'image',
          })),
        ]
      : []),
    { kind: 'label', title: '' },
    { id: 'all', icon: 'image', title: t('snapShowAll') },
  ]
  menu = { tab: tab?.id }
  render()
  const left = Math.round(Math.max(4, Math.min(r.right - menuWidth, innerWidth - menuWidth - 4)))
  void request('menuShow', { id: 'snap', x: left, y: Math.round(r.bottom + 6), width: menuWidth, items }).catch(() => {
    menu = null
    render()
  })
}
api.onMenu((choice) => {
  if (!menu) return
  const was = menu
  menu = null
  render()
  if (was.share) {
    if (choice.startsWith('share:')) void share(choice.slice(6))
    return
  }
  if (choice === 'capture:visible' || choice === 'capture:full')
    void action('snapCapture', { full: choice === 'capture:full' })
  else if (choice.startsWith('open:')) void action('snapOpen', choice.slice(5))
  else if (choice === 'all') void action('snapnotes')
})
// The menu draws in its own window; nothing in the shell.
export function snapMenuHTML() {
  return ''
}

// The sidebar section: the latest note, and the button to the list.
export function snapSection() {
  const notes = state.snapNotes
  if (!notes) return ''
  const open = state.tabs.find((tab) => tab.id === state.active)?.kind === 'snapnotes'
  const folded = state.snapNotesCollapsed
  const latest = notes.list[0]
  const preview = latest
    ? `<button class="shot-latest snap-latest" data-action="snapOpen" data-id="${esc(latest.id)}" title="${esc(latest.title)}">${thumbHTML(latest)}<span class="shot-age">${esc(noteTime(latest))}</span></button>`
    : `<p class="shot-empty">${esc(t('snapNewBlank'))}…</p>`
  return `<div class="section-label screenshot-section-label snap-section-label ${open ? 'active' : ''}"><button class="section-link" data-action="toggleSnapSection" aria-expanded="${!folded}" title="${esc(t(folded ? 'expandSection' : 'collapseSection'))}">${esc(t('snapNotes'))}${icon('chevron', 'section-chevron')}</button><button class="new-tab shots-open" data-action="snapnotes" title="${esc(t('snapNotesAll'))}" aria-label="${esc(t('snapNotesAll'))}" aria-pressed="${open}">${icon('snap')}</button></div>${folded ? '' : `<div class="shot-section">${preview}</div>`}`
}

// A card's delete, at the other corner from the star: the first press arms
// it (it turns red and says so), a second within a few seconds deletes.
const cardDeleteButton = (note) => {
  const armed = armedDelete === note.id
  const label = t(armed ? 'snapDeleteConfirm' : 'snapDelete')
  return `<button class="snap-card-delete${armed ? ' armed' : ''}" data-action="snapDeleteNote" data-id="${esc(note.id)}" title="${esc(label)}" aria-label="${esc(label)}">${icon('trash')}${armed ? `<span>${esc(label)}</span>` : ''}</button>`
}
// The star of a note: on its card and in the editor's head.
const starButton = (note, cls) =>
  `<button class="snap-star ${cls}${note.favorite ? ' on' : ''}" data-action="snapFavorite" data-id="${esc(note.id)}" title="${esc(t(note.favorite ? 'snapUnfavorite' : 'snapFavorite'))}" aria-label="${esc(t(note.favorite ? 'snapUnfavorite' : 'snapFavorite'))}" aria-pressed="${!!note.favorite}">${icon('star')}</button>`

// The page: the list of notes, or the note open for drawing.
export function snapNotesPage() {
  const notes = state.snapNotes
  if (!notes) return `<div class="page"><p class="empty-tabs">${esc(t('snapEmpty'))}</p></div>`
  return notes.open ? editorHTML(notes.open) : listHTML(notes)
}
function listHTML(notes) {
  const filter = notes.filter || 'all'
  const shown = notes.list.filter(
    (n) => filter === 'all' || (filter === 'favorite' ? n.favorite : filter === 'linked' ? n.linked : !n.linked),
  )
  const chip = (value, label) =>
    `<button class="segment${filter === value ? ' selected' : ''}" data-action="snapFilter" data-id="${value}" aria-pressed="${filter === value}">${esc(t(label))}</button>`
  const cards = shown
    .map(
      (n) =>
        `<div class="snap-card-wrap"><button class="snap-card" data-action="snapOpen" data-id="${esc(n.id)}" title="${esc(n.title)}"><span class="snap-card-thumb">${thumbHTML(n)}</span><span class="snap-card-text"><strong>${esc(n.title)}</strong><small>${n.from?.name ? `${esc(t('snapFrom').replace('{name}', n.from.name))} · ` : ''}${siteLine(n)} · ${esc(noteTime(n))}</small></span></button>${starButton(n, 'snap-card-star')}${cardDeleteButton(n)}</div>`,
    )
    .join('')
  return `<div class="page snap-page-list"><div class="bookmarks-head"><h1>${esc(t('snapNotes'))}</h1><div class="bookmarks-tools"><button data-action="snapNewBlank">${icon('plus')}<span>${esc(t('snapNewBlank'))}</span></button><button data-action="snapNewFile">${icon('image')}<span>${esc(t('snapNewImage'))}</span></button><input type="file" id="snap-file" accept="image/*" hidden></div></div><div class="snap-filters"><div class="segmented" role="group">${chip('all', 'snapFilterAll')}${chip('favorite', 'snapFilterFavorite')}${chip('linked', 'snapFilterLinked')}${chip('single', 'snapFilterSingle')}</div><p class="hint">${esc(t('snapPasteHint'))}</p></div>${notes.list.length ? (shown.length ? `<div class="snap-grid">${cards}</div>` : `<p class="shot-empty">${esc(t('snapNoMatch'))}</p>`) : `<p class="shot-empty">${esc(t('snapEmpty'))}</p>`}</div>`
}

// The editor. Strokes are in the image's pixels: {c: colour, w: width, p:
// [[x,y],…]}. The image and the ink canvas keep what they show across
// renders (data-keep); afterRender paints them.
const colors = ['#ff3b30', '#ffd60a', '#34c759', '#32ade6', '#ffffff', '#111111']
const sizes = /** @type {[string,number][]} */ ([
  ['s', 3],
  ['m', 6],
  ['l', 12],
])
let tool = 'pen',
  color = colors[0],
  size = 'm',
  zoom = /** @type {'fit'|number} */ ('fit'),
  armedDelete = '',
  saving = false
let ed = null
const lineWidth = (note) => sizes.find(([k]) => k === size)[1] * Math.max(1, note.width / 1000)
// The ink canvas is at most about 24 million pixels; a larger image is inked
// at a smaller scale.
const inkScale = (note) => Math.min(1, Math.sqrt(24e6 / (note.width * note.height)))

// The editor's head sits in the toolbar row above the page (where a web
// page has its address): back, the title, the save state and the note's
// actions. Only while a note is open; the list has its own heading.
export function snapToolbar() {
  const open = state.snapNotes?.open
  if (!open) return ''
  const note = open.note
  const link = note.url
    ? note.linked
      ? `<button data-action="snapLinkToggle" title="${esc(t('snapUnlink'))}">${icon('unlinked')}<span>${esc(t('snapUnlink'))}</span></button>`
      : `<button data-action="snapLinkToggle" title="${esc(t('snapRelink'))}">${icon('linked')}<span>${esc(t('snapRelink'))}</span></button>`
    : ''
  const page = note.url
    ? `<button data-action="snapOpenPage" data-id="${esc(note.id)}" title="${esc(note.url)}">${icon('external')}<span>${esc(t('snapOpenPage'))}</span></button>`
    : ''
  const status = textEdit
    ? t('snapTextHint')
    : shareMessage || (saving ? t('snapSaving') : ed?.failed ? t('snapSaveFailed') : ed?.dirty ? '' : t('snapSaved'))
  const share = `<button class="${menu?.share ? 'on' : ''}" data-action="snapShare" title="${esc(t('snapShare'))}" aria-haspopup="menu">${icon('share')}<span>${esc(t('snapShare'))}</span></button>`
  return `<div class="snap-head"><button data-action="snapBack" title="${esc(t('snapBack'))}" aria-label="${esc(t('snapBack'))}">${icon('back')}</button><input id="snap-title" class="snap-title" ${ed?.editing ? '' : 'readonly'} value="${esc(ed?.id === note.id ? ed.title : note.title)}" placeholder="${esc(t('snapTitlePlaceholder'))}" title="${esc(t('snapRename'))}" aria-label="${esc(t('snapTitle'))}" maxlength="160">${starButton(note, 'snap-head-star')}<span class="snap-status">${esc(status)}</span><div class="snap-actions">${share}${page}${link}<button class="snap-delete${armedDelete === note.id ? ' armed' : ''}" data-action="snapDeleteNote" data-id="${esc(note.id)}">${icon('trash')}<span>${esc(t(armedDelete === note.id ? 'snapDeleteConfirm' : 'snapDelete'))}</span></button></div></div>`
}

// fromHTML names the squadmate a shared note came from.
function fromHTML(note) {
  const from = note.from
  if (!from?.name) return ''
  const c = /^#[0-9a-f]{6}$/i.test(from.color || '') ? from.color : 'var(--muted)'
  return `<span class="snap-from"><span class="squad-dot" style="--c:${c}"></span>${esc(t('snapFrom').replace('{name}', from.name))}</span>`
}
// viewToolsHTML is the tools while a note is looked at: "Edit", and the zoom.
function viewToolsHTML(note) {
  return `<div class="snap-tools snap-view-tools" role="toolbar"><button class="snap-edit" data-action="snapEdit">${icon('brush')}<span>${esc(t('snapEdit'))}</span></button><span class="snap-sep"></span><button data-action="snapZoomOut" title="${esc(t('snapZoomOut'))}">${icon('minus')}</button><button class="snap-zoom-level" data-action="snapZoomReset" title="${esc(t('snapActual'))}">${Math.round(currentScale(note) * 100)}%</button><button data-action="snapZoomIn" title="${esc(t('snapZoomIn'))}">${icon('plus')}</button><button class="${zoom === 'fit' ? 'on' : ''}" data-action="snapZoomFit" title="${esc(t('snapFit'))}" aria-pressed="${zoom === 'fit'}">${icon('fit')}</button></div>`
}
function editorHTML(open) {
  const note = open.note
  const scale = inkScale(note)
  const editing = !!ed?.editing
  const tools = !editing
    ? viewToolsHTML(note)
    : `<div class="snap-tools" role="toolbar"><button class="snap-edit on" data-action="snapEditDone" title="${esc(t('snapEditDoneHint'))}">${icon('check')}<span>${esc(t('snapEditDone'))}</span></button><span class="snap-sep"></span>` +
      `<button class="snap-tool${tool === 'pen' ? ' on' : ''}" data-action="snapTool" data-id="pen" title="${esc(t('snapPen'))}" aria-pressed="${tool === 'pen'}">${icon('brush')}</button><button class="snap-tool${tool === 'text' ? ' on' : ''}" data-action="snapTool" data-id="text" title="${esc(t('snapText'))}" aria-pressed="${tool === 'text'}">${icon('type')}</button><button class="snap-tool${tool === 'eraser' ? ' on' : ''}" data-action="snapTool" data-id="eraser" title="${esc(t('snapEraser'))}" aria-pressed="${tool === 'eraser'}">${icon('eraser')}</button><span class="snap-sep"></span>${colors.map((c) => `<button class="snap-color${color === c && tool !== 'eraser' ? ' on' : ''}" data-action="snapColor" data-id="${c}" title="${esc(t('snapColor'))} ${c}" style="--swatch:${c}"></button>`).join('')}${customColorHTML()}<span class="snap-sep"></span>${tool === 'text' ? '' : sizes.map(([k, w]) => `<button class="snap-size${size === k ? ' on' : ''}" data-action="snapSize" data-id="${k}" title="${esc(t('snapSize' + k.toUpperCase()))}"><span style="--dot:${w + 2}px"></span></button>`).join('') + '<span class="snap-sep"></span>'}<button data-action="snapUndo" title="${esc(t('snapUndo'))}" ${ed?.undo.length ? '' : 'disabled'}>${icon('undo')}</button><button data-action="snapRedo" title="${esc(t('snapRedo'))}" ${ed?.redo.length ? '' : 'disabled'}>${icon('redo')}</button><button data-action="snapClear" title="${esc(t('snapClear'))}" ${ed?.layers[ed.active].strokes.length ? '' : 'disabled'}>${icon('trash')}</button><span class="snap-sep"></span><button data-action="snapZoomOut" title="${esc(t('snapZoomOut'))}">${icon('minus')}</button><button class="snap-zoom-level" data-action="snapZoomReset" title="${esc(t('snapActual'))}">${Math.round(currentScale(note) * 100)}%</button><button data-action="snapZoomIn" title="${esc(t('snapZoomIn'))}">${icon('plus')}</button><button class="${zoom === 'fit' ? 'on' : ''}" data-action="snapZoomFit" title="${esc(t('snapFit'))}" aria-pressed="${zoom === 'fit'}">${icon('fit')}</button>${tool === 'text' ? textPaletteHTML() : ''}</div>`
  return `<div class="page snap-editor ${editing ? 'editing' : 'viewing'}">${noticeHTML()}<p class="snap-where">${fromHTML(note)}${note.url ? `${esc(t(note.linked ? 'snapLinked' : 'snapUnlinked'))}: <span title="${esc(note.url)}">${esc(note.pageTitle || note.url)}</span>` : esc(t('snapNoPage'))}${spotHTML(note)}</p>${tools}<div class="snap-body"><div class="snap-stage ${zoom}" data-tool="${tool}">${adjustFilterSVG()}<div class="snap-sheet${ed && !ed.baseVisible ? ' base-hidden' : ''}" style="--snap-filter:${ed ? filterOf(ed.adjust) : 'none'};width:${zoom === 'fit' ? '100%' : Math.round(note.width * zoom) + 'px'};aspect-ratio:${note.width}/${note.height}"><img class="snap-base" data-keep="${esc(note.id)}" alt="" draggable="false"><canvas class="snap-ink" data-keep="${esc(note.id)}" width="${Math.round(note.width * scale)}" height="${Math.round(note.height * scale)}"></canvas>${textBoxHTML(note)}</div></div>${editing ? layersHTML() : ''}</div></div>`
}

// The layer panel: the three drawing layers over the original, top first.
// A layer is chosen to draw on; each, the original too, can be hidden.
function layersHTML() {
  if (!ed) return ''
  const eye = (action, id, visible) =>
    `<button class="snap-eye${visible ? '' : ' off'}" data-action="${action}" data-id="${id}" title="${esc(t(visible ? 'snapHide' : 'snapShow'))}" aria-pressed="${visible}">${icon(visible ? 'eye' : 'eyeOff')}</button>`
  const rows = [2, 1, 0]
    .map((i) => {
      const layer = ed.layers[i]
      return `<div class="snap-layer${ed.active === i ? ' on' : ''}${layer.visible ? '' : ' hidden'}">${eye('snapLayerEye', i, layer.visible)}<button class="snap-layer-name" data-action="snapLayer" data-id="${i}" aria-pressed="${ed.active === i}"><span>${esc(t('snapLayer').replace('{n}', String(i + 1)))}</span><small>${esc(countOf(layer.strokes))}</small></button><button class="snap-layer-clear" data-action="snapLayerClear" data-id="${i}" title="${esc(t('snapLayerClear'))}" ${layer.strokes.length ? '' : 'disabled'}>${icon('trash')}</button></div>`
    })
    .join('')
  return `<aside class="snap-layers" aria-label="${esc(t('snapLayers'))}"><h3>${esc(t('snapLayers'))}</h3>${rows}<div class="snap-layer base${ed.baseVisible ? '' : ' hidden'}">${eye('snapBaseEye', 'base', ed.baseVisible)}<span class="snap-layer-name"><span>${esc(t('snapOriginal'))}</span><small>${esc(t('snapOriginalHint'))}</small></span>${icon('lock')}</div>${adjustHTML()}</aside>`
}
function adjustHTML() {
  const row = (key, label, format) => {
    const [lo, hi, step] = adjustRanges[key]
    const v = ed.adjust[key]
    return `<label class="snap-adjust-row"><span>${esc(t(label))}</span><output data-adjust-out="${key}">${format(v)}</output><input type="range" data-adjust="${key}" min="${lo}" max="${hi}" step="${step}" value="${v}"></label>`
  }
  const changedAny = ed.adjust.brightness !== 1 || ed.adjust.contrast !== 1 || ed.adjust.shadows !== 0
  return `<div class="snap-adjust"><div class="snap-adjust-head"><span>${esc(t('snapAdjust'))}</span><button data-action="snapAdjustReset" ${changedAny ? '' : 'disabled'}>${esc(t('snapAdjustReset'))}</button></div>${row('brightness', 'snapBrightness', adjustPercent)}${row('contrast', 'snapContrast', adjustPercent)}${row('shadows', 'snapShadows', adjustPercent)}<p class="hint">${esc(t('snapAdjustHint'))}</p></div>`
}
const countOf = (strokes) => {
  const texts = strokes.filter(isText).length
  const lines = t('snapLines').replace('{n}', String(strokes.length - texts))
  return texts ? `${lines} · ${t('snapTexts').replace('{n}', String(texts))}` : lines
}
const adjustPercent = (v) => Math.round(v * 100) + '%'
// The shadows filter: a gamma on each colour channel.
function adjustFilterSVG() {
  const e = ed ? gammaOf(ed.adjust.shadows) : 1
  return `<svg class="snap-filter-defs" width="0" height="0" aria-hidden="true"><filter id="snap-shadows" color-interpolation-filters="sRGB"><feComponentTransfer><feFuncR type="gamma" exponent="${e}"/><feFuncG type="gamma" exponent="${e}"/><feFuncB type="gamma" exponent="${e}"/></feComponentTransfer></filter></svg>`
}

// How the original shows: brightness (0.5–3), contrast (0.5–2) and shadows,
// a gamma lifting the dark parts (1 none, down to 0.3). EFT's screenshots are
// often dark; this is for looking into them. The image kept never changes.
const adjustRanges = { brightness: [0.5, 3, 0.05], contrast: [0.5, 2, 0.05], shadows: [0, 1, 0.05] }
const defaultAdjust = () => ({ brightness: 1, contrast: 1, shadows: 0 })
function readAdjust(base) {
  const adjust = defaultAdjust()
  for (const [key, [lo, hi]] of Object.entries(adjustRanges)) {
    const v = Number(base?.[key])
    if (Number.isFinite(v)) adjust[key] = Math.min(hi, Math.max(lo, v))
  }
  return adjust
}
// The shadows slider (0 none, 1 most) is the gamma's exponent from 1 to 0.3.
const gammaOf = (shadows) => Math.round((1 - shadows * 0.7) * 1000) / 1000
const filterOf = (adjust) =>
  `${adjust.shadows > 0 ? 'url(#snap-shadows) ' : ''}brightness(${adjust.brightness}) contrast(${adjust.contrast})`

// A note's drawing: {v:2, base:{visible, brightness, contrast, shadows}, layers:[{visible, strokes}]×3}.
// A note drawn before layers (a list of strokes) has them on layer 1.
const layerCount = 3
const validStrokes = (list) =>
  Array.isArray(list) ? list.filter((s) => s && typeof s.c === 'string' && Array.isArray(s.p)) : []
function readDrawing(saved) {
  const layers = Array.from({ length: layerCount }, () => ({ visible: true, strokes: [] }))
  let baseVisible = true,
    adjust = defaultAdjust()
  if (Array.isArray(saved)) layers[0].strokes = validStrokes(saved)
  else if (saved && typeof saved === 'object') {
    baseVisible = saved.base?.visible !== false
    adjust = readAdjust(saved.base)
    ;(Array.isArray(saved.layers) ? saved.layers : []).slice(0, layerCount).forEach((l, i) => {
      layers[i] = { visible: l?.visible !== false, strokes: validStrokes(l?.strokes) }
    })
  }
  return { layers, baseVisible, adjust }
}
const writeDrawing = (target) => ({
  v: 2,
  base: { visible: target.baseVisible, ...target.adjust },
  layers: target.layers.map((l) => ({ visible: l.visible, strokes: l.strokes })),
})

// The editor follows the note open: a new one starts with its strokes; a
// note left with unsaved drawing is saved first.
function syncEditor() {
  const open = state.snapNotes?.open
  if (ed && (!open || open.note.id !== ed.id)) {
    if (ed.dirty) void saveNow(ed)
    ed = null
    textEdit = null
    armedDelete = ''
  }
  if (open && !ed) {
    const drawing = readDrawing(open.strokes)
    ed = {
      id: open.note.id,
      note: open.note,
      layers: drawing.layers,
      baseVisible: drawing.baseVisible,
      adjust: drawing.adjust,
      active: 0,
      undo: [],
      redo: [],
      dirty: false,
      title: open.note.title,
      // A note opens to be looked at (its tools, the layers and the title
      // wait for "Edit"), but one just made to draw on (a capture, a blank
      // sheet, an image: within a few seconds, and not one a squadmate sent).
      editing: !open.note.from && Date.now() - Date.parse(open.note.createdAt || '') < 15000,
      img: null,
      version: 1,
      thumbed: !!state.snapNotes.thumbs[open.note.id],
    }
    const img = new Image()
    const mine = ed
    img.onload = () => {
      if (ed !== mine) return
      mine.img = img
      mine.version++
      paint()
      if (!mine.thumbed) {
        mine.thumbed = true
        void saveNow(mine)
      }
    }
    img.src = open.image
  }
  if (ed && open) ed.note = open.note
}
function paint() {
  if (!ed) return
  const base = document.querySelector('.snap-base')
  if (base && ed.img && base.dataset.src !== ed.id) {
    base.src = ed.img.src
    base.dataset.src = ed.id
  }
  const canvas = document.querySelector('.snap-ink')
  if (!canvas) return
  // Only a change of the strokes redraws: a render in the middle of a line
  // must not wipe it.
  const version = ed.id + '/' + ed.version
  if (canvas.dataset.v === version) return
  canvas.dataset.v = version
  const ctx = canvas.getContext('2d'),
    scale = canvas.width / ed.note.width
  ctx.clearRect(0, 0, canvas.width, canvas.height)
  for (const layer of ed.layers)
    if (layer.visible) for (const s of layer.strokes) if (s !== textEdit?.item) drawStroke(ctx, s, scale)
}
function drawStroke(ctx, s, scale) {
  if (isText(s)) return drawText(ctx, s, scale)
  if (!s.p.length) return
  ctx.save()
  ctx.scale(scale, scale)
  ctx.lineCap = 'round'
  ctx.lineJoin = 'round'
  ctx.strokeStyle = s.c
  ctx.lineWidth = s.w
  ctx.beginPath()
  ctx.moveTo(s.p[0][0], s.p[0][1])
  if (s.p.length === 1) ctx.lineTo(s.p[0][0] + 0.01, s.p[0][1])
  for (let i = 1; i < s.p.length; i++) ctx.lineTo(s.p[i][0], s.p[i][1])
  ctx.stroke()
  ctx.restore()
}
// Text: an item of a layer like a line, {c, w: the font size, p: [[x, y]]
// (its top left), text}, in the image's pixels. It is typed in a box over
// the page and drawn with an outline, so it reads on any picture.
const defaultFamilies = '"Segoe UI","Yu Gothic UI",Meiryo,sans-serif'
const families = (f) => (f ? `"${String(f).replace(/["\\]/g, '')}",${defaultFamilies}` : defaultFamilies)
const textFont = (px, f) => `600 ${px}px ${families(f)}`
const textLine = 1.25
const isText = (s) => typeof s.text === 'string'
// The size is in points per 1000 pixels of the image's width (at least 1),
// like the pen's width, so it reads the same on a small and a large capture.
const textScale = (note) => Math.max(1, note.width / 1000)
const textSize = (note) => textStyle.size * textScale(note)
// The text palette: size, and the outline's colour ("auto": white or black
// against the text's colour) and width (a percentage of the size, 0 none).
const textStyle = { size: 32, outline: 'auto', width: 8, font: '' }
const outlineColor = (s) => (s.o && s.o !== 'auto' ? s.o : outlineOf(s.c))
const outlineWidth = (s) => ((s.ow ?? 8) / 100) * s.w
const hexColor = (c) => (/^#[0-9a-f]{6}$/i.test(c) ? c : '#ffffff')
function customColorHTML() {
  if (tool === 'eraser') return ''
  return `<label class="snap-custom-color${colors.includes(color) ? '' : ' on'}" title="${esc(t('snapMoreColors'))}" style="--swatch:${hexColor(color)}"><input type="color" data-color-input value="${hexColor(color)}" aria-label="${esc(t('snapMoreColors'))}"></label>`
}
function textPaletteHTML() {
  const auto = textStyle.outline === 'auto'
  const swatch = (c) =>
    `<button class="snap-color snap-outline-color${textStyle.outline === c ? ' on' : ''}" data-action="snapOutline" data-id="${c}" title="${esc(t('snapOutlineColor'))} ${c}" style="--swatch:${c}"></button>`
  const custom = !auto && !colors.includes(textStyle.outline)
  const fonts = state.snapNotes?.fonts || []
  const known = !textStyle.font || fonts.includes(textStyle.font)
  const option = (f, label) =>
    `<option value="${esc(f)}"${f === textStyle.font ? ' selected' : ''} style="font-family:${esc(families(f))}">${esc(label)}</option>`
  const font = `<span class="snap-group"><span class="snap-label">${esc(t('snapFont'))}</span><select class="snap-font" data-text-font aria-label="${esc(t('snapFont'))}" style="font-family:${esc(families(textStyle.font))}">${option('', t('snapFontDefault'))}${known ? '' : option(textStyle.font, textStyle.font)}${fonts.map((f) => option(f, f)).join('')}</select></span>`
  return `<div class="snap-text-style">${font}<span class="snap-group"><span class="snap-label">${esc(t('snapTextSize'))}</span><label class="snap-range"><input type="range" data-text-style="size" min="8" max="200" step="1" value="${textStyle.size}" aria-label="${esc(t('snapTextSize'))}"><output data-text-out="size">${textStyle.size}</output></label></span><span class="snap-group"><span class="snap-label">${esc(t('snapOutlineColor'))}</span><button class="snap-outline-auto${auto ? ' on' : ''}" data-action="snapOutline" data-id="auto" title="${esc(t('snapOutlineAutoHint'))}">${esc(t('snapOutlineAuto'))}</button>${colors.map(swatch).join('')}<label class="snap-custom-color${custom ? ' on' : ''}" title="${esc(t('snapMoreColors'))}" style="--swatch:${hexColor(textStyle.outline)}"><input type="color" data-outline-input value="${hexColor(textStyle.outline)}" aria-label="${esc(t('snapOutlineColor'))}"></label></span><span class="snap-group"><span class="snap-label">${esc(t('snapOutlineWidth'))}</span><label class="snap-range"><input type="range" data-text-style="width" min="0" max="30" step="1" value="${textStyle.width}" aria-label="${esc(t('snapOutlineWidth'))}"><output data-text-out="width">${textStyle.width}%</output></label></span></div>`
}
// A dark outline around a light colour, a light one around a dark colour.
function outlineOf(c) {
  const n = parseInt(String(c).slice(1), 16) || 0
  return 0.299 * (n >> 16) + 0.587 * ((n >> 8) & 255) + 0.114 * (n & 255) > 140
    ? 'rgba(0,0,0,.85)'
    : 'rgba(255,255,255,.9)'
}
function drawText(ctx, s, scale) {
  if (!s.p.length) return
  ctx.save()
  ctx.scale(scale, scale)
  ctx.font = textFont(s.w, s.f)
  ctx.textBaseline = 'top'
  ctx.lineJoin = 'round'
  // The stroke is centred on the letters' edge: twice the outline's width.
  const outline = outlineWidth(s)
  ctx.lineWidth = outline * 2
  ctx.strokeStyle = outlineColor(s)
  ctx.fillStyle = s.c
  s.text.split('\n').forEach((line, i) => {
    const y = s.p[0][1] + i * s.w * textLine
    if (outline > 0) ctx.strokeText(line, s.p[0][0], y)
    ctx.fillText(line, s.p[0][0], y)
  })
  ctx.restore()
}
let measurer = null
function textBounds(s) {
  measurer ||= document.createElement('canvas').getContext('2d')
  measurer.font = textFont(s.w, s.f)
  const lines = s.text.split('\n')
  const width = Math.max(s.w / 2, ...lines.map((line) => measurer.measureText(line).width))
  return { x: s.p[0][0], y: s.p[0][1], w: width, h: lines.length * s.w * textLine }
}
const inBounds = (point, b, pad) =>
  point[0] >= b.x - pad && point[0] <= b.x + b.w + pad && point[1] >= b.y - pad && point[1] <= b.y + b.h + pad
// The text under a point: the topmost, of the layers shown.
function textAt(point) {
  const pad = 4 * Math.max(1, ed.note.width / 1000)
  for (let layer = layerCount - 1; layer >= 0; layer--) {
    if (!ed.layers[layer].visible) continue
    const strokes = ed.layers[layer].strokes
    for (let i = strokes.length - 1; i >= 0; i--)
      if (isText(strokes[i]) && inBounds(point, textBounds(strokes[i]), pad)) return { layer, item: strokes[i] }
  }
  return null
}

// The text being typed: a new one (item null) or one being changed. Its box
// sits over the page at the text's place, sized with the page (cqw), and
// stays across renders (data-keep); the canvas leaves the text out meanwhile.
// Clicking outside the box, Esc or Ctrl+Enter finishes it; an emptied text
// is removed.
let textEdit = null,
  textKeys = 0
function textBoxHTML(note) {
  const e = textEdit
  if (!e || !ed || ed.id !== note.id) return ''
  return `<div class="snap-text-box" data-keep="text-${e.key}" style="left:${(e.x / note.width) * 100}%;top:${(e.y / note.height) * 100}%;${boxStyle(e, note)}"><span class="snap-text-grip" title="${esc(t('snapTextMove'))}">${icon('move')}</span><textarea class="snap-text-input" spellcheck="false" rows="1" aria-label="${esc(t('snapText'))}">${esc(e.value)}</textarea></div>`
}
// The box's look, in the page's width units (cqw) so it follows the zoom.
function boxStyle(e, note) {
  return `--size:${e.w / note.width};--ow:${outlineWidth(e) / note.width};--family:${esc(families(e.f))};color:${esc(e.c)};--outline:${esc(outlineColor(e))}`
}
function editText(point, found) {
  if (!ed) return
  if (found) ed.active = found.layer
  const item = found?.item || null
  // A text taken for editing shows its style in the palette.
  if (item) {
    color = item.c
    textStyle.size = Math.round(item.w / textScale(ed.note))
    textStyle.outline = item.o || 'auto'
    textStyle.width = item.ow ?? 8
    textStyle.font = item.f || ''
  }
  textEdit = {
    key: ++textKeys,
    layer: ed.active,
    item,
    x: item ? item.p[0][0] : point[0],
    y: item ? item.p[0][1] : point[1],
    c: item ? item.c : color,
    w: item ? item.w : textSize(ed.note),
    o: textStyle.outline,
    ow: textStyle.width,
    f: textStyle.font,
    value: item ? item.text : '',
  }
  ed.version++
  render()
}
function finishText() {
  const e = textEdit
  textEdit = null
  if (!e || !ed) return
  const box = document.querySelector('.snap-text-input')
  const value = (box ? box.value : e.value).replace(/\s+$/, '')
  const strokes = ed.layers[e.layer].strokes
  const at = e.item ? strokes.indexOf(e.item) : -1
  const moved = e.item && (e.item.p[0][0] !== e.x || e.item.p[0][1] !== e.y)
  const same =
    e.item &&
    !moved &&
    e.item.text === value &&
    e.item.c === e.c &&
    e.item.w === e.w &&
    (e.item.o || 'auto') === e.o &&
    (e.item.ow ?? 8) === e.ow &&
    (e.item.f || '') === e.f
  ed.version++
  if (same || (!value.trim() && at < 0)) {
    render()
    return
  }
  remember()
  ed.layers[e.layer].visible = true
  // A new item, not the old one changed: the undo keeps the old.
  const next = {
    ...e.item,
    p: [[e.x, e.y]],
    c: e.c,
    w: e.w,
    o: e.o === 'auto' ? undefined : e.o,
    ow: e.ow,
    f: e.f || undefined,
    text: value,
  }
  if (at < 0) strokes.push(next)
  else if (value.trim()) strokes[at] = next
  else strokes.splice(at, 1)
  changed()
}
// The box after a colour or a size was chosen for its text.
function restyleTextBox(focus = true) {
  const wrap = document.querySelector('.snap-text-box'),
    box = document.querySelector('.snap-text-input')
  if (!wrap || !box || !textEdit || !ed) return
  wrap.style.color = textEdit.c
  wrap.style.setProperty('--outline', outlineColor(textEdit))
  wrap.style.setProperty('--size', String(textEdit.w / ed.note.width))
  wrap.style.setProperty('--ow', String(outlineWidth(textEdit) / ed.note.width))
  wrap.style.setProperty('--family', families(textEdit.f))
  sizeTextBox(box)
  if (focus) box.focus()
}
// The palette's controls: choosing in them keeps the text being typed.
const paletteControl = '.snap-color,.snap-size,.snap-text-style,.snap-custom-color,.snap-font'
// The caret: white over a dark part of the picture, black over a light one
// (its own colour, the text's, would vanish on a like background).
let caretProbe = null
function caretFor(e) {
  if (!ed?.img) return ''
  const w = Math.max(e.w * 4, 1),
    h = Math.max(e.w * textLine * Math.max(1, e.value.split('\n').length), 1)
  caretProbe ||= document.createElement('canvas')
  caretProbe.width = 8
  caretProbe.height = 8
  const ctx = caretProbe.getContext('2d', { willReadFrequently: true })
  ctx.clearRect(0, 0, 8, 8)
  ctx.filter = ed.baseVisible ? filterOf(ed.adjust) : 'none'
  if (ed.baseVisible) ctx.drawImage(ed.img, e.x, e.y, w, h, 0, 0, 8, 8)
  else {
    ctx.fillStyle = '#fff'
    ctx.fillRect(0, 0, 8, 8)
  }
  const d = ctx.getImageData(0, 0, 8, 8).data
  let sum = 0
  for (let i = 0; i < d.length; i += 4) sum += 0.299 * d[i] + 0.587 * d[i + 1] + 0.114 * d[i + 2]
  return sum / 64 < 128 ? '#ffffff' : '#000000'
}
function setCaret() {
  const wrap = document.querySelector('.snap-text-box')
  if (wrap && textEdit) wrap.style.setProperty('--caret', caretFor(textEdit) || 'auto')
}
function sizeTextBox(box) {
  if (!textEdit || !ed) return
  const b = textBounds({ text: box.value || ' ', w: textEdit.w, f: textEdit.f, p: [[0, 0]] })
  box.style.width = `calc(${(b.w / ed.note.width) * 100}cqw + 0.5em)`
  box.style.height = `${box.value.split('\n').length * textLine}em`
}

afterRenderHooks.push(() => {
  syncEditor()
  paint()
  const box = document.querySelector('.snap-text-input')
  if (box && textEdit && !box.dataset.focused) {
    box.dataset.focused = '1'
    sizeTextBox(box)
    setCaret()
    box.focus()
    box.setSelectionRange(box.value.length, box.value.length)
  }
})
const repaint = () => {
  if (!ed) return
  ed.version++
  paint()
}

// Zoom: "fit" fills the stage's width (the default, like a page); a number
// is the image's scale. Ctrl+wheel zooms around the pointer, Ctrl+plus and
// Ctrl+minus by steps, Ctrl+0 back to fit; the toolbar has the same.
const zoomLimits = [0.1, 8]
function currentScale(note) {
  if (zoom !== 'fit') return zoom
  const stage = document.querySelector('.snap-stage')
  const width = stage ? stage.clientWidth - 20 : note.width
  return Math.max(0.01, width / note.width)
}
function setZoom(next, anchor) {
  if (!ed) return
  const stage = document.querySelector('.snap-stage'),
    sheet = document.querySelector('.snap-sheet')
  let fx = 0.5,
    fy = 0,
    px = 0,
    py = 0
  if (stage && sheet) {
    const r = sheet.getBoundingClientRect(),
      sr = stage.getBoundingClientRect()
    px = anchor ? anchor.x : sr.left + sr.width / 2
    py = anchor ? anchor.y : sr.top + Math.min(sr.height / 2, r.height / 2)
    fx = (px - r.left) / r.width
    fy = (py - r.top) / r.height
  }
  zoom = Math.min(zoomLimits[1], Math.max(zoomLimits[0], Math.round(next * 1000) / 1000))
  render()
  // The point under the pointer (or the middle) stays where it was.
  const after = document.querySelector('.snap-sheet'),
    st = document.querySelector('.snap-stage')
  if (after && st) {
    const r = after.getBoundingClientRect()
    st.scrollLeft += r.left + fx * r.width - px
    st.scrollTop += r.top + fy * r.height - py
  }
}
const zoomBy = (factor, anchor) => {
  if (ed) setZoom(currentScale(ed.note) * factor, anchor)
}
document.addEventListener(
  'wheel',
  (event) => {
    if (!event.ctrlKey || !ed || !event.target.closest?.('.snap-stage')) return
    event.preventDefault()
    zoomBy(event.deltaY < 0 ? 1.15 : 1 / 1.15, { x: event.clientX, y: event.clientY })
  },
  { passive: false },
)

// Saving: a moment after the last change, with a small picture for the lists
// (the top of the image, drawing included).
let saveTimer = 0
// changedQuietly saves like changed, without drawing the page again now.
function changedQuietly() {
  if (!ed) return
  ed.dirty = true
  clearTimeout(saveTimer)
  const mine = ed
  saveTimer = setTimeout(() => void saveNow(mine), 900)
}
function changed() {
  if (!ed) return
  ed.dirty = true
  clearTimeout(saveTimer)
  const mine = ed
  saveTimer = setTimeout(() => void saveNow(mine), 900)
  render()
}
// The note as it shows, width wide (height cuts it; the thumbnail is its top).
function composite(target, width, height) {
  const s = width / target.note.width
  const c = document.createElement('canvas')
  c.width = width
  c.height = height
  const ctx = c.getContext('2d')
  ctx.fillStyle = '#fff'
  ctx.fillRect(0, 0, width, height)
  if (target.baseVisible) {
    ctx.filter = filterOf(target.adjust)
    ctx.drawImage(target.img, 0, 0, width, target.note.height * s)
    ctx.filter = 'none'
  }
  for (const layer of target.layers) if (layer.visible) for (const st of layer.strokes) drawStroke(ctx, st, s)
  return c
}
async function saveNow(target) {
  if (!target) return
  clearTimeout(saveTimer)
  target.dirty = false
  saving = true
  if (target === ed) render()
  let thumb = ''
  if (target.img) {
    const w = 360,
      h = Math.min(Math.round((w * target.note.height) / target.note.width), Math.round((w * 10) / 16))
    thumb = composite(target, w, h).toDataURL('image/jpeg', 0.82)
  }
  // A failed save keeps the note unsaved: it is saved again on the next
  // change, on going back, or on opening another note.
  try {
    await request('snapSave', { id: target.id, title: target.title, strokes: writeDrawing(target), thumb })
    target.failed = false
  } catch {
    target.dirty = true
    target.failed = true
  } finally {
    saving = false
    if (target === ed) render()
  }
}

// Sharing: the note as it shows, at full size, copied (for pasting in a
// chat or a post), saved as a PNG file, or copied for a post on X. X's post
// page takes no picture from a link, so the picture goes on the clipboard
// and the page opens in the browser, where Ctrl+V adds it.
let shareMessage = '',
  shareTimer = 0
function shareDone(message, ms = 3000) {
  shareMessage = message
  clearTimeout(shareTimer)
  shareTimer = setTimeout(() => {
    shareMessage = ''
    render()
  }, ms)
  render()
}
// A notice over the page for a few seconds (what to do next), closed early
// with its button.
let notice = null,
  noticeTimer = 0
function showNotice(title, body, ms) {
  notice = { title, body, at: Date.now(), ms }
  clearTimeout(noticeTimer)
  noticeTimer = setTimeout(() => {
    notice = null
    render()
  }, ms)
  render()
}
function noticeHTML() {
  if (!notice) return ''
  const opening = Date.now() - notice.at < 2000
  return `<div class="snap-notice" role="status" style="--notice-ms:${notice.ms}ms">${icon('check', 'snap-notice-icon')}<div><strong>${esc(notice.title)}</strong><p>${esc(notice.body)}</p>${opening ? `<small>${esc(t('snapPostXOpening'))}</small>` : ''}</div><button data-action="snapNoticeClose" title="${esc(t('close'))}" aria-label="${esc(t('close'))}">${icon('x')}</button><span class="snap-notice-bar"></span></div>`
}
function openShareMenu(button) {
  const r = button.getBoundingClientRect()
  menu = { share: true }
  render()
  const items = [
    { id: 'share:copy', icon: 'copy', title: t('snapCopyImage'), hint: t('snapCopyImageHint') },
    { id: 'share:save', icon: 'download', title: t('snapSaveAs'), hint: t('snapSaveAsHint') },
    { id: 'share:x', icon: 'send', title: t('snapPostX'), hint: t('snapPostXHint') },
    ...(canShare()
      ? [{ id: 'share:squad', icon: 'squad', title: t('squadShareSnap'), hint: t('squadShareSnapHint') }]
      : []),
  ]
  const left = Math.round(Math.max(4, Math.min(r.left, innerWidth - menuWidth - 4)))
  void request('menuShow', { id: 'snapShare', x: left, y: Math.round(r.bottom + 6), width: menuWidth, items }).catch(
    () => {
      menu = null
      render()
    },
  )
}
async function share(kind) {
  finishText()
  const target = ed
  if (!target?.img) return
  try {
    // To the squad: the note as it shows, at most 1600 pixels on its longer
    // side, as a JPEG (it goes through the squad's room in pieces).
    if (kind === 'squad') {
      const k = Math.min(1, 1600 / Math.max(target.note.width, target.note.height))
      let jpeg = composite(target, Math.round(target.note.width * k), Math.round(target.note.height * k)).toDataURL(
        'image/jpeg',
        0.8,
      )
      if (jpeg.length > 1_400_000)
        jpeg = composite(
          target,
          Math.round(target.note.width * k * 0.7),
          Math.round(target.note.height * k * 0.7),
        ).toDataURL('image/jpeg', 0.7)
      shareDone(t('squadShareSending').replace('{n}', '0'), 60000)
      const ok = await shareSnap(target.title, jpeg, (done) => {
        shareMessage = t('squadShareSending').replace('{n}', String(Math.round(done * 100)))
        render()
      })
      shareDone(t(ok ? 'squadShareDone' : 'snapShareFailed'))
      return
    }
    const image = composite(target, target.note.width, target.note.height).toDataURL('image/png')
    if (kind === 'save') {
      const next = await request('snapExport', { image, name: target.title })
      if (next?.snapExported) shareDone(t('snapExported'))
      return
    }
    await request('snapCopyImage', { image })
    if (kind === 'x') {
      // How to add the picture, shown before the browser comes to the front.
      showNotice(t('snapPostXReady'), t('snapPostXPaste'), 9000)
      const text = postText(target.note, target.title)
      setTimeout(() => void action('snapOpenPost', text), 2000)
    } else shareDone(t('snapCopied'))
  } catch {
    shareDone(t('snapShareFailed'))
  }
}

// Drawing with the pointer: the pen adds a stroke, the eraser removes the
// strokes it touches.
let drawing = null
function imagePoint(event, canvas) {
  const r = canvas.getBoundingClientRect()
  return [
    Math.round(((event.clientX - r.left) / r.width) * ed.note.width * 10) / 10,
    Math.round(((event.clientY - r.top) / r.height) * ed.note.height * 10) / 10,
  ]
}
const snapshotLayers = () => ed.layers.map((l) => l.strokes.slice())
const restoreLayers = (saved) =>
  saved.forEach((strokes, i) => {
    ed.layers[i].strokes = strokes
  })
function remember() {
  ed.undo.push(snapshotLayers())
  if (ed.undo.length > 100) ed.undo.shift()
  ed.redo = []
}
function erase(point) {
  const radius = 10 * Math.max(1, ed.note.width / 1000)
  const layer = ed.layers[ed.active]
  const keep = layer.strokes.filter((s) =>
    isText(s)
      ? !inBounds(point, textBounds(s), radius)
      : !s.p.some(([x, y]) => Math.hypot(x - point[0], y - point[1]) <= radius + s.w / 2),
  )
  if (keep.length === layer.strokes.length) return false
  if (!drawing.erased) {
    remember()
    drawing.erased = true
  }
  layer.strokes = keep
  repaint()
  return true
}
let gripDrag = null
document.addEventListener('pointerdown', (event) => {
  const grip = event.target.closest?.('.snap-text-grip')
  if (grip && textEdit && ed) {
    // Kept from moving the focus: the text box stays in use.
    event.preventDefault()
    grip.setPointerCapture(event.pointerId)
    const canvas = document.querySelector('.snap-ink')
    const r = canvas.getBoundingClientRect()
    gripDrag = {
      pointer: event.pointerId,
      x: event.clientX,
      y: event.clientY,
      origin: [textEdit.x, textEdit.y],
      scale: ed.note.width / r.width,
    }
    return
  }
  if (textEdit && event.target.closest && !event.target.closest('.snap-text-box,.snap-ink,' + paletteControl))
    finishText()
})
document.addEventListener('pointermove', (event) => {
  if (gripDrag && event.pointerId === gripDrag.pointer && textEdit && ed) {
    const x = gripDrag.origin[0] + (event.clientX - gripDrag.x) * gripDrag.scale,
      y = gripDrag.origin[1] + (event.clientY - gripDrag.y) * gripDrag.scale
    textEdit.x = Math.round(Math.min(ed.note.width - 1, Math.max(0, x)) * 10) / 10
    textEdit.y = Math.round(Math.min(ed.note.height - 1, Math.max(0, y)) * 10) / 10
    const wrap = document.querySelector('.snap-text-box')
    if (wrap) {
      wrap.style.left = (textEdit.x / ed.note.width) * 100 + '%'
      wrap.style.top = (textEdit.y / ed.note.height) * 100 + '%'
    }
    return
  }
  if (drawing || tool !== 'text' || !ed || textEdit) return
  const canvas = event.target.closest?.('.snap-ink')
  if (canvas) canvas.style.cursor = textAt(imagePoint(event, canvas)) ? 'move' : ''
})
const endGrip = (event) => {
  if (!gripDrag || event.pointerId !== gripDrag.pointer) return
  gripDrag = null
  setCaret()
  document.querySelector('.snap-text-input')?.focus()
}
document.addEventListener('pointerup', endGrip)
document.addEventListener('pointercancel', endGrip)
document.addEventListener('pointerdown', (event) => {
  const canvas = event.target.closest?.('.snap-ink')
  if (!canvas || !ed || !ed.editing || event.button !== 0) return
  event.preventDefault()
  if (textEdit) {
    finishText()
    return
  }
  canvas.setPointerCapture(event.pointerId)
  const point = imagePoint(event, canvas)
  if (tool === 'text') {
    const found = textAt(point)
    if (found) drawing = { pointer: event.pointerId, canvas, text: found, start: point, moved: false }
    else editText(point, null)
    return
  }
  if (!ed.layers[ed.active].visible) {
    ed.layers[ed.active].visible = true
    repaint()
    changed()
  }
  if (tool === 'eraser') {
    drawing = { pointer: event.pointerId, canvas, erase: true, erased: false }
    erase(point)
    return
  }
  drawing = { pointer: event.pointerId, canvas, stroke: { c: color, w: lineWidth(ed.note), p: [point] } }
  drawStroke(canvas.getContext('2d'), drawing.stroke, canvas.width / ed.note.width)
})
document.addEventListener('pointermove', (event) => {
  if (!drawing || event.pointerId !== drawing.pointer || !ed) return
  const point = imagePoint(event, drawing.canvas)
  if (drawing.text) {
    const d = drawing,
      dx = point[0] - d.start[0],
      dy = point[1] - d.start[1]
    if (!d.moved) {
      if (Math.hypot(dx, dy) < 4 * Math.max(1, ed.note.width / 1000)) return
      d.moved = true
      remember()
      const strokes = ed.layers[d.text.layer].strokes
      d.origin = d.text.item.p[0]
      d.item = { ...d.text.item }
      strokes[strokes.indexOf(d.text.item)] = d.item
    }
    d.item.p = [[Math.round((d.origin[0] + dx) * 10) / 10, Math.round((d.origin[1] + dy) * 10) / 10]]
    repaint()
    return
  }
  if (drawing.erase) {
    erase(point)
    return
  }
  const p = drawing.stroke.p,
    last = p[p.length - 1]
  if (Math.hypot(point[0] - last[0], point[1] - last[1]) < 1.5) return
  p.push(point)
  const ctx = drawing.canvas.getContext('2d'),
    scale = drawing.canvas.width / ed.note.width
  drawStroke(ctx, { c: drawing.stroke.c, w: drawing.stroke.w, p: [last, point] }, scale)
})
const endDraw = (event) => {
  if (!drawing || event.pointerId !== drawing.pointer) return
  const done = drawing
  drawing = null
  if (!ed) return
  if (done.text) {
    if (done.moved) changed()
    else editText(null, done.text)
    return
  }
  if (done.erase) {
    if (done.erased) changed()
    return
  }
  remember()
  ed.layers[ed.active].strokes.push(done.stroke)
  ed.version++
  done.canvas.dataset.v = ed.id + '/' + ed.version
  changed()
}
document.addEventListener('pointerup', endDraw)
document.addEventListener('pointercancel', endDraw)

function undo() {
  if (!ed?.undo.length) return
  ed.redo.push(snapshotLayers())
  restoreLayers(ed.undo.pop())
  repaint()
  changed()
}
function redo() {
  if (!ed?.redo.length) return
  ed.undo.push(snapshotLayers())
  restoreLayers(ed.redo.pop())
  repaint()
  changed()
}
document.addEventListener('keydown', (event) => {
  if (!event.target.classList?.contains('snap-text-input')) return
  if (event.key === 'Escape' || (event.key === 'Enter' && event.ctrlKey)) {
    event.preventDefault()
    finishText()
  }
})
// Leaving the box finishes the text, but for a colour or a size chosen for it.
document.addEventListener('focusout', (event) => {
  if (!textEdit || !event.target.classList?.contains('snap-text-input')) return
  if (event.relatedTarget?.closest?.(paletteControl + ',.snap-text-box')) return
  textEdit.value = event.target.value
  finishText()
})
document.addEventListener('keydown', (event) => {
  if (
    !ed ||
    !ed.editing ||
    state.tabs.find((t) => t.id === state.active)?.kind !== 'snapnotes' ||
    event.target.closest?.('input,textarea')
  )
    return
  const key = event.key.toLowerCase()
  if (!event.ctrlKey && !event.altKey && ['1', '2', '3'].includes(event.key)) {
    ed.active = Number(event.key) - 1
    render()
    return
  }
  if (event.ctrlKey && !event.altKey && (key === 'z' || key === 'y')) {
    event.preventDefault()
    if (key === 'y' || event.shiftKey) redo()
    else undo()
  }
  if (event.ctrlKey && !event.altKey && ['+', ';', '=', '-', '0'].includes(event.key)) {
    event.preventDefault()
    if (event.key === '0') {
      zoom = 'fit'
      render()
    } else zoomBy(event.key === '-' ? 1 / 1.25 : 1.25)
  }
})
document.addEventListener('change', (event) => {
  const data = event.target.dataset
  if (data?.spotMap && event.target.value) {
    void action('snapSetMap', { id: data.spotMap, map: event.target.value })
    return
  }
  if (ed && data && 'textFont' in data) {
    textStyle.font = event.target.value
    if (textEdit) {
      textEdit.f = textStyle.font
      restyleTextBox()
    }
    render()
    return
  }
  if (ed && data && ('textStyle' in data || 'colorInput' in data || 'outlineInput' in data)) {
    if ('colorInput' in data && tool === 'eraser') tool = 'pen'
    render()
  }
})
document.addEventListener('input', (event) => {
  if (textEdit && event.target.classList?.contains('snap-text-input')) {
    textEdit.value = event.target.value
    sizeTextBox(event.target)
    return
  }
  const style = event.target.dataset?.textStyle
  if (style && ed) {
    const v = Number(event.target.value)
    const out = document.querySelector(`[data-text-out="${style}"]`)
    if (style === 'size') {
      textStyle.size = v
      if (out) out.textContent = String(v)
      if (textEdit) textEdit.w = textSize(ed.note)
    } else {
      textStyle.width = v
      if (out) out.textContent = v + '%'
      if (textEdit) textEdit.ow = v
    }
    restyleTextBox(false)
    return
  }
  if (event.target.dataset && 'colorInput' in event.target.dataset && ed) {
    color = event.target.value
    if (textEdit) textEdit.c = color
    restyleTextBox(false)
    return
  }
  if (event.target.dataset && 'outlineInput' in event.target.dataset && ed) {
    textStyle.outline = event.target.value
    if (textEdit) textEdit.o = textStyle.outline
    restyleTextBox(false)
    return
  }
  if (event.target.id === 'snap-title' && ed) {
    ed.title = event.target.value
    changed()
    return
  }
  const key = event.target.dataset?.adjust
  if (key && ed && adjustRanges[key]) {
    ed.adjust[key] = Number(event.target.value)
    // Straight onto the page, so the slider moves smoothly; the render and
    // the save follow.
    document.querySelector('.snap-sheet')?.style.setProperty('--snap-filter', filterOf(ed.adjust))
    const e = String(gammaOf(ed.adjust.shadows))
    document
      .querySelectorAll('#snap-shadows feFuncR,#snap-shadows feFuncG,#snap-shadows feFuncB')
      .forEach((fn) => fn.setAttribute('exponent', e))
    const out = document.querySelector(`[data-adjust-out="${key}"]`)
    if (out) out.textContent = adjustPercent(ed.adjust[key])
    changedQuietly()
  }
})

// New notes from a picture: pasted, or an image file. Any image the browser
// reads is turned into PNG.
async function imageToPNG(blob) {
  const bitmap = await createImageBitmap(blob)
  const c = document.createElement('canvas')
  c.width = bitmap.width
  c.height = bitmap.height
  c.getContext('2d').drawImage(bitmap, 0, 0)
  bitmap.close?.()
  return c.toDataURL('image/png')
}
async function newFromImage(blob, title) {
  try {
    await action('snapNew', { image: await imageToPNG(blob), title })
  } catch {}
}
function blankSheet() {
  const c = document.createElement('canvas')
  c.width = 1600
  c.height = 1000
  const ctx = c.getContext('2d')
  ctx.fillStyle = '#ffffff'
  ctx.fillRect(0, 0, c.width, c.height)
  return c.toDataURL('image/png')
}
document.addEventListener('paste', (event) => {
  if (state?.tabs.find((t) => t.id === state.active)?.kind !== 'snapnotes' || event.target.closest?.('input,textarea'))
    return
  const file = [...(event.clipboardData?.items || [])]
    .find((i) => i.kind === 'file' && i.type.startsWith('image/'))
    ?.getAsFile()
  if (file) {
    event.preventDefault()
    void newFromImage(file, '')
  }
})
document.addEventListener('change', (event) => {
  if (event.target.id !== 'snap-file') return
  const file = event.target.files?.[0]
  event.target.value = ''
  if (file && file.type.startsWith('image/')) void newFromImage(file, file.name.replace(/\.[^.]+$/, ''))
})

clickHandlers.push(async (type, id, button) => {
  if (type === 'snapMenu') {
    if (!menu) openMenu(button)
    return true
  }
  if (type === 'toggleSnapSection') {
    void action('preferences', { snapNotesCollapsed: !state.snapNotesCollapsed })
    return true
  }
  if (type === 'snapNewBlank') {
    void action('snapNew', { image: blankSheet(), title: '' })
    return true
  }
  if (type === 'snapNewFile') {
    document.querySelector('#snap-file')?.click()
    return true
  }
  if (type === 'snapTool') {
    finishText()
    tool = id === 'eraser' || id === 'text' ? id : 'pen'
    render()
    if (tool === 'text' && !state.snapNotes?.fonts) void action('snapFonts')
    return true
  }
  if (type === 'snapColor') {
    color = colors.includes(id) ? id : colors[0]
    if (tool === 'eraser') tool = 'pen'
    if (textEdit) {
      textEdit.c = color
      restyleTextBox()
    }
    render()
    return true
  }
  if (type === 'snapSize') {
    size = sizes.some(([k]) => k === id) ? id : 'm'
    if (tool === 'eraser') tool = 'pen'
    render()
    return true
  }
  if (type === 'snapOutline') {
    textStyle.outline = id === 'auto' || colors.includes(id) ? id : 'auto'
    if (textEdit) {
      textEdit.o = textStyle.outline
      restyleTextBox()
    }
    render()
    return true
  }
  if (type === 'snapUndo') {
    undo()
    return true
  }
  if (type === 'snapRedo') {
    redo()
    return true
  }
  if (type === 'snapClear') {
    const layer = ed?.layers[ed.active]
    if (layer?.strokes.length) {
      remember()
      layer.strokes = []
      repaint()
      changed()
    }
    return true
  }
  if (type === 'snapLayer') {
    if (ed) {
      ed.active = Math.max(0, Math.min(layerCount - 1, Number(id) || 0))
      render()
    }
    return true
  }
  if (type === 'snapLayerEye') {
    const layer = ed?.layers[Number(id)]
    if (layer) {
      layer.visible = !layer.visible
      repaint()
      changed()
    }
    return true
  }
  if (type === 'snapLayerClear') {
    const layer = ed?.layers[Number(id)]
    if (layer?.strokes.length) {
      remember()
      layer.strokes = []
      repaint()
      changed()
    }
    return true
  }
  if (type === 'snapAdjustReset') {
    if (ed) {
      ed.adjust = defaultAdjust()
      changed()
    }
    return true
  }
  if (type === 'snapBaseEye') {
    if (ed) {
      ed.baseVisible = !ed.baseVisible
      changed()
    }
    return true
  }
  if (type === 'snapZoomIn') {
    zoomBy(1.25)
    return true
  }
  if (type === 'snapZoomOut') {
    zoomBy(1 / 1.25)
    return true
  }
  if (type === 'snapZoomReset') {
    setZoom(1)
    return true
  }
  if (type === 'snapZoomFit') {
    zoom = 'fit'
    render()
    return true
  }
  if (type === 'snapFavorite') {
    const note =
      state.snapNotes?.open?.note.id === id ? state.snapNotes.open.note : state.snapNotes?.list.find((n) => n.id === id)
    if (note) void action('snapFavorite', { id, favorite: !note.favorite })
    return true
  }
  if (type === 'snapShowSpot') {
    try {
      await request('snapShowSpot', id)
      shareDone(t('snapSpotShown'))
    } catch {}
    return true
  }
  if (type === 'snapNoticeClose') {
    notice = null
    clearTimeout(noticeTimer)
    render()
    return true
  }
  if (type === 'snapShare') {
    if (!menu) openShareMenu(button)
    return true
  }
  // Drawing on the note shown, and back to looking at it.
  if (type === 'snapEdit' || type === 'snapEditDone') {
    if (!ed) return true
    if (type === 'snapEditDone' && textEdit) finishText()
    ed.editing = type === 'snapEdit'
    render()
    return true
  }
  if (type === 'snapBack') {
    if (ed?.dirty) await saveNow(ed)
    void action('snapClose')
    return true
  }
  if (type === 'snapLinkToggle') {
    if (ed) void action('snapLink', { id: ed.id, linked: !ed.note.linked })
    return true
  }
  if (type === 'snapDeleteNote') {
    if (armedDelete !== id) {
      armedDelete = id
      render()
      setTimeout(() => {
        if (armedDelete === id) {
          armedDelete = ''
          render()
        }
      }, 4000)
      return true
    }
    // A save still waiting for this note would find it gone.
    armedDelete = ''
    if (ed?.id === id) {
      ed.dirty = false
      clearTimeout(saveTimer)
    }
    void action('snapDelete', id)
    return true
  }
  return false
})
