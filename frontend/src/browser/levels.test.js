import test from 'node:test'
import assert from 'node:assert/strict'
import { allLevelsOf, ladderOf, levelTargetOf, shortNumber } from './levels.js'

// PvE's levels 31 to 40, as the catalog has them (app_levels.go levelSteps).
const steps = [
  { level: 31, exp: 166982, total: 1666320 },
  { level: 32, exp: 180344, total: 1846664, badge: 'https://assets.tarkov.dev/player-level-group-7.png' },
  { level: 33, exp: 196685, total: 2043349, unlocks: [{ kind: 'trader', name: 'Jaeger', loyalty: 4 }] },
  { level: 34, exp: 215087, total: 2258436 },
  { level: 35, exp: 233690, total: 2492126, unlocks: [{ kind: 'badge' }, { kind: 'trader', name: 'Ref', loyalty: 4 }] },
  { level: 36, exp: 258091, total: 2750217 },
  { level: 37, exp: 281805, total: 3032022 },
  { level: 38, exp: 305744, total: 3337766 },
  { level: 39, exp: 326065, total: 3663831 },
  { level: 40, exp: 346570, total: 4010401, unlocks: [{ kind: 'badge' }] },
]

test('the level aimed at is the one chosen while ahead, else the next unlock', () => {
  assert.equal(levelTargetOf(steps, 32, 0), 33)
  assert.equal(levelTargetOf(steps, 33, 0), 35)
  assert.equal(levelTargetOf(steps, 32, 40), 40)
  // A level reached already, or one past the catalog, is no aim.
  assert.equal(levelTargetOf(steps, 36, 35), 40)
  assert.equal(levelTargetOf(steps, 36, 99), 40)
  assert.equal(levelTargetOf(steps, 40, 0), null)
})

test('the ladder: what is left to the aim and to each level, and what each takes more', () => {
  const l = ladderOf(steps, 32, 1891495, 35)
  assert.equal(l.target, 35)
  assert.equal(l.badge, 'https://assets.tarkov.dev/player-level-group-7.png')
  assert.equal(l.left, 2492126 - 1891495)
  assert.equal(l.unlocks.length, 2)
  // The ring: how far into level 32 the player is, whatever the aim.
  assert.ok(Math.abs(l.toNext - (1891495 - 1846664) / 196685) < 1e-9)
  // The next 8 levels at least: 33 to 40.
  assert.deepEqual(
    l.rows.map((r) => r.level),
    [33, 34, 35, 36, 37, 38, 39, 40],
  )
  const [next, after] = l.rows
  assert.equal(next.left, 2043349 - 1891495)
  assert.equal(next.more, 196685 - 180344)
  assert.ok(Math.abs(next.done - (1891495 - 1846664) / 196685) < 1e-9)
  assert.equal(after.done, 0)
  assert.equal(l.widest, 346570)
  assert.equal(ladderOf(steps, 32, NaN), null)
  assert.equal(ladderOf([], 32, 1), null)
})

test('an amount written short where the full one does not fit', () => {
  assert.equal(shortNumber(2118906), '2.12M')
  assert.equal(shortNumber(12345678), '12.3M')
  assert.equal(shortNumber(600631), '601K')
  assert.equal(shortNumber(999999), '1.00M')
  assert.equal(shortNumber(15234), '15.2K')
  assert.equal(shortNumber(812), '812')
})

test('the levels page has every level: reached, the one played, the next and those ahead', () => {
  const all = allLevelsOf(steps, 32, 1891495)
  assert.deepEqual(
    all.rows.map((r) => [r.level, r.state]),
    [
      [31, 'reached'],
      [32, 'now'],
      [33, 'next'],
      [34, 'ahead'],
      [35, 'ahead'],
      [36, 'ahead'],
      [37, 'ahead'],
      [38, 'ahead'],
      [39, 'ahead'],
      [40, 'ahead'],
    ],
  )
  const [reached, now, next] = all.rows
  assert.equal(reached.done, 1)
  assert.equal(reached.left, 0)
  assert.equal(now.done, 1)
  assert.ok(next.done > 0 && next.done < 1)
  assert.equal(next.more, 196685 - 180344)
  assert.equal(all.target, 33)
})
