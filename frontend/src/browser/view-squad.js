import { age } from './item.js'
import { findMap, freshness, players, squadColors, squadKey } from './map-geo.js'
import { colorOf, squadColorMap } from './squad-colors.js'
import { codeBoxes, onCodeBoxes } from './code-boxes.js'
import { shares, markSeen, waiting } from './squad-share.js'
import {
  state,
  esc,
  t,
  icon,
  action,
  clickHandlers,
  render,
  afterRenderHooks,
  secretText,
  revealButton,
  maskedClass,
} from './shell-core.js'

// The squad (app_squad.go, docs/squad-sharing.md): its section in the
// sidebar (who is in it, where), its page (the code, the name, the squad
// colour, the squads joined lately, leaving) and the list of members the
// map's squad panel shows too. The map draws the members (view-map.js).

// What is typed in the squad's forms, kept across renders.
const drafts = { code: '', name: null, notice: '' }
// The name this PC shows: the one given for the squad, else the player's
// own (TarkovTracker's display name, from the Host).
export const ownName = () => state.squadName || state.host?.player || ''

export const mapName = (key) =>
  state.bosses?.maps?.find((m) => m.key === key)?.name ||
  String(key || '')
    .split('-')
    .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
    .join(' ')

// you tells whether a member is this PC's player: this PC, or, on a PC that
// only watches (a Client), its Host (the same display name, which the Host
// and its Clients keep the same).
function you(m) {
  if (m.me) return true
  const me = state.squad?.state?.members?.find((x) => x.me)
  return !!(me?.viewer && m.name && m.name === (state.squadName || me.name))
}

// whereOf is where a member is, as text: their map and how long ago, or
// out of a raid.
function whereOf(m) {
  const maps = state.squad?.maps || []
  return m.map
    ? m.pos
      ? `${mapName(findMap(maps, m.map)?.key || m.map)} · ${age(m.at, state.language)}`
      : mapName(m.map)
    : t('squadOutOfRaid')
}
function memberLine(m) {
  const where = esc(whereOf(m))
  const fade = m.map && m.pos ? freshness(m.at) : 'gone'
  const line = `<span class="squad-dot" style="--c:${colorOf(m)}"></span><span class="squad-member-name">${esc(m.name || '?')}${you(m) ? ` <small>(${esc(t('squadYou'))})</small>` : ''}</span><span class="squad-member-where">${where}</span>`
  // A member with a position is a button that shows them on the map (on
  // their map and floor; view-map.js squadFocus).
  return m.map && m.pos
    ? `<li class="squad-member ${fade}"><button class="squad-member-focus" data-action="squadFocus" data-id="${esc(squadKey(m))}" title="${esc(t('squadFocus'))}">${line}</button></li>`
    : `<li class="squad-member ${fade}">${line}</li>`
}
// The sidebar's members: a colour dot and a name each (pressed, it shows
// them on the map as their place does). Under it, always open, a small
// tree of the latest of each: where they are (pressed, it shows them on the
// map: their map and floor, at the zoom the map has, with rings in their
// colour: view-map.js squadFocus), the page, the snap note and the pin they
// shared (pressed, each opens as on the squad's page, and stays). The count
// of what they shared unread opens the squad's page on their shares alone.
function memberChip(m) {
  const fade = m.map && m.pos ? freshness(m.at) : 'gone'
  const title = `${m.name || '?'} — ${whereOf(m)}`
  const theirs = you(m) || !m.key ? [] : shares.filter((x) => x.by === m.key)
  const unread = theirs.filter((x) => !x.seen).length
  const line = `<span class="squad-dot" style="--c:${colorOf(m)}"></span><span class="squad-chip-name">${esc(m.name || '?')}${you(m) ? ` <small>(${esc(t('squadYou'))})</small>` : ''}</span>`
  const head =
    m.map && m.pos
      ? `<button class="squad-chip-focus" data-action="squadFocus" data-id="${esc(squadKey(m))}" title="${esc(t('squadFocus'))}\n${esc(whereOf(m))}">${line}</button>`
      : `<span class="squad-chip-focus" title="${esc(title)}">${line}</span>`
  const badge = unread
    ? `<button class="squad-chip-count" data-action="squadMemberShares" data-id="${esc(m.key)}" title="${esc(t('squadMemberUnread'))}">${unread}</button>`
    : ''
  // The latest page, snap note and pin they shared, in that order.
  const latest = ['tab', 'snap', 'view'].map((kind) => theirs.find((x) => x.kind === kind)).filter(Boolean)
  // Each with how many more of its kind they shared: pressed, the squad's
  // page shows their shares of that kind.
  const more = (x) => {
    const n = theirs.filter((y) => y.kind === x.kind).length - 1
    return n > 0
      ? `<button class="squad-chip-more" data-action="squadMemberShares" data-id="${esc(`${m.key}|${x.kind}`)}" title="${esc(t('squadMemberMore'))}">+${n}</button>`
      : ''
  }
  const row = (x) =>
    `<div class="squad-chip-item"><button class="squad-chip-latest ${x.seen ? '' : 'unseen'}" data-action="squadShareOpen" data-id="${esc(x.id)}" title="${esc(
      x.kind === 'tab'
        ? `${shareTitle(x)}
${x.url}`
        : shareTitle(x),
    )}">${icon(shareIcons[x.kind] || 'globe')}<span>${esc(shareTitle(x))}</span><small>${esc(age(new Date(x.at).toISOString(), state.language))}</small></button>${more(x)}</div>`
  // Where they are: their map and how long ago (shows them), else out of
  // a raid or on a map without a place yet.
  const place =
    m.map && m.pos
      ? `<div class="squad-chip-item"><button class="squad-chip-latest" data-action="squadFocus" data-id="${esc(squadKey(m))}" title="${esc(t('squadFocus'))}">${icon('map')}<span>${esc(mapName(findMap(state.squad?.maps || [], m.map)?.key || m.map))}</span><small>${esc(age(m.at, state.language))}</small></button></div>`
      : `<span class="squad-chip-where">${icon('map')}<span>${esc(whereOf(m))}</span></span>`
  const under = `<div class="squad-chip-items">${place}${latest.map(row).join('')}</div>`
  return `<li class="squad-chip ${fade}"><div class="squad-chip-row">${head}${badge}</div>${under}</li>`
}

