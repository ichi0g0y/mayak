import { age } from './item.js'
import { allLevelsOf, ladderOf, shortNumber } from './levels.js'
import { t, state, esc, icon, clickHandlers, action } from './shell-core.js'

// The level line first in the sidebar: the level's badge in a ring filled
// as far as the level is made, the level and the one aimed at (the next that
// unlocks something, or the one chosen), the experience left to it and what
// it unlocks; pressed, the ladder of the levels ahead,
// each with what it takes (its bar's length, growing level after level), what
// is left to it and what it unlocks, and pressed in turn the level to aim at.
// The experience is the Overall screen's, as the Host read it last.

const number = (v) => Math.round(v).toLocaleString(state.language === 'ja' ? 'ja-JP' : 'en-US')

function unlockText(u) {
  if (u.kind === 'trader') return `${u.name} LL${u.loyalty}`
  if (u.kind === 'flea') return t('levelFlea')
  return t('levelBadge')
}
const unlocksText = (list) => list.map(unlockText).join(' · ')

// What is left to the level aimed at, right of the level: "Lv33 まで
// 151,854", the amount written short (2.12M) where the line is too narrow
// for it in full, as the CSS decides by the line's width (a million and
// more takes more room: "big").
function leftHTML(ladder) {
  const [before, after = ''] = t('levelLeft').replace('%l', String(ladder.target)).split('%d')
  const full = number(ladder.left)
  return `<span class="level-left ${ladder.left >= 1e6 ? 'big' : ''}" title="${esc(before + full + after)}">${esc(before)}<span class="n-full">${full}</span><span class="n-short">${shortNumber(ladder.left)}</span>${esc(after)}</span>`
}

export function levelSection() {
  // The Host's own level: a Client has no Overall screen of its own. Nothing
  // shows before the Overall screen is read, nor past the last level.
  if (!state.localHost || state.connection.mode !== 'local') return ''
  const info = state.levels
  const ladder = info && ladderOf(info.steps, info.level, info.exp, state.levelTarget)
  if (!ladder) return ''
  const open = state.levelsView === 'ladder'
  // The badge in a ring filled as far as the level is made, to the next (a
  // level's ring, whatever level is aimed at: what is left to that one is
  // the text's).
  const ring = `<span class="level-ring" style="--p:${(ladder.toNext * 100).toFixed(1)}">${ladder.badge ? `<img src="${esc(ladder.badge)}" alt="" loading="lazy" draggable="false">` : ''}</span>`
  const line = `<button class="level-line" data-action="levelLadder" aria-expanded="${open}" title="${esc(t(open ? 'levelClose' : 'levelOpen'))}">${ring}<span class="level-now">Lv${ladder.level}</span>${leftHTML(ladder)}<span class="level-what">${esc(unlocksText(ladder.unlocks) || t('levelNothing'))}</span></button>`
  const rows = open
    ? `<div class="level-card"><div class="level-ladder" role="list">${ladder.rows
        .map((r) => {
          const tip = t('levelTip')
            .replace('%l', String(r.level))
            .replace('%e', number(r.exp))
            .replace('%m', number(r.more))
          return `<button class="level-row ${r.level === ladder.target ? 'target' : ''}" role="listitem" data-action="levelTarget" data-id="${r.level}" aria-pressed="${r.level === ladder.target}" title="${esc(tip)}"><span class="level-num">${r.level}</span><span class="level-step"><i style="width:${((r.exp / ladder.widest) * 100).toFixed(1)}%"></i>${r.done ? `<em style="width:${((r.exp / ladder.widest) * r.done * 100).toFixed(1)}%"></em>` : ''}</span><span class="level-row-left">${number(r.left)}</span>${r.unlocks.length ? `<span class="level-unlocks">${esc(unlocksText(r.unlocks))}</span>` : ''}</button>`
        })
        .join(
          '',
        )}</div><button class="level-more" data-action="levelsPage" aria-pressed="${pageOpen()}">${esc(t('levelMore'))}${icon('chevron', 'level-more-chevron')}</button><p class="level-note">${esc(t('levelNote').replace('%a', age(info.at, state.language) || '—'))}</p></div>`
    : ''
  // Open, what opens under the line is a card (the line stays a row).
  return `<div class="level-section">${line}${rows}</div>`
}

