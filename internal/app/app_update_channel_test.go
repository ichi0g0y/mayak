package app

import (
	"testing"

	"github.com/local/mayak/internal/update"
)

// A nightly that finished downloading after the channel went back to stable
// is not installed; a release is, on either channel.
func TestStagedForChannel(t *testing.T) {
	nightly := update.Staged{Version: "0.1.17-34-g11b21c0", Tag: update.NightlyTag}
	release := update.Staged{Version: "0.1.18", Tag: "v0.1.18"}
	if stagedForChannel(nightly, update.ChannelStable) {
		t.Fatal("a nightly would install on the stable channel")
	}
	if !stagedForChannel(nightly, update.ChannelNightly) || !stagedForChannel(release, update.ChannelStable) || !stagedForChannel(release, update.ChannelNightly) {
		t.Fatal("an allowed update is refused")
	}
}
