import { t, esc, icon, render, clickHandlers } from './shell-core.js'
import {
  corners,
  cropPresets,
  ratioOf,
  fitRatio,
  moveInside,
  moveInView,
  resizeCorner,
  cropTo,
} from './snap-geometry.js'

// The snap note editor's canvas (view-snapnotes.js): the original image's
// place on it, and its crop. The original is locked at first; unlocked (the
// lock in the layer panel) and chosen there, a drag on the sheet moves it and
// its corners resize it, its ratio kept. Its pixels never change. The crop
// tool shows a frame of a chosen ratio; "Crop" makes the canvas the frame's
// size and moves the original and every line with it, removing nothing. The
// editor hands its state over with frameHost: {ed(), tool(), setTool(t),
// remember(), changed(), repaint(), finishText()}.

let host = null
export const frameHost = (h) => {
  host = h
}

const tenth = (v) => Math.round(v * 10) / 10
const imageSize = (ed) => ({ w: ed.note.width, h: ed.note.height })
const pct = (v, of) => (v / of) * 100 + '%'
// placeStyle sets an element at a rectangle of the canvas, in percents.
export const placeStyle = (r, c) =>
  `left:${pct(r.x, c.w)};top:${pct(r.y, c.h)};width:${pct(r.w, c.w)};height:${pct(r.h, c.h)}`
function place(selector, r, c) {
  const el = /** @type {HTMLElement} */ (document.querySelector(selector))
  if (!el) return
  el.style.left = pct(r.x, c.w)
  el.style.top = pct(r.y, c.h)
  el.style.width = pct(r.w, c.w)
  el.style.height = pct(r.h, c.h)
}
const handlesHTML = () => corners.map((c) => `<span class="snap-handle" data-corner="${c}"></span>`).join('')
const sizeText = (r) => `${Math.round(r.w)} × ${Math.round(r.h)}`

// The crop being set, for one note at one canvas size; the tool to go back
// to when it is done.
let crop = null,
  back = 'pen'
function cropOf(ed) {
  if (!crop || crop.id !== ed.id || crop.cw !== ed.canvas.w || crop.ch !== ed.canvas.h)
    crop = { id: ed.id, cw: ed.canvas.w, ch: ed.canvas.h, ratio: 'free', rect: fitRatio(0, ed.canvas) }
  return crop
}
export function enterCrop(from) {
  if (from !== 'crop') back = from
  crop = null
}
export function leaveCrop() {
  crop = null
  return back
}

// frameHTML is what lies over the ink: the crop's frame, or the original's
// with its corners while it is chosen.
export function frameHTML(ed, tool) {
  if (!ed?.editing) return ''
  if (tool === 'crop') {
    const c = cropOf(ed),
      style = placeStyle(c.rect, ed.canvas)
    return `<div class="snap-crop-layer"><div class="snap-crop-dim"><div class="snap-crop-hole" style="${style}"></div></div><div class="snap-crop" style="${style}"><span class="snap-crop-size">${sizeText(c.rect)}</span>${handlesHTML()}</div></div>`
  }
  return ed.baseActive
    ? `<div class="snap-base-frame" style="${placeStyle(ed.base, ed.canvas)}">${handlesHTML()}</div>`
    : ''
}

// The crop tool's row: the ratios, then crop and cancel.
export function cropPaletteHTML(ed) {
  const c = cropOf(ed)
  const label = (id) => (id === 'free' ? t('snapCropFree') : id === 'original' ? t('snapCropOriginal') : id)
  const preset = (id) =>
    `<button class="snap-crop-ratio${c.ratio === id ? ' on' : ''}" data-action="snapCropRatio" data-id="${id}" aria-pressed="${c.ratio === id}">${esc(label(id))}</button>`
  return `<div class="snap-text-style snap-crop-style"><span class="snap-group"><span class="snap-label">${esc(t('snapCropRatio'))}</span>${cropPresets.map(preset).join('')}</span><button class="snap-crop-apply" data-action="snapCropApply" title="${esc(t('snapCropApplyHint'))}">${icon('crop')}<span>${esc(t('snapCropApply'))}</span></button><button class="snap-crop-cancel" data-action="snapCropCancel" title="${esc(t('snapCropCancel'))} (Esc)">${esc(t('snapCropCancel'))}</button></div>`
}

// The original's row of the layer panel: eye is its eye button. Unlocked, its
// name chooses it.
export function baseRowHTML(ed, eye) {
  const locked = ed.base.locked
  const name = `<span>${esc(t('snapOriginal'))}</span><small>${esc(t(locked ? 'snapOriginalHint' : 'snapBaseMoveHint'))}</small>`
  const lock = t(locked ? 'snapBaseUnlock' : 'snapBaseLock')
  return `<div class="snap-layer base${ed.baseActive ? ' on' : ''}${ed.baseVisible ? '' : ' hidden'}">${eye}${locked ? `<span class="snap-layer-name">${name}</span>` : `<button class="snap-layer-name" data-action="snapBaseSelect" title="${esc(t('snapBaseSelect'))}" aria-pressed="${!!ed.baseActive}">${name}</button>`}<button class="snap-base-lock${locked ? '' : ' open'}" data-action="snapBaseLock" title="${esc(lock)}" aria-label="${esc(lock)}">${icon(locked ? 'lock' : 'unlock')}</button></div>`
}

