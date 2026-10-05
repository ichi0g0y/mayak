import { words } from './words.js'
import { esc, icon } from './icons.js'

// What every part of the browser shell shares: the desktop API, the current
// state, the render and action entry points, the words and the HTML helpers.
// The views (view-*.js) import from here and never from shell.js, so that
// shell.js can import them.

// The shell's state, a snapshot from api.js: every view reads it here and
// only setState (shell.js, on each state event) and action replace it.
export let state = null
export const setState = (next) => {
  state = next
}
// render is shell.js's; the views call it through this binding.
export let render = () => {}
export const setRender = (fn) => {
  render = fn
}
// The views' click handlers: (type,id,button,event) => handled.
export const clickHandlers = []
// Functions run after each render, for what the markup cannot carry (a
// canvas's drawing, an image too large to put in the markup).
export const afterRenderHooks = []

// The shell's API (api.js sets window.mayak), looked up when used, not when
// this module runs: in a build, a module can run before api.js has.
// A callback registered before api.js has run is handed over once it has
// (tried again every 10 ms, for up to 5 s).
const register = (name, fn, tries = 0) => {
  if (window.mayak) window.mayak[name](fn)
  else if (tries < 500) setTimeout(() => register(name, fn, tries + 1), 10)
}
export const api = {
  action: (type, data) => window.mayak.action(type, data),
  request: (type, data) => window.mayak.request(type, data),
  onState: (fn) => register('onState', fn),
  onKey: (fn) => register('onKey', fn),
  onMenu: (fn) => register('onMenu', fn),
}
// The build's version, for the About section; empty in development. The
// desktop bridge (api.js) exists once the first state arrives, so it loads then.
export let appVersion = ''
// Whether this is the `task dev` build (its own data; runs beside MAYAK).
export let devInstance = false
// The built-in voices, for their credits under Licenses.
export let voicePacks = []
export async function loadVersion() {
  try {
    appVersion = (await window.mayakDesktop?.backend?.GetVersion?.()) || ''
  } catch {}
  try {
    devInstance = !!(await window.mayakDesktop?.backend?.DevInstance?.())
  } catch {}
  try {
    const packs = await window.mayakDesktop?.backend?.VoicePacks?.()
    if (Array.isArray(packs)) voicePacks = packs
  } catch {}
  if (appVersion || voicePacks.length || devInstance) render()
}
// Where development can be supported (About, the tutorial's last step).
export const SUPPORT_URL = 'https://buymeacoffee.com/ichi0g0y'
export const t = (key) => words[state?.language || 'ja'][key] || key
export { esc, icon }

// Codes that let someone in (a squad, a pairing) show as dots until their eye
// button is pressed, so a stream or a screenshot does not give them away. A
// code stays shown until pressed again or the app restarts. Fields typed into
// hide theirs with the masked class (-webkit-text-security).
const revealed = new Set()
export const isRevealed = (key) => revealed.has(key)
export const secretText = (key, text) => (revealed.has(key) ? String(text) : String(text).replace(/[^\s-]/g, '•'))
export const maskedClass = (key) => (revealed.has(key) ? '' : 'masked')
export function revealButton(key) {
  const shown = revealed.has(key)
  const label = esc(t(shown ? 'hideCode' : 'showCode'))
  return `<button type="button" class="reveal" data-action="reveal" data-id="${esc(key)}" title="${label}" aria-label="${label}" aria-pressed="${shown}">${icon(shown ? 'eyeOff' : 'eye')}</button>`
}
clickHandlers.push(async (type, id) => {
  if (type !== 'reveal') return false
  if (!revealed.delete(id)) revealed.add(id)
  render()
  return true
})
export const option = (value, label, current) =>
  `<option value="${esc(value)}" ${current === value ? 'selected' : ''}>${esc(label)}</option>`
export function select(key, label, choices, value, scope = 'preferences') {
  return `<label class="field"><span>${esc(label)}</span><select data-scope="${scope}" data-key="${key}">${choices.map(([v, l]) => option(v, l, value)).join('')}</select></label>`
}
// The task sites in their default order (state.js sites), as menus list them.
export const siteChoices = () => [
  ['official-wiki', t('official')],
  ['japanese-wiki', t('japanese')],
  ['tarkov-dev', 'tarkov.dev'],
]

export async function action(type, data) {
  const next = await api.action(type, data)
  if (next) {
    state = next
    render()
  }
  return next
}
// request is action that throws when the action fails (action turns a
// failure into the current state, for callers that need not know).
export async function request(type, data) {
  const next = await api.request(type, data)
  if (next) {
    state = next
    render()
  }
  return next
}
