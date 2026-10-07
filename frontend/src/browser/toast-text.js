// What a notice says (app_toast.go Toast), in the language given: its words
// with their values, then its own text (an error's message). The toast
// window and the notifications page share it; like words.js and icons.js it
// has no side effects, so other pages (toast.html) can load it.
import { t } from './words.js'

const modes = { pve: 'PvE', pvp: 'PvP' }

export function toastText(toast, language) {
  const params = { ...(toast?.params || {}) }
  if (params.mode) params.mode = modes[params.mode] || t(language, 'toastMode_' + params.mode)
  if (params.map) params.map = params.map.replace(/-/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase())
  let text = toast?.message ? t(language, toast.message) : ''
  for (const [key, value] of Object.entries(params)) text = text.split('{' + key + '}').join(String(value))
  if (toast?.text) text = text ? text + ' ' + toast.text : toast.text
  return text
}

// toastIcon is the icon of a notice's level (icons.js), a job running its own.
export const toastIcon = (toast) =>
  toast?.category === 'working' ? 'reload' : { success: 'check', warn: 'alert', error: 'alert' }[toast?.level] || 'info'
