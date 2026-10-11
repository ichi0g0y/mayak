// The rectangles of a snap note's canvas ({x, y, w, h}, in the canvas's
// pixels): the original image's place, and a crop. No DOM here.

const clamp = (v, lo, hi) => Math.min(hi, Math.max(lo, v))
const tenth = (v) => Math.round(v * 10) / 10

export const corners = ['nw', 'ne', 'sw', 'se']

// The crop's ratios: free, the original image's own, and fixed ones.
export const cropPresets = ['free', 'original', '16:9', '4:3', '1:1', '9:16']
// ratioOf is a preset's width / height (0 for free); image is the original's {w, h}.
export function ratioOf(id, image) {
  if (id === 'original') return image.w / image.h
  const m = /^(\d+):(\d+)$/.exec(String(id))
  return m ? Number(m[1]) / Number(m[2]) : 0
}

// fitRatio is the largest rectangle of the ratio centred in the canvas; free
// (0) is the whole canvas.
export function fitRatio(ratio, canvas) {
  if (!(ratio > 0)) return { x: 0, y: 0, w: canvas.w, h: canvas.h }
  let w = canvas.w,
    h = w / ratio
  if (h > canvas.h) {
    h = canvas.h
    w = h * ratio
  }
  return { x: (canvas.w - w) / 2, y: (canvas.h - h) / 2, w, h }
}

// moveInside moves a rectangle, kept inside bounds (the crop).
export const moveInside = (rect, dx, dy, bounds) => ({
  ...rect,
  x: clamp(rect.x + dx, 0, bounds.w - rect.w),
  y: clamp(rect.y + dy, 0, bounds.h - rect.h),
})

// moveInView moves a rectangle anywhere, but a part of it stays on the canvas
// so it can be taken again (the original image).
export function moveInView(rect, dx, dy, canvas) {
  const m = Math.min(rect.w, rect.h, canvas.w, canvas.h, Math.max(16, Math.max(canvas.w, canvas.h) / 50))
  return {
    ...rect,
    x: tenth(clamp(rect.x + dx, m - rect.w, canvas.w - m)),
    y: tenth(clamp(rect.y + dy, m - rect.h, canvas.h - m)),
  }
}

// resizeCorner is start with its corner dragged to point (the opposite
// corner stays). A ratio (width / height, 0 free) is kept; bounds, when
// given, holds the rectangle; no side gets under min or over max.
export function resizeCorner(start, corner, point, { ratio = 0, bounds = null, min = 1, max = Infinity } = {}) {
  const east = corner.includes('e'),
    south = corner.includes('s')
  const ax = east ? start.x : start.x + start.w,
    ay = south ? start.y : start.y + start.h
  let w = east ? point[0] - ax : ax - point[0],
    h = south ? point[1] - ay : ay - point[1]
  const maxW = Math.min(max, bounds ? (east ? bounds.w - ax : ax) : Infinity),
    maxH = Math.min(max, bounds ? (south ? bounds.h - ay : ay) : Infinity)
  if (ratio > 0) {
    w = clamp(Math.max(w, h * ratio), Math.max(min, min * ratio), Math.min(maxW, maxH * ratio))
    h = w / ratio
  } else {
    w = clamp(w, min, maxW)
    h = clamp(h, min, maxH)
  }
  return { x: east ? ax : ax - w, y: south ? ay : ay - h, w, h }
}

// cropBox is a crop in whole pixels, inside the canvas.
export function cropBox(rect, canvas) {
  const x = clamp(Math.round(rect.x), 0, canvas.w - 1),
    y = clamp(Math.round(rect.y), 0, canvas.h - 1)
  return {
    x,
    y,
    w: clamp(Math.round(rect.w), 1, canvas.w - x),
    h: clamp(Math.round(rect.h), 1, canvas.h - y),
  }
}

const shiftStroke = (s, dx, dy) => ({ ...s, p: s.p.map(([x, y]) => [tenth(x + dx), tenth(y + dy)]) })

// cropTo is the drawing after a crop: the canvas is the crop's size, and the
// original and every line and text move with it. Nothing outside is removed;
// the strokes are new objects, so an undo keeps the old ones.
export function cropTo({ canvas, base, layers }, rect) {
  const box = cropBox(rect, canvas)
  return {
    canvas: { w: box.w, h: box.h },
    base: { ...base, x: tenth(base.x - box.x), y: tenth(base.y - box.y) },
    layers: layers.map((strokes) => strokes.map((s) => shiftStroke(s, -box.x, -box.y))),
  }
}
