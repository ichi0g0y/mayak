import { age } from './item.js'
import { findMap, freshness, players, squadColors, squadKey } from './map-geo.js'
import { colorOf, squadColorMap } from './squad-colors.js'
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
  const line = `<span class="squad-dot" style="--c:${colorOf(m)}"></span><span class="squad-member-name">${esc(m.name || '?')}${m.me ? ` <small>(${esc(t('squadYou'))})</small>` : ''}</span><span class="squad-member-where">${where}</span>`
  // A member with a position is a button that shows them on the map (on
  // their map and floor; view-map.js squadFocus).
  return m.map && m.pos
    ? `<li class="squad-member ${fade}"><button class="squad-member-focus" data-action="squadFocus" data-id="${esc(squadKey(m))}" title="${esc(t('squadFocus'))}">${line}</button></li>`
    : `<li class="squad-member ${fade}">${line}</li>`
}
// The sidebar's members: a colour dot and a name each, where they are in
// the title; one with a position shows them on the map when pressed.
function memberChip(m) {
  const fade = m.map && m.pos ? freshness(m.at) : 'gone'
  const title = `${m.name || '?'} — ${whereOf(m)}`
  const line = `<span class="squad-dot" style="--c:${colorOf(m)}"></span><span class="squad-chip-name">${esc(m.name || '?')}${m.me ? ` <small>(${esc(t('squadYou'))})</small>` : ''}</span>`
  return m.map && m.pos
    ? `<li class="squad-chip ${fade}"><button data-action="squadFocus" data-id="${esc(squadKey(m))}" title="${esc(title)}">${line}</button></li>`
    : `<li class="squad-chip ${fade}" title="${esc(title)}">${line}</li>`
}

export { colorOf }

// The members, the PCs that only watch left out.
export const memberList = (members, cls = '') =>
  `<ul class="squad-members ${cls}">${players(members).map(memberLine).join('')}</ul>`

const activeKind = () => state.tabs.find((tab) => tab.id === state.active)?.kind

// A shared thing's row: its kind's icon, its title (or who drew where), who
// shared it, when, and a dot while unread. Pressing it opens it.
const shareIcons = { tab: 'globe', snap: 'snap', draw: 'pencil' }
function shareTitle(s) {
  return s.kind === 'draw' ? t('squadDrew').replace('{map}', mapName(s.map)) : s.title || s.url || t('snapNotes')
}
function shareRow(s, full) {
  const from = `<span class="squad-share-from"><span class="squad-dot" style="--c:${esc(s.c)}"></span>${esc(s.mine ? t('squadYou') : s.name || '?')}${full ? ` · ${esc(age(new Date(s.at).toISOString(), state.language))}` : ''}</span>`
  const title = s.kind === 'tab' ? `${shareTitle(s)}\n${s.url}` : shareTitle(s)
  return `<li class="squad-share ${s.seen ? '' : 'unseen'}"><button data-action="squadShareOpen" data-id="${esc(s.id)}" title="${esc(title)}">${icon(shareIcons[s.kind] || 'globe')}<span class="squad-share-text"><span class="squad-share-title">${esc(shareTitle(s))}</span>${from}</span>${s.seen ? '' : '<span class="squad-share-dot"></span>'}</button></li>`
}
// Notices of what the squad shared, at the bottom of the sidebar: they
// stay until opened (pressed), put away (×) or seen on the squad's page;
// three at most show, the rest counted on a button to the page. The page
// views are native windows over the page area, so the notices keep to the
// sidebar (not on the collapsed one, nor in the horizontal layout, where the
// squad's button counts them).
const TOASTS = 3
export function shareToasts() {
  const list = waiting()
  if (!list.length || state.layout !== 'vertical' || state.sidebarCollapsed) return ''
  const toast = (s) =>
    `<div class="squad-toast" style="--c:${esc(s.c)}"><button class="squad-toast-open" data-action="squadShareOpen" data-id="${esc(s.id)}" title="${esc(s.kind === 'tab' ? `${shareTitle(s)}\n${s.url}` : shareTitle(s))}">${icon(shareIcons[s.kind] || 'globe')}<span class="squad-share-text"><span class="squad-share-title">${esc(shareTitle(s))}</span><span class="squad-share-from"><span class="squad-dot" style="--c:${esc(s.c)}"></span>${esc(t(s.kind === 'snap' ? 'squadToastSnap' : 'squadToastTab').replace('{name}', s.name || '?'))}</span></span></button><button class="squad-toast-close" data-action="squadShareDismiss" data-id="${esc(s.id)}" title="${esc(t('squadToastDismiss'))}" aria-label="${esc(t('squadToastDismiss'))}">${icon('x')}</button></div>`
  const more = list.length > TOASTS ? `<button class="squad-toast-more" data-action="squadPage">${esc(t('squadToastMore').replace('{n}', String(list.length - TOASTS)))}</button>` : ''
  return `<div class="squad-toasts" role="status">${list.slice(0, TOASTS).map(toast).join('')}${more}</div>`
}

