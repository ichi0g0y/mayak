package app

import "testing"

// TarkovTracker counts as reset after a Prestige when its completed tasks
// fell to under half of those at the Prestige (353 → a few done since).
func TestTrackerWasReset(t *testing.T) {
	for _, tc := range []struct {
		before, completed int
		want              bool
	}{
		{353, 3, true}, {353, 0, true}, {353, 176, true},
		{353, 177, false}, {353, 353, false}, {353, 360, false},
		// Too few to tell: the settings page tells instead.
		{0, 0, false}, {10, 1, false},
	} {
		if got := trackerWasReset(tc.before, tc.completed); got != tc.want {
			t.Fatalf("%d → %d: %v", tc.before, tc.completed, got)
		}
	}
}