const pageOpen = () => state.tabs.find((tab) => tab.id === state.active)?.kind === 'levels'

// What a level unlocks, with its picture: a trader's face, the new badge,
// the flea market's store.
function unlockHTML(u) {
  const picture =
    u.kind === 'flea'
      ? `<span class="level-unlock-icon">${icon('store')}</span>`
      : u.image
        ? `<img class="level-unlock-img ${u.kind}" src="${esc(u.image)}" alt="" loading="lazy" draggable="false">`
        : ''
  return `<span class="level-unlock">${picture}${esc(unlockText(u))}</span>`
}

// The levels page: every level from the first, each reached one faded with
// its bar full, the one played marked with its badge, the next filled as far
// as the player is into it; what each takes and how much more than the level
// before, what is left to it, and what it unlocks. A level ahead pressed is
// aimed at, as in the sidebar's ladder. It opens at the level played.
export function levelsPage() {
  const info = state.levels
  const all = info && allLevelsOf(info.steps, info.level, info.exp, state.levelTarget)
  const title = `<div class="bookmarks-head"><h1>${esc(t('levelsPage'))}</h1></div>`
  if (!all) return `<div class="page levels-page">${title}<p class="hint">${esc(t('levelEmpty'))}</p></div>`
  const head = `<div class="levels-row levels-head" aria-hidden="true"><span>${esc(t('levelColLevel'))}</span><span>${esc(t('levelColExp'))}</span><span>${esc(t('levelColMore'))}</span><span>${esc(t('levelColLeft'))}</span><span>${esc(t('levelColUnlocks'))}</span></div>`
  const more = (m) => (m > 0 ? '+' + number(m) : m < 0 ? '−' + number(-m) : '—')
  const rows = all.rows
    .map((r) => {
      const ahead = r.state === 'next' || r.state === 'ahead'
      const bar = `<span class="level-step"><i style="width:${((r.exp / all.widest) * 100).toFixed(1)}%"></i>${r.done ? `<em style="width:${((r.exp / all.widest) * r.done * 100).toFixed(1)}%"></em>` : ''}</span>`
      const num =
        r.state === 'now' && all.badge
          ? `<img class="levels-badge" src="${esc(all.badge)}" alt="" loading="lazy" draggable="false">${r.level}`
          : String(r.level)
      const left =
        r.state === 'now' ? esc(t('levelNow')) : r.state === 'reached' ? icon('check', 'levels-check') : number(r.left)
      const cells = `<span class="levels-num">${num}</span><span class="levels-exp"><b>${number(r.exp)}</b>${bar}</span><span class="levels-more">${more(r.more)}</span><span class="levels-left">${left}</span><span class="levels-unlocks">${r.unlocks.map(unlockHTML).join('')}</span>`
      const cls = `levels-row ${r.state} ${r.level === all.target ? 'target' : ''}`
      return ahead
        ? `<button class="${cls}" data-action="levelTarget" data-id="${r.level}" aria-pressed="${r.level === all.target}">${cells}</button>`
        : `<div class="${cls}" ${r.state === 'now' ? 'id="levels-now"' : ''}>${cells}</div>`
    })
    .join('')
  return `<div class="page levels-page">${title}<p class="hint">${esc(t('levelPageNote').replace('%a', age(info.at, state.language) || '—'))}</p><div class="levels-list">${head}${rows}</div></div>`
}

clickHandlers.push(async (type, id) => {
  if (type === 'levelsPage') {
    // The page opens at the level played.
    await action('levelsPage')
    requestAnimationFrame(() => document.querySelector('#levels-now')?.scrollIntoView({ block: 'center' }))
    return true
  }
  if (type === 'levelLadder') {
    void action('preferences', { levelsView: state.levelsView === 'ladder' ? 'line' : 'ladder' })
    return true
  }
  if (type === 'levelTarget') {
    const level = Number(id)
    // The level aimed at pressed again goes back to the next unlock.
    void action('preferences', { levelTarget: level === state.levelTarget ? 0 : level })
    return true
  }
  return false
})
