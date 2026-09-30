import { esc } from './shell-core.js'

// Codes typed one character a box, as a passcode is: the pairing code (eight
// digits) and the squad code (eight letters and digits). Two groups of four
// boxes with the hyphen of ABCD-1234 shown between (never typed); the
// typing moves on by itself, Backspace goes back, the arrows move, a paste
// fills the boxes from the one it lands in, and the last character hands
// the code over. A group is named; its owner says what a character may be
// and hears each change and the whole code (onCodeBoxes).

const groups = new Map()

// onCodeBoxes sets up a group: allow (one character, upper-cased first) and
// what hears the code as it changes and once whole.
export function onCodeBoxes(name, { allow, onChange, onComplete }) {
  groups.set(name, { allow, onChange, onComplete })
}

// codeBoxes is a group's boxes, holding value; cls is added to each box (the
// masked class of a hidden code), label names them for a screen reader.
export function codeBoxes(name, value = '', { cls = '', label = '', numeric = false, size = 8 } = {}) {
  const code = String(value || '')
  const box = (i) =>
    `<input class="code-box ${cls}" data-code-box="${esc(name)}" data-i="${i}" ${numeric ? 'inputmode="numeric"' : ''} autocomplete="off" autocapitalize="characters" spellcheck="false" maxlength="1" aria-label="${esc(label)} ${i + 1}" value="${esc(code[i] || '')}">`
  const half = Math.ceil(size / 2)
  const all = Array.from({ length: size }, (_, i) => i)
  return `<span class="code-boxes">${all.slice(0, half).map(box).join('')}<span class="code-gap" aria-hidden="true">-</span>${all.slice(half).map(box).join('')}</span>`
}

const boxesOf = (name) =>
  /** @type {HTMLInputElement[]} */ ([...document.querySelectorAll(`[data-code-box="${CSS.escape(name)}"]`)])
const valueOf = (boxes) =>
  boxes
    .map((b) => b.value || ' ')
    .join('')
    .replace(/\s+$/, '')

function typed(el, text) {
  const name = el.dataset.codeBox
  const group = groups.get(name)
  if (!group) return
  const boxes = boxesOf(name)
  const i = Number(el.dataset.i)
  const chars = [...String(text || '').toUpperCase()].filter((c) => group.allow.test(c))
  for (let k = 0; k < chars.length && i + k < boxes.length; k++) boxes[i + k].value = chars[k]
  if (!chars.length) el.value = ''
  const value = valueOf(boxes)
  group.onChange?.(value)
  if (chars.length) boxes[Math.min(boxes.length - 1, i + chars.length)]?.focus()
  if (chars.length && value.length === boxes.length && !value.includes(' ')) group.onComplete?.(value)
}

document.addEventListener('input', (event) => {
  const el = /** @type {HTMLInputElement} */ (event.target)
  if (el.dataset?.codeBox) typed(el, el.value)
})
// A box takes one character, so the browser would cut a pasted code short.
document.addEventListener('paste', (event) => {
  const el = /** @type {HTMLInputElement} */ (event.target)
  if (!el.dataset?.codeBox) return
  event.preventDefault()
  typed(el, event.clipboardData?.getData('text') || '')
})
document.addEventListener('keydown', (event) => {
  const el = /** @type {HTMLInputElement} */ (event.target)
  if (!el.dataset?.codeBox) return
  const boxes = boxesOf(el.dataset.codeBox)
  const i = Number(el.dataset.i)
  if (event.key === 'Backspace' && !el.value && i > 0) {
    event.preventDefault()
    boxes[i - 1].value = ''
    boxes[i - 1].focus()
    groups.get(el.dataset.codeBox)?.onChange?.(valueOf(boxes))
  } else if (event.key === 'ArrowLeft' && i > 0) boxes[i - 1].focus()
  else if (event.key === 'ArrowRight' && i < boxes.length - 1) boxes[i + 1].focus()
})
// A box takes the focus with its character selected, to type over.
document.addEventListener('focusin', (event) => {
  const el = /** @type {HTMLInputElement} */ (event.target)
  if (el.dataset?.codeBox) el.select()
})
