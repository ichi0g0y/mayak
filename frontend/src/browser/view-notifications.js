// The notifications page: the notices shown over the pages (app_toast.go),
// newest first, kept on this PC; it empties on request.
import { t, state, esc, icon } from './shell-core.js'
import { toastText, toastIcon } from './toast-text.js'

const when = (at) => {
  const d = new Date(at)
  return Number.isNaN(d.getTime())
    ? ''
    : d.toLocaleString(state.language === 'en' ? 'en' : 'ja', {
        month: 'numeric',
        day: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
      })
}

export function notificationsPage() {
  const list = [...(state.notifications || [])].reverse()
  const rows = list.length
    ? `<ul class="notice-list">${list
        .map(
          (n) =>
            `<li class="notice-row" data-level="${esc(n.level || 'info')}">${icon(toastIcon(n), 'notice-icon')}<span class="notice-text">${esc(toastText(n, state.language))}</span><span class="notice-meta"><span class="notice-kind">${esc(t('toastKind_' + (n.category || 'error')))}</span><time>${esc(when(n.at))}</time></span></li>`,
        )
        .join('')}</ul>`
    : `<p class="empty-tabs">${esc(t('notificationsEmpty'))}</p>`
  return `<div class="page notifications-page"><div class="bookmarks-head"><h1>${esc(t('notifications'))}</h1>${list.length ? `<button data-action="notificationsClear">${icon('trash')}<span>${esc(t('notificationsClear'))}</span></button>` : ''}</div><p class="hint">${esc(t('notificationsHelp'))}</p>${rows}</div>`
}
