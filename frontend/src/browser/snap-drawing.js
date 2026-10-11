// A snap note's drawing as it is saved (view-snapnotes.js draws it; Go keeps
// it as it comes):
//
//   {v: 3, canvas: {w, h},
//    base: {visible, brightness, contrast, shadows, locked, x, y, w, h},
//    layers: [{visible, strokes}] ×3}
//
// Everything is in the canvas's pixels. The canvas starts as the original
// image's size, with the image at (0, 0) and locked; a crop makes it smaller,
// and an unlocked image can be moved and resized on it. A drawing of v2 (no
// canvas, the image fixed) and one from before layers (a list of strokes, on
// layer 1) read as the image's own size.

export const layerCount = 3
const limit = 1e6

// How the original shows: brightness (0.5–3), contrast (0.5–2) and shadows,
// a gamma lifting the dark parts (1 none, down to 0.3). EFT's screenshots are
// often dark; this is for looking into them. The image kept never changes.
export const adjustRanges = { brightness: [0.5, 3, 0.05], contrast: [0.5, 2, 0.05], shadows: [0, 1, 0.05] }
export const defaultAdjust = () => ({ brightness: 1, contrast: 1, shadows: 0 })
export function readAdjust(base) {
  const adjust = defaultAdjust()
  for (const [key, [lo, hi]] of Object.entries(adjustRanges)) {
    const v = Number(base?.[key])
    if (Number.isFinite(v)) adjust[key] = Math.min(hi, Math.max(lo, v))
  }
  return adjust
}

export const validStrokes = (list) =>
  Array.isArray(list) ? list.filter((s) => s && typeof s.c === 'string' && Array.isArray(s.p)) : []

// A number in [lo, hi], or null.
const num = (v, lo, hi) => {
  const n = typeof v === 'number' ? v : NaN
  return Number.isFinite(n) && n >= lo && n <= hi ? n : null
}

// readFrame is the canvas and the original's place on it; size is the
// original image's {w, h}.
export function readFrame(saved, size) {
  const canvas = { w: size.w, h: size.h }
  const base = { locked: true, x: 0, y: 0, w: size.w, h: size.h }
  if (!saved || typeof saved !== 'object' || Array.isArray(saved)) return { canvas, base }
  const cw = num(saved.canvas?.w, 1, limit),
    ch = num(saved.canvas?.h, 1, limit)
  if (cw !== null && ch !== null) {
    canvas.w = Math.round(cw)
    canvas.h = Math.round(ch)
  }
  const b = saved.base
  base.locked = b?.locked !== false
  const x = num(b?.x, -limit, limit),
    y = num(b?.y, -limit, limit),
    w = num(b?.w, 1, limit),
    h = num(b?.h, 1, limit)
  if (x !== null && y !== null && w !== null && h !== null) Object.assign(base, { x, y, w, h })
  return { canvas, base }
}

export function readDrawing(saved, size) {
  const layers = Array.from({ length: layerCount }, () => ({ visible: true, strokes: [] }))
  let baseVisible = true,
    adjust = defaultAdjust()
  if (Array.isArray(saved)) layers[0].strokes = validStrokes(saved)
  else if (saved && typeof saved === 'object') {
    baseVisible = saved.base?.visible !== false
    adjust = readAdjust(saved.base)
    ;(Array.isArray(saved.layers) ? saved.layers : []).slice(0, layerCount).forEach((l, i) => {
      layers[i] = { visible: l?.visible !== false, strokes: validStrokes(l?.strokes) }
    })
  }
  return { layers, baseVisible, adjust, ...readFrame(saved, size) }
}

export const writeDrawing = (target) => ({
  v: 3,
  canvas: { w: target.canvas.w, h: target.canvas.h },
  base: {
    visible: target.baseVisible,
    ...target.adjust,
    locked: target.base.locked,
    x: target.base.x,
    y: target.base.y,
    w: target.base.w,
    h: target.base.h,
  },
  layers: target.layers.map((l) => ({ visible: l.visible, strokes: l.strokes })),
})