let shareFilter = 'all'
function sharesCard() {
  const chip = (id, label) =>
    `<button data-action="squadShareFilter" data-id="${id}" class="${shareFilter === id ? 'selected' : ''}">${esc(t(label))}</button>`
  const list = shares.filter((s) => shareFilter === 'all' || s.kind === shareFilter)
  return `<section class="panel squad-shares-card"><div class="squad-shares-head"><h2>${esc(t('squadShares'))}</h2><div class="segmented" role="group">${chip('all', 'squadSharesAll')}${chip('tab', 'squadSharesTabs')}${chip('snap', 'snapNotes')}${chip('draw', 'squadSharesDraw')}</div></div><p class="hint">${esc(t('squadSharesHint'))}</p>${list.length ? `<ul class="squad-shares">${list.map((s) => shareRow(s, true)).join('')}</ul>` : `<p class="shot-empty">${esc(t('squadSharesEmpty'))}</p>`}</section>`
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
  return `<div class="squad-colors"><span class="squad-colors-label">${esc(t('squadColor'))}</span><div class="squad-color-row"><button type="button" class="squad-color-auto ${chosen ? '' : 'on'}" data-action="squadColorPick" data-id="" aria-pressed="${!chosen}">${esc(t('squadColorAuto'))}</button>${squadColors.map(swatch).join('')}</div><p class="hint">${esc(t(busy ? 'squadColorBusy' : 'squadColorHelp'))}</p></div>`
}

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
    return `<div class="page squad-page">${head}${profile}<section class="panel squad-forms"><h2>${esc(t('squadJoinTitle'))}</h2><p class="hint">${esc(t('squadIntro'))}</p><form id="squad-form" class="squad-form"><button class="primary" type="submit" value="create">${esc(t('squadCreate'))}</button><div class="squad-join"><span class="secret-field"><input name="squadCode" class="${maskedClass('squadInput')}" maxlength="12" autocomplete="off" spellcheck="false" placeholder="ABCD-1234" aria-label="${esc(t('squadCode'))}" value="${esc(drafts.code)}">${revealButton('squadInput')}</span><button type="submit" value="join">${esc(t('squadJoin'))}</button></div>${notice}</form>${recentSquads()}${privacy}</section></div>`
  const viewer = s.members.find((m) => m.me)?.viewer ? `<p class="hint">${esc(t('squadViewer'))}</p>` : ''
  const code = `<div class="squad-code"><output>${esc(secretText('squad', state.squadCode || s.code))}</output>${revealButton('squad')}<button data-action="squadCopy" title="${esc(t('squadCopy'))}">${icon('copy')}<span>${esc(t(drafts.notice === 'squadCopied' ? 'squadCopied' : 'squadCopy'))}</span></button></div><p class="squad-phase" data-phase="${esc(s.phase)}">${esc(t('squadPhase_' + s.phase))}</p>`
  return `<div class="page squad-page">${head}${profile}<section class="panel squad-forms"><h2>${esc(t('squadJoined'))}</h2><h3>${esc(t('squadCode'))}</h3>${code}<h3>${esc(t('squadMembers'))}</h3>${memberList(s.members)}${viewer}${notice}<button class="squad-leave" data-action="squadLeave">${esc(t('squadLeave'))}</button>${privacy}</section>${sharesCard()}</div>`
}

const inForms = (el) => !!el.closest?.('.squad-forms')
document.addEventListener('input', (event) => {
  const el = /** @type {HTMLInputElement} */ (event.target)
  if (!inForms(el)) return
  if (el.name === 'squadName') drafts.name = el.value
  if (el.name === 'squadCode') drafts.code = el.value
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
  if (type === 'squadShareDismiss') {
    markSeen(id)
    return true
  }
  if (type === 'squadShareOpen') {
    const s = shares.find((x) => x.id === id)
    if (!s) return true
    markSeen(s.id)
    if (s.kind === 'tab') void action('open', s.url)
    else if (s.kind === 'snap') void action('snapNew', { image: s.image, title: s.title })
    else window.dispatchEvent(new CustomEvent('mayak:map-show', { detail: s.map }))
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
