// The toast window (app_toast.go): the notices over the pages, which the
// shell cannot cover, stacked at the bottom middle of the main window. It
// draws the ones it is sent, reports its height so the window fits them, and
// passes a press back (a notice's button, its close).
import './style.css'
import { Events } from '@wailsio/runtime'
import {
  BrowserDesktopNotify,
  BrowserLoad,
  BrowserToastAction,
  BrowserToastClose,
  BrowserToastList,
  BrowserToastReady,
} from '../../bindings/github.com/local/mayak/internal/app/app'
import { icon, esc } from './icons.js'
import { t } from './words.js'
import { toastText, toastIcon } from './toast-text.js'

let language = 'ja'
let current = []
// The shell's language and theme, from its saved state: read again for each
// change of the notices, so a change of either shows on the next.
async function look() {
  try {
    const s = JSON.parse((await BrowserLoad()) || '{}')
    language = s.language === 'en' ? 'en' : 'ja'
    const light = matchMedia('(prefers-color-scheme: light)').matches
    document.documentElement.dataset.theme =
      s.theme === 'system' || !s.theme ? (light ? 'mayak-light' : 'mayak-dark') : s.theme
    document.documentElement.lang = language
  } catch {}
}

function row(toast) {
  const actions = (toast.actions || [])
    .map(
      (a) =>
        `<button class="toast-action" data-id="${esc(toast.id)}" data-act="${esc(a.id)}">${esc(t(language, a.label))}</button>`,
    )
    .join('')
  const progress =
    toast.progress > 0
      ? `<span class="toast-progress"><i style="width:${Math.max(0, Math.min(100, toast.progress | 0))}%"></i></span>`
      : ''
  return `<div class="toast-row" data-level="${esc(toast.level || 'info')}" data-category="${esc(toast.category || '')}" role="${toast.level === 'error' ? 'alert' : 'status'}">${icon(toastIcon(toast), toast.category === 'working' ? 'toast-icon spin' : 'toast-icon')}<div class="toast-body"><span class="toast-text">${esc(toastText(toast, language))}</span>${progress}${actions ? `<span class="toast-actions">${actions}</span>` : ''}</div><button class="toast-close" data-close="${esc(toast.id)}" title="${esc(t(language, 'close'))}" aria-label="${esc(t(language, 'close'))}">${icon('x')}</button></div>`
}

async function draw(list) {
  current = Array.isArray(list) ? list : []
  await look()
  const app = document.querySelector('#app')
  app.innerHTML = current.length ? `<div class="toast-list">${current.map(row).join('')}</div>` : ''
  requestAnimationFrame(
    () => void BrowserToastReady(current.length ? Math.ceil(app.getBoundingClientRect().height) : 0),
  )
}

Events.On('toast:list', (event) => void draw(event.data))
// A notice the settings also show as a desktop notification of the OS (while
// MAYAK is not in front): worded here, as the toast is.
Events.On('toast:desktop', async (event) => {
  await look()
  void BrowserDesktopNotify(toastText(event.data, language)).catch(() => {})
})
// Notices sent before this page loaded: ask for them.
BrowserToastList()
  .then((list) => void draw(list))
  .catch(() => {})
document.addEventListener('click', (event) => {
  const close = event.target.closest('[data-close]')
  if (close) {
    void BrowserToastClose(close.dataset.close)
    return
  }
  const press = event.target.closest('[data-act]')
  if (press) void BrowserToastAction(press.dataset.id, press.dataset.act)
})