export { colorOf }

// The members, the PCs that only watch left out.
export const memberList = (members, cls = '') =>
  `<ul class="squad-members ${cls}">${players(members).map(memberLine).join('')}</ul>`

const activeKind = () => state.tabs.find((tab) => tab.id === state.active)?.kind

// A shared thing's row: its kind's icon, its title (or who drew where), who
// shared it, when, and a dot while unread. Pressing it opens it.
const shareIcons = { tab: 'globe', snap: 'snap', draw: 'pencil', view: 'mapPin' }
function shareTitle(s) {
  if (s.kind === 'draw') return t('squadDrew').replace('{map}', mapName(s.map))
  if (s.kind === 'view') return t('squadLookHere').replace('{map}', mapName(s.map))
  return s.title || s.url || t('snapNotes')
}
function shareRow(s, full) {
  const from = `<span class="squad-share-from"><span class="squad-dot" style="--c:${esc(s.c)}"></span>${esc(s.mine ? t('squadYou') : s.name || '?')}${full ? ` · ${esc(age(new Date(s.at).toISOString(), state.language))}` : ''}</span>`
  const title = s.kind === 'tab' ? `${shareTitle(s)}\n${s.url}` : shareTitle(s)
  return `<li class="squad-share ${s.seen ? '' : 'unseen'}"><button data-action="squadShareOpen" data-id="${esc(s.id)}" title="${esc(title)}">${icon(shareIcons[s.kind] || 'globe')}<span class="squad-share-text"><span class="squad-share-title">${esc(shareTitle(s))}</span>${from}</span>${s.seen ? '' : '<span class="squad-share-dot"></span>'}</button></li>`
}
let shareFilter = 'all'
// The member whose shares alone the list shows (member key), or ''.
let shareBy = ''
function sharesCard() {
  const chip = (id, label) =>
    `<button data-action="squadShareFilter" data-id="${id}" class="${shareFilter === id ? 'selected' : ''}">${esc(t(label))}</button>`
  const list = shares.filter((s) => (shareFilter === 'all' || s.kind === shareFilter) && (!shareBy || s.by === shareBy))
  const by = shareBy && shares.find((s) => s.by === shareBy)
  const who = by
    ? `<button class="squad-shares-by" data-action="squadSharesAllMembers" title="${esc(t('squadSharesEveryone'))}"><span class="squad-dot" style="--c:${esc(by.c)}"></span>${esc(by.name || '?')}${icon('x')}</button>`
    : ''
  return `<section class="panel squad-shares-card"><div class="squad-shares-head"><h2>${esc(t('squadShares'))}</h2>${who}<div class="segmented" role="group">${chip('all', 'squadSharesAll')}${chip('tab', 'squadSharesTabs')}${chip('snap', 'snapNotes')}${chip('view', 'squadSharesView')}${chip('draw', 'squadSharesDraw')}</div></div><p class="hint">${esc(t('squadSharesHint'))}</p>${list.length ? `<ul class="squad-shares">${list.map((s) => shareRow(s, true)).join('')}</ul>` : `<p class="shot-empty">${esc(t('squadSharesEmpty'))}</p>`}</section>`
}

