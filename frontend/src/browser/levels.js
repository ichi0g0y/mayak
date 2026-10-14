// The level ladder's arithmetic (view-levels.js): where the player stands
// between levels, the level aimed at and what is left to it, and the levels
// ahead with what each takes and unlocks. steps are app_levels.go's
// LevelStep (exp: from the level before, total: from the start); level and
// exp the player's, from the Overall screen.

// shortNumber writes an amount of experience in a few characters, for where
// the full one does not fit: 15.2K, 152K, 2.12M.
export function shortNumber(n) {
  // 999,500 and more round to a thousand thousands: a million.
  if (n >= 999500) return `${(n / 1e6).toFixed(n >= 1e7 ? 1 : 2)}M`
  if (n >= 1e3) return `${(n / 1e3).toFixed(n >= 1e5 ? 0 : 1)}K`
  return String(Math.round(n))
}

// The levels ahead shown at least, when the one aimed at is nearer.
export const ladderAhead = 8

// levelTargetOf is the level aimed at: the one chosen while still ahead,
// else the next that unlocks something, else the next.
export function levelTargetOf(steps, level, chosen) {
  const ahead = steps.filter((s) => s.level > level)
  if (!ahead.length) return null
  if (chosen > level && ahead.some((s) => s.level === chosen)) return chosen
  return (ahead.find((s) => s.unlocks?.length) || ahead[0]).level
}

// ladderOf is all the ladder shows; null without steps or the player's
// experience. Each row: the level, what it takes, how much more than the
// level before, what is left to it, its unlocks, and how far into it the
// player is (0 to 1; only the next level is started). all: every level
// ahead, for the levels page, not only the next few.
export function ladderOf(steps, level, exp, chosen = 0, all = false) {
  if (!Array.isArray(steps) || !steps.length || !(level > 0) || !(exp >= 0)) return null
  const at = (l) => steps.find((s) => s.level === l)
  const here = at(level)
  const target = levelTargetOf(steps, level, chosen)
  if (!here || target === null) return null
  const last = all ? Infinity : Math.max(target, level + ladderAhead)
  const rows = steps
    .filter((s) => s.level > level && s.level <= last)
    .map((s) => {
      const before = at(s.level - 1)
      return {
        level: s.level,
        exp: s.exp,
        more: before && before.level > 1 ? s.exp - before.exp : 0,
        left: Math.max(0, s.total - exp),
        unlocks: s.unlocks || [],
        done: s.level === level + 1 ? Math.min(1, Math.max(0, (exp - here.total) / s.exp)) : 0,
      }
    })
  const goal = at(target)
  return {
    level,
    exp,
    // The level's badge (tarkov.dev's picture), "" without one.
    badge: here.badge || '',
    target,
    left: Math.max(0, goal.total - exp),
    unlocks: goal.unlocks || [],
    // How far into the level the player is, to the next (0 to 1): the
    // badge's ring, read as a level's always is, whatever level is aimed at.
    toNext: rows[0]?.done || 0,
    rows,
    widest: Math.max(...rows.map((r) => r.exp), 1),
  }
}

// allLevelsOf is every level for the levels page, from the first: each
// reached ("reached", its bar full), the one played ("now"), the next (its
// bar filled as far as the player is into it) and those ahead; what each
// takes, how much more than the one before, what is left to it, what it
// unlocks, and whether it is the level aimed at. null as ladderOf.
export function allLevelsOf(steps, level, exp, chosen = 0) {
  const ladder = ladderOf(steps, level, exp, chosen, true)
  if (!ladder) return null
  const sorted = [...steps].sort((a, b) => a.level - b.level)
  const widest = Math.max(...sorted.map((s) => s.exp), 1)
  const ahead = new Map(ladder.rows.map((r) => [r.level, r]))
  const rows = sorted.map((s, i) => {
    const before = sorted[i - 1]
    const more = before && before.level > 1 ? s.exp - before.exp : 0
    const row = ahead.get(s.level)
    if (row) return { ...row, more, state: row.done > 0 || s.level === level + 1 ? 'next' : 'ahead' }
    return {
      level: s.level,
      exp: s.exp,
      more,
      left: 0,
      unlocks: s.unlocks || [],
      done: 1,
      state: s.level === level ? 'now' : 'reached',
    }
  })
  return { ...ladder, rows, widest }
}
