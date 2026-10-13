import { t, state, esc, icon, api, render, clickHandlers, siteChoices } from './shell-core.js'
import { defaults, siteOrder } from './state.js'

// "Back to defaults" for the browser's own settings pages, as the Host's
// pages have it (SectionReset.tsx), in two steps: the first press lists what
// goes back (the values it goes back to coloured), the second applies them;
// "cancel" gives up. The defaults are the shell's (state.js) and, for the
// task sites and the notifications, the Go side's (DefaultSettings).

// The squad's notification events, the only ones on a Client's page.
const squadEvents = ['squadSelf', 'squadMembers', 'squadRelay']
const resettable = ['appearance', 'tasks', 'notifications', 'adblock']

// pending is the page whose reset waits for the second press: its rows and
// what applies it.
let pending = null

const onOff = (on) => t(on ? 'resetOn' : 'resetOff')
const navPosition = (s) => (s.layout === 'horizontal' ? 'top' : s.sidebarSide)
const place = { left: 'placeLeft', right: 'placeRight', top: 'placeTop', bottom: 'placeBottom' }

// changes lists a page's settings that differ from their defaults: a row
// each (its name, now, then), and the actions putting them back.
async function changes(section) {
  const d = defaults()
  const host = (await window.mayakDesktop?.backend?.DefaultSettings?.()) || {}
  const rows = []
  const prefs = {}
  const steps = []
  const pref = (key, label, now, then, show) => {
    if (now === then) return
    rows.push({ label: t(label), from: show(now), to: show(then) })
    prefs[key] = then
  }
  if (section === 'appearance') {
    const themes = (k) => (k === 'system' ? t('themeSystem') : k)
    pref('theme', 'theme', state.theme, d.theme, themes)
    pref('clock', 'clockFormat', state.clock, d.clock, (v) => t(v === '12' ? 'clock12' : 'clock24'))
    pref('navPosition', 'navPosition', navPosition(state), navPosition(d), (v) => t(place[v]))
    pref('itemDock', 'itemDock', state.itemDock, d.itemDock, (v) => t(place[v]))
    pref('trackerTab', 'trackerTab', !!state.trackerTab, !!d.trackerTab, (v) =>
      t(v ? 'trackerTabShow' : 'trackerTabHide'),
    )
  }
  if (section === 'tasks') {
    // The site order is the Host's: on the Host, or on a Client its Host's.
    const local = state.localHost && state.connection.mode === 'local'
    const now = local
      ? siteOrder(state.hostQuestSites)
      : state.hostView
        ? siteOrder(state.hostView.questSites, state.hostView.questSite)
        : null
    const then = siteOrder(host.questSites, host.questSite)
    if (now && now.join() !== then.join()) {
      const names = Object.fromEntries(siteChoices())
      const show = (order) => order.map((s) => names[s] || s).join(' → ')
      rows.push({ label: t('questSites'), from: show(now), to: show(then) })
      steps.push(() => api.action('questSites', then))
    }
    pref('taskMode', 'taskMode', state.taskMode, d.taskMode, (v) => t(v === 'reuse' ? 'reuse' : 'new'))
    pref('translateWiki', 'translateWiki', !!state.translateWiki, !!d.translateWiki, onOff)
  }
  if (section === 'notifications') {
    for (const event of squadEvents) {
      // A toast shows unless its event is off; a desktop notification shows
      // when it is on.
      const channels = [
        { channel: 'toast', list: 'toastsOff', shown: (l) => !l.includes(event) },
        { channel: 'desktop', list: 'desktopOn', shown: (l) => l.includes(event) },
      ]
      for (const { channel, list, shown } of channels) {
        const now = shown(state[list] || [])
        const then = shown(host[list] || [])
        if (now === then) continue
        rows.push({
          label: `${t('notifyEvent_' + event)}（${t(channel === 'toast' ? 'notifyToast' : 'notifyDesktop')}）`,
          from: onOff(now),
          to: onOff(then),
        })
        steps.push(() => api.action('notifyChannel', { event, channel, on: then }))
      }
    }
    if (!!state.desktopAlways !== !!host.desktopAlways) {
      const show = (v) => t(v ? 'notifyDesktopAlways' : 'notifyDesktopAway')
      rows.push({ label: t('notifyDesktopWhen'), from: show(!!state.desktopAlways), to: show(!!host.desktopAlways) })
      steps.push(() => api.action('notifyWhen', !!host.desktopAlways))
    }
  }
  if (section === 'adblock') pref('adblock', 'adblockEnable', state.adblock !== false, d.adblock !== false, onOff)
  if (Object.keys(prefs).length) steps.unshift(() => api.action('preferences', prefs))
  return {
    section,
    rows,
    apply: async () => {
      for (const step of steps) await step()
    },
  }
}

// resetHead is the button at the end of a page's title: the first press
// lists the changes, the second (it says so) applies them.
export function resetHead(section) {
  if (!resettable.includes(section)) return ''
  const armed = pending?.section === section && pending.rows.length
  return `<button class="reset-button ${armed ? 'primary' : ''}" data-action="sectionReset" data-id="${esc(section)}">${icon('undo')}${esc(t(armed ? 'resetCommit' : 'resetButton'))}</button>`
}

// resetPanel lists, under the title, what goes back while the second press
// waits; another page gives it up.
export function resetPanel(section) {
  if (pending && pending.section !== section) pending = null
  if (!pending) return ''
  if (!pending.rows.length) {
    return `<div class="reset-panel"><div class="reset-actions"><p class="reset-panel-title">${esc(t('resetNothing'))}</p><button data-action="sectionResetCancel">${esc(t('resetClose'))}</button></div></div>`
  }
  const rows = pending.rows
    .map(
      (r) =>
        `<li><span class="reset-name">${esc(r.label)}</span><span class="reset-from">${esc(r.from)}</span><span class="reset-arrow">→</span><span class="reset-to">${esc(r.to)}</span></li>`,
    )
    .join('')
  return `<div class="reset-panel" role="region" aria-label="${esc(t('resetTitle'))}"><p class="reset-panel-title">${esc(t('resetTitle'))}</p><ul>${rows}</ul><div class="reset-actions"><button class="primary" data-action="sectionReset" data-id="${esc(pending.section)}">${icon('undo')}${esc(t('resetCommit'))}</button><button data-action="sectionResetCancel">${esc(t('resetCancel'))}</button></div></div>`
}

clickHandlers.push(async (type, id) => {
  if (type === 'sectionResetCancel') {
    pending = null
    render()
    return true
  }
  if (type !== 'sectionReset') return false
  if (pending?.section === id && pending.rows.length) {
    const apply = pending.apply
    pending = null
    await apply()
  } else {
    pending = await changes(String(id))
  }
  render()
  return true
})