// The sidebar's section: the heading (with how many are in the squad and
// whether it is connected) folds it; the button at its end opens the page.
export function squadSection() {
  if (!state.squad) return ''
  const s = state.squad.state
  const open = activeKind() === 'squad'
  const folded = state.squadCollapsed
  const count = s
    ? `<span class="squad-section-count" data-phase="${esc(s.phase)}" title="${esc(t('squadPhase_' + s.phase))}">${players(s.members).length}</span>`
    : ''
  const head = `<div class="section-label squad-section-label ${open ? 'active' : ''}"><button class="section-link" data-action="toggleSquadSection" aria-expanded="${!folded}" title="${esc(t(folded ? 'expandSection' : 'collapseSection'))}">${esc(t('squad'))}${count}${icon('chevron', 'section-chevron')}</button><button class="new-tab squad-open" data-action="squadPage" title="${esc(t('squadOpen'))}" aria-label="${esc(t('squadOpen'))}" aria-pressed="${open}">${icon('squad')}${waiting().length ? `<span class="squad-open-count">${waiting().length}</span>` : ''}</button></div>`
  if (folded) return head
  const body = s
    ? `<ul class="squad-chips squad-section">${players(s.members).map(memberChip).join('')}</ul>`
    : `<div class="squad-section"><button class="squad-start" data-action="squadPage">${esc(t('squadStart'))}</button></div>`
  return head + body
}

// The squads joined lately, to join again with a click.
function recentSquads() {
  const list = (state.squadRecent || []).map(
    (r) =>
      `<li><button class="squad-recent-join" data-action="squadRejoin" data-id="${esc(r.code)}" title="${esc(t('squadJoin'))}"><span class="squad-recent-code">${esc(secretText('squadRecent', r.code))}</span><span class="squad-recent-when">${esc(age(new Date(r.at).toISOString(), state.language))}</span></button></li>`,
  )
  return list.length
    ? `<div class="squad-recent"><h3>${esc(t('squadRecent'))}${revealButton('squadRecent')}</h3><ul>${list.join('')}</ul><p class="hint">${esc(t('squadRecentHint'))}</p></div>`
    : ''
}

// The squad colours to choose from: automatic (the squad gives one), or one
// no one else in the squad has.
function colorPicker() {
  const s = state.squad.state
  const given = squadColorMap()
  const me = s?.members.find((m) => m.me)
  // A PC that only watches is its Host's player: the Host's colour is its own.
  const holders = new Map()
  if (s && !me?.viewer)
    for (const m of players(s.members)) if (!m.me) holders.set(given.get(squadKey(m)), m.name || '?')
  const chosen = state.squadColor
  const swatch = (c) => {
    const holder = holders.get(c)
    const label = holder ? t('squadColorTaken').replace('{name}', holder) : c
    return `<button type="button" class="squad-color ${chosen === c ? 'on' : ''}" data-action="squadColorPick" data-id="${c}" style="--swatch:${c}" title="${esc(label)}" aria-label="${esc(label)}" aria-pressed="${chosen === c}" ${holder && chosen !== c ? 'disabled' : ''}></button>`
  }
  const busy = s && chosen && !me?.viewer && given.get('me') && given.get('me') !== chosen
  return `<div class="squad-colors"><span class="squad-colors-label">${esc(t('squadColor'))}</span><div class="squad-color-row"><button type="button" class="squad-color-auto ${chosen ? '' : 'on'}" data-action="squadColorPick" data-id="" aria-pressed="${!chosen}">${esc(t('squadColorAuto'))}</button>${squadColors.map(swatch).join('')}${custom(chosen && !squadColors.includes(chosen) ? chosen : '', holders)}</div><p class="hint">${esc(t(busy ? 'squadColorBusy' : 'squadColorHelp'))}</p></div>`
}

