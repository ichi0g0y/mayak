import test from 'node:test'
import assert from 'node:assert/strict'
import { readDrawing, readFrame, writeDrawing } from './snap-drawing.js'
import { ratioOf, fitRatio, moveInside, moveInView, resizeCorner, cropBox, cropTo } from './snap-geometry.js'

const size = { w: 1600, h: 1000 }
const line = {
  c: '#ff0000',
  w: 6,
  p: [
    [10, 20],
    [30, 40],
  ],
}

test('a drawing from before layers and a v2 drawing read with the canvas at the image size', () => {
  const legacy = readDrawing([line, { bad: true }], size)
  assert.deepEqual(legacy.layers[0].strokes, [line])
  assert.deepEqual(legacy.canvas, { w: 1600, h: 1000 })
  assert.deepEqual(legacy.base, { locked: true, x: 0, y: 0, w: 1600, h: 1000 })
  const v2 = readDrawing(
    { v: 2, base: { visible: false, brightness: 9 }, layers: [{ strokes: [line] }, { visible: false }] },
    size,
  )
  assert.equal(v2.baseVisible, false)
  assert.equal(v2.adjust.brightness, 3)
  assert.equal(v2.layers[1].visible, false)
  assert.deepEqual(v2.canvas, { w: 1600, h: 1000 })
  assert.equal(v2.base.locked, true)
  assert.deepEqual(readDrawing(null, size).canvas, { w: 1600, h: 1000 })
})

test('a v3 drawing keeps its canvas and the original place, and bad numbers fall back', () => {
  const saved = {
    v: 3,
    canvas: { w: 800.4, h: 450 },
    base: { visible: true, locked: false, x: -100, y: 20.5, w: 1200, h: 750 },
    layers: [{ strokes: [line] }],
  }
  const read = readDrawing(saved, size)
  assert.deepEqual(read.canvas, { w: 800, h: 450 })
  assert.deepEqual(read.base, { locked: false, x: -100, y: 20.5, w: 1200, h: 750 })
  const out = writeDrawing(read)
  assert.equal(out.v, 3)
  assert.deepEqual(out.canvas, { w: 800, h: 450 })
  assert.deepEqual(out.base, {
    visible: true,
    brightness: 1,
    contrast: 1,
    shadows: 0,
    locked: false,
    x: -100,
    y: 20.5,
    w: 1200,
    h: 750,
  })
  assert.deepEqual(readDrawing(out, size).layers[0].strokes, [line])

  const bad = readFrame({ canvas: { w: 0, h: 'x' }, base: { x: Infinity, y: 0, w: -5, h: 10 } }, size)
  assert.deepEqual(bad.canvas, { w: 1600, h: 1000 })
  assert.deepEqual(bad.base, { locked: true, x: 0, y: 0, w: 1600, h: 1000 })
  assert.deepEqual(readFrame({ canvas: { w: 1e9, h: 10 } }, size).canvas, { w: 1600, h: 1000 })
})

test('crop presets fit the largest centred rectangle', () => {
  assert.equal(ratioOf('free', size), 0)
  assert.equal(ratioOf('original', size), 1.6)
  assert.equal(ratioOf('9:16', size), 9 / 16)
  assert.deepEqual(fitRatio(0, size), { x: 0, y: 0, w: 1600, h: 1000 })
  assert.deepEqual(fitRatio(1, size), { x: 300, y: 0, w: 1000, h: 1000 })
  assert.deepEqual(fitRatio(16 / 9, size), { x: 0, y: 50, w: 1600, h: 900 })
})

test('a crop moves inside the canvas, the original stays partly on it', () => {
  const r = { x: 100, y: 100, w: 400, h: 300 }
  assert.deepEqual(moveInside(r, -500, 5000, size), { x: 0, y: 700, w: 400, h: 300 })
  const far = moveInView(r, 10000, -10000, size)
  assert.equal(far.x, 1600 - 32)
  assert.equal(far.y, 32 - 300)
})

test('a corner resizes from the opposite corner, keeping the ratio and the bounds', () => {
  const r = { x: 100, y: 100, w: 400, h: 200 }
  // Free: the south-east corner to the point.
  assert.deepEqual(resizeCorner(r, 'se', [700, 500]), { x: 100, y: 100, w: 600, h: 400 })
  // The north-west corner moves, the south-east one stays.
  assert.deepEqual(resizeCorner(r, 'nw', [300, 200]), { x: 300, y: 200, w: 200, h: 100 })
  // A ratio takes the larger side.
  assert.deepEqual(resizeCorner(r, 'se', [600, 500], { ratio: 2 }), { x: 100, y: 100, w: 800, h: 400 })
  // Bounds hold it, the ratio kept.
  const held = resizeCorner(r, 'se', [5000, 5000], { ratio: 2, bounds: size })
  assert.deepEqual(held, { x: 100, y: 100, w: 1500, h: 750 })
  // Past the fixed corner: the smallest size.
  assert.deepEqual(resizeCorner(r, 'ne', [0, 900], { min: 16 }), { x: 100, y: 284, w: 16, h: 16 })
  assert.deepEqual(resizeCorner(r, 'sw', [900, 0], { ratio: 2, min: 16 }), { x: 468, y: 100, w: 32, h: 16 })
})

test('a crop sets the canvas and moves everything without removing anything', () => {
  assert.deepEqual(cropBox({ x: -3.2, y: 10.6, w: 2000, h: 99.5 }, size), { x: 0, y: 11, w: 1600, h: 100 })
  const text = { c: '#fff', w: 32, p: [[5, 5]], text: 'hi' }
  const before = { canvas: size, base: { locked: true, x: 0, y: 0, w: 1600, h: 1000 }, layers: [[line, text], []] }
  const after = cropTo(before, { x: 20, y: 30, w: 800.4, h: 450 })
  assert.deepEqual(after.canvas, { w: 800, h: 450 })
  assert.deepEqual(after.base, { locked: true, x: -20, y: -30, w: 1600, h: 1000 })
  assert.deepEqual(after.layers[0][0].p, [
    [-10, -10],
    [10, 10],
  ])
  assert.deepEqual(after.layers[0][1], { ...text, p: [[-15, -25]] })
  assert.deepEqual(after.layers[1], [])
  // The old strokes are left as they were, for the undo.
  assert.deepEqual(line.p, [
    [10, 20],
    [30, 40],
  ])
})