// Dragging: the crop (inside it moves it, a corner resizes it) and the
// original while chosen (anywhere on the sheet moves it, a corner resizes it).
let drag = null
function canvasPoint(event, ed) {
  const r = document.querySelector('.snap-ink')?.getBoundingClientRect()
  if (!r?.width || !r.height) return null
  return [((event.clientX - r.left) / r.width) * ed.canvas.w, ((event.clientY - r.top) / r.height) * ed.canvas.h]
}
document.addEventListener('pointerdown', (event) => {
  const ed = host?.ed()
  const target = /** @type {Element} */ (event.target)
  if (!ed?.editing || event.button !== 0 || !target.closest) return
  const handle = /** @type {HTMLElement} */ (target.closest('.snap-handle'))
  const kind = target.closest('.snap-crop')
    ? 'crop'
    : handle || (target.closest('.snap-ink') && ed.baseActive && host.tool() !== 'crop')
      ? 'base'
      : ''
  const from = kind && canvasPoint(event, ed)
  if (!from) return
  event.preventDefault()
  target.setPointerCapture?.(event.pointerId)
  const start = kind === 'crop' ? cropOf(ed).rect : ed.base
  drag = {
    pointer: event.pointerId,
    ed,
    kind,
    corner: handle?.dataset.corner || '',
    from,
    start: { ...start },
    moved: false,
  }
})
document.addEventListener('pointermove', (event) => {
  if (!drag || event.pointerId !== drag.pointer) return
  const d = drag,
    ed = d.ed
  const p = ed === host.ed() && canvasPoint(event, ed)
  if (!p) return
  const dx = p[0] - d.from[0],
    dy = p[1] - d.from[1]
  if (!d.moved && !dx && !dy) return
  if (d.kind === 'base') {
    if (!d.moved) host.remember()
    const r = d.corner
      ? resizeCorner(d.start, d.corner, p, {
          ratio: d.start.w / d.start.h,
          min: 16,
          max: 20 * Math.max(ed.canvas.w, ed.canvas.h),
        })
      : moveInView(d.start, dx, dy, ed.canvas)
    ed.base = { ...ed.base, x: tenth(r.x), y: tenth(r.y), w: tenth(r.w), h: tenth(r.h) }
    place('.snap-base-box', ed.base, ed.canvas)
    place('.snap-base-frame', ed.base, ed.canvas)
  } else {
    const c = cropOf(ed)
    c.rect = d.corner
      ? resizeCorner(d.start, d.corner, p, {
          ratio: ratioOf(c.ratio, imageSize(ed)),
          bounds: ed.canvas,
          min: Math.min(16, ed.canvas.w, ed.canvas.h),
        })
      : moveInside(d.start, dx, dy, ed.canvas)
    place('.snap-crop', c.rect, ed.canvas)
    place('.snap-crop-hole', c.rect, ed.canvas)
    const size = document.querySelector('.snap-crop-size')
    if (size) size.textContent = sizeText(c.rect)
  }
  d.moved = true
})
const endDrag = (event) => {
  if (!drag || event.pointerId !== drag.pointer) return
  const done = drag
  drag = null
  if (!done.moved || done.ed !== host.ed()) return
  if (done.kind === 'base') host.changed()
  else render()
}
document.addEventListener('pointerup', endDrag)
document.addEventListener('pointercancel', endDrag)
document.addEventListener('keydown', (event) => {
  if (event.key !== 'Escape' || host?.tool() !== 'crop' || !document.querySelector('.snap-crop')) return
  if (/** @type {Element} */ (event.target).closest?.('input,textarea')) return
  event.preventDefault()
  host.setTool(leaveCrop())
  render()
})

// The crop: the canvas becomes the frame's size, and everything moves with it.
function applyCrop(ed) {
  const next = cropTo({ canvas: ed.canvas, base: ed.base, layers: ed.layers.map((l) => l.strokes) }, cropOf(ed).rect)
  host.setTool(leaveCrop())
  if (next.canvas.w === ed.canvas.w && next.canvas.h === ed.canvas.h) {
    render()
    return
  }
  host.remember()
  ed.canvas = next.canvas
  ed.base = next.base
  next.layers.forEach((strokes, i) => {
    ed.layers[i].strokes = strokes
  })
  host.repaint()
  host.changed()
}

clickHandlers.push((type, id) => {
  const ed = host?.ed()
  if (type === 'snapBaseLock') {
    if (ed) {
      host.finishText()
      host.remember()
      ed.base = { ...ed.base, locked: !ed.base.locked }
      // Unlocked to be moved: it is chosen.
      ed.baseActive = !ed.base.locked
      host.changed()
    }
    return true
  }
  if (type === 'snapBaseSelect') {
    if (ed && !ed.base.locked) {
      host.finishText()
      ed.baseActive = true
      render()
    }
    return true
  }
  if (type === 'snapCropRatio') {
    if (ed) {
      const c = cropOf(ed)
      c.ratio = cropPresets.includes(id) ? id : 'free'
      c.rect = fitRatio(ratioOf(c.ratio, imageSize(ed)), ed.canvas)
      render()
    }
    return true
  }
  if (type === 'snapCropApply') {
    if (ed) applyCrop(ed)
    return true
  }
  if (type === 'snapCropCancel') {
    host?.setTool(leaveCrop())
    render()
    return true
  }
  return false
})