// custom is the last swatch: any colour, from the system's colour picker
// (its colour once chosen; a colour someone else has is taken by them).
function custom(chosen, holders) {
  const holder = chosen && holders.get(chosen)
  const label = holder ? t('squadColorTaken').replace('{name}', holder) : t('squadColorCustom')
  return `<label class="squad-color squad-color-custom ${chosen ? 'on' : ''}" style="--swatch:${chosen || 'transparent'}" title="${esc(label)}">${chosen ? '' : icon('plus')}<input type="color" data-squad-color value="${esc(chosen || '#ffffff')}" aria-label="${esc(t('squadColorCustom'))}"></label>`
}
// A colour chosen in the picker is taken when it closes.
document.addEventListener('change', (event) => {
  const el = /** @type {HTMLInputElement} */ (event.target)
  if (el?.dataset?.squadColor !== undefined && el.matches?.('input[type=color]')) void action('squadColor', el.value)
})

// The squad's page: your profile (the name and colour the squad sees, kept
// whether in a squad or not), then the squad: creating or joining one, or
// the one joined (its code, members, leaving).
export function squadPage() {
  if (!state.squad) return `<div class="page"><p class="empty-tabs">${esc(t('squadUnavailable'))}</p></div>`
  const s = state.squad.state
  const name = drafts.name ?? ownName()
  const notice =
    drafts.notice && drafts.notice !== 'squadCopied'
      ? `<p class="squad-notice" role="alert">${esc(t(drafts.notice))}</p>`
      : ''
  const head = `<div class="bookmarks-head"><h1>${esc(t('squad'))}</h1></div>`
  const profile = `<section class="panel squad-forms squad-profile"><h2>${esc(t('squadProfile'))}</h2><p class="hint">${esc(t('squadProfileHelp'))}</p><form id="squad-name-form" class="squad-name"><label class="field"><span>${esc(t('squadName'))}</span><input name="squadName" maxlength="24" autocomplete="off" spellcheck="false" placeholder="${esc(t('squadNamePlaceholder'))}" value="${esc(name)}"></label></form>${colorPicker()}</section>`
  const privacy = `<p class="hint">${esc(t('squadPrivacy'))}</p>`
  if (!s)
    return `<div class="page squad-page">${head}${profile}<section class="panel squad-forms"><h2>${esc(t('squadJoinTitle'))}</h2><p class="hint">${esc(t('squadIntro'))}</p><form id="squad-form" class="squad-form"><button class="primary" type="submit" value="create">${esc(t('squadCreate'))}</button><div class="squad-join"><span class="secret-field code-boxes-field">${codeBoxes('squad', drafts.code, { cls: maskedClass('squadInput'), label: t('squadCode') })}${revealButton('squadInput')}</span><button type="submit" value="join">${esc(t('squadJoin'))}</button></div>${notice}</form>${recentSquads()}${privacy}</section></div>`
  const viewer = s.members.find((m) => m.me)?.viewer ? `<p class="hint">${esc(t('squadViewer'))}</p>` : ''
  const code = `<div class="squad-code"><output>${esc(secretText('squad', state.squadCode || s.code))}</output>${revealButton('squad')}<button data-action="squadCopy" title="${esc(t('squadCopy'))}">${icon('copy')}<span>${esc(t(drafts.notice === 'squadCopied' ? 'squadCopied' : 'squadCopy'))}</span></button></div><p class="squad-phase" data-phase="${esc(s.phase)}">${esc(t('squadPhase_' + s.phase))}</p>`
  return `<div class="page squad-page">${head}${profile}<section class="panel squad-forms"><h2>${esc(t('squadJoined'))}</h2><h3>${esc(t('squadCode'))}</h3>${code}<h3>${esc(t('squadMembers'))}</h3>${memberList(s.members)}${viewer}${notice}<button class="squad-leave" data-action="squadLeave">${esc(t('squadLeave'))}</button>${privacy}</section>${sharesCard()}</div>`
}

