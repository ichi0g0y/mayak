import { price, offerNeeds } from './item.js'
import { t, state, esc, icon } from './shell-core.js'

// The item panel's traders selling the item: for cash, cheapest first (in
// roubles), then in barters from the lowest loyalty level. Each shows the
// trader and the loyalty level, the price (dollars and euros with their
// roubles) or the items the barter takes (each opens in the panel), what
// reaching the level takes, the buy limit per restock, and the task
// unlocking the offer, which opens like the item's tasks and shows when
// TarkovTracker has it done. Nothing for an item no trader sells. iconHTML
// draws an item's picture (view-item.js).
// traderAvatar is a trader's picture, round, before its name (iconHTML
// draws it, a "?" when it does not load); "flea" is the flea market's store,
// and a trader without a picture its initial, so the rows keep their shape.
export function traderAvatar(item, name, iconHTML) {
  if (name === 'flea') return `<span class="trader-avatar flea">${icon('store')}</span>`
  const url = item.traderImages?.[name]
  return url
    ? iconHTML(url, 'trader-avatar', 'loading="lazy"')
    : `<span class="trader-avatar initial" aria-hidden="true">${esc([...String(name)][0]?.toUpperCase() || '?')}</span>`
}

export function itemOffers(item, iconHTML) {
  if (!item.offers.length) return ''
  const lang = state.language
  const money = (v, c = 'RUB') => esc(price(v, c, lang))
  const number = (v) => v.toLocaleString(lang === 'ja' ? 'ja-JP' : 'en-US', { maximumFractionDigits: 2 })
  const rows = item.offers.map((o) => {
    const barter = o.barter.length > 0
    const needs = offerNeeds(o)
    const notes = [
      needs ? t('offerNeeds').replace('%l', String(needs.level)).replace('%r', needs.reputation) : '',
      o.count > 1 ? t('offerCount').replace('%d', number(o.count)) : '',
      o.buyLimit ? t('offerLimit').replace('%d', number(o.buyLimit)) : '',
    ].filter(Boolean)
    const cost = barter
      ? `<span class="offer-barter-tag">${esc(t('offerBarter'))}</span>`
      : `<b class="offer-price">${o.currency === 'RUB' ? money(o.price) : `${money(o.price, o.currency)} <small>(${money(o.priceRub)})</small>`}</b>`
    const takes = barter
      ? `<div class="offer-barter">${o.barter.map((b) => barterItem(b, number, iconHTML)).join('')}</div>`
      : ''
    // The trader's picture, its name beside it in the middle, and on the right
    // the price over the terms, as tall as the picture.
    return `<li><div class="offer-top">${traderAvatar(item, o.trader, iconHTML)}<span class="offer-trader">${esc(o.trader)}<span class="offer-ll">LL${o.level}</span></span>${cost}${notes.length ? `<div class="offer-notes">${notes.map((n) => `<span>${esc(n)}</span>`).join('')}</div>` : ''}</div>${takes}${o.task ? offerTask(o.task) : ''}</li>`
  })
  return `<section class="item-section"><h3>${esc(t('itemOffers'))}<span class="count">${item.offers.length}</span></h3><ul class="item-offers">${rows.join('')}</ul></section>`
}

// An item a barter takes, in the shell's language: its picture, name and
// count; it opens in the panel.
function barterItem(b, number, iconHTML) {
  const name = b.names?.[state.language] || b.name
  return `<button class="need-row offer-barter-item" data-action="itemSelect" data-id="${esc(b.id)}" title="${esc(b.name)}">${b.iconUrl ? iconHTML(b.iconUrl, 'offer-barter-img', 'loading="lazy"') : icon('package')}<span class="need-name">${esc(name)}</span><span class="offer-barter-count">×${esc(number(b.count))}</span></button>`
}

// The task unlocking an offer, in the shell's language when the catalog has
// its name there.
function offerTask(task) {
  const name = task.names?.[state.language] || task.name
  const done = task.state === 'completed'
  return `<button class="need-row offer-task ${state.popup?.key === 'task:' + task.id ? 'popup-source' : ''}" data-action="itemTask" data-id="${esc(task.id)}" title="${esc(t('openTask'))}">${icon('task')}<span class="need-name" title="${esc(task.name)}">${esc(t('offerTask').replace('%s', () => name))}</span>${done ? `<span class="need-done" title="${esc(t('needDone'))}">${icon('check')}</span>` : ''}</button>`
}