const inForms = (el) => !!el.closest?.('.squad-forms')
document.addEventListener('input', (event) => {
  const el = /** @type {HTMLInputElement} */ (event.target)
  if (!inForms(el)) return
  if (el.name === 'squadName') drafts.name = el.value
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
  const name = String(drafts.name ?? ownName()).trim()
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
// The squad code: eight boxes of a letter or digit each (code-boxes.js), the
// hyphen never typed; the eighth joins.
onCodeBoxes('squad', {
  allow: /[0-9A-Z]/,
  onChange: (code) => (drafts.code = code),
  onComplete: (code) => join(code),
})
function join(code) {
  const name = String(drafts.name ?? ownName()).trim()
  drafts.notice = !name ? 'squadNeedName' : code.replace(/[\s-]/g, '').length !== 8 ? 'squadInvalidCode' : ''
  if (drafts.notice) {
    render()
    return
  }
  void action('squadJoin', { code, name }).then((next) => {
    if (next?.squad?.state) Object.assign(drafts, { code: '', name: null, notice: '' })
  })
}
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

// openSharedSnap keeps a shared snap note as a note of this PC's the first
// time, and opens that note after (a new one if it was deleted since).
async function openSharedSnap(s) {
  if (s.noteId) {
    try {
      const next = await action('snapOpen', s.noteId)
      if (next?.snapNotes?.open?.note?.id === s.noteId) return
    } catch {}
  }
  const next = await action('snapNew', { image: s.image, title: s.title })
  s.noteId = next?.snapNotes?.open?.note?.id || ''
}

// Seen on the squad's page, what was shared is read.
afterRenderHooks.push(() => {
  if (activeKind() === 'squad' && shares.some((s) => !s.seen)) setTimeout(() => markSeen(), 1500)
})

clickHandlers.push(async (type, id) => {
  if (type === 'squadShareFilter') {
    shareFilter = id || 'all'
    render()
    return true
  }
  // A member's count: the squad's page on their shares alone.
  // A member's count (all they shared) or a row's more (that kind alone).
  if (type === 'squadMemberShares') {
    const [key, kind] = String(id || '').split('|')
    shareBy = key || ''
    shareFilter = kind || 'all'
    void action('squadPage')
    return true
  }
  if (type === 'squadSharesAllMembers') {
    shareBy = ''
    render()
    return true
  }
  if (type === 'squadShareOpen') {
    const s = shares.find((x) => x.id === id)
    if (!s) return true
    markSeen(s.id)
    // A page already open in a tab comes forward rather than again; a snap
    // note taken in once opens that note again.
    if (s.kind === 'tab') void action('openOrFocus', s.url)
    else if (s.kind === 'snap') void openSharedSnap(s)
    else if (s.kind === 'view')
      window.dispatchEvent(
        new CustomEvent('mayak:map-show', {
          detail: { map: s.map, floor: s.floor, x: s.x, z: s.z, zoom: s.zoom, c: s.c },
        }),
      )
    else window.dispatchEvent(new CustomEvent('mayak:map-show', { detail: { map: s.map } }))
    return true
  }
  if (type === 'toggleSquadSection') {
    void action('preferences', { squadCollapsed: !state.squadCollapsed })
    return true
  }
  if (type === 'squadColorPick') {
    void action('squadColor', id || '')
    return true
  }
  if (type === 'squadRejoin') {
    const name = String(drafts.name ?? ownName()).trim()
    drafts.notice = name ? '' : 'squadNeedName'
    if (!name) render()
    else
      void action('squadJoin', { code: id, name }).then((next) => {
        if (next?.squad?.state) Object.assign(drafts, { code: '', name: null, notice: '' })
      })
    return true
  }
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
    void action('squadLeave')
    return true
  }
  return false
})
