package update

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
)

// channelServer serves a latest release and, when nightly is set, a nightly
// build; each carries an archive for this OS and CPU.
func channelServer(t *testing.T, stable, nightly string) *Client {
	t.Helper()
	release := func(tag, name, version string) Release {
		return Release{Tag: tag, Name: name, Assets: []Asset{{Name: ArchiveName(version, runtime.GOOS, runtime.GOARCH)}, {Name: ChecksumsAsset}}}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/ichi0g0y/mayak/releases/latest":
			json.NewEncoder(w).Encode(release("v"+stable, "MAYAK v"+stable, stable))
		case "/repos/ichi0g0y/mayak/releases/tags/nightly":
			if nightly == "" {
				http.NotFound(w, r)
				return
			}
			json.NewEncoder(w).Encode(release(NightlyTag, "MAYAK nightly "+nightly, nightly))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	client := NewClient("MAYAK/test")
	client.Mirror = ""
	client.APIBase = server.URL
	return client
}

func TestLatestForChoosesTheNewerOfReleaseAndNightly(t *testing.T) {
	for _, tc := range []struct {
		name, channel, stable, nightly, want string
	}{
		{"stable channel ignores the nightly", ChannelStable, "0.1.17", "0.1.17-16-ge057eae", "0.1.17"},
		{"nightly after the release", ChannelNightly, "0.1.17", "0.1.17-16-ge057eae", "0.1.17-16-ge057eae"},
		{"a release newer than the nightly wins", ChannelNightly, "0.1.18", "0.1.17-16-ge057eae", "0.1.18"},
		{"no nightly published", ChannelNightly, "0.1.17", "", "0.1.17"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			release, err := channelServer(t, tc.stable, tc.nightly).LatestFor(context.Background(), tc.channel)
			if err != nil {
				t.Fatal(err)
			}
			if release.Version() != tc.want {
				t.Fatalf("got %s, want %s", release.Version(), tc.want)
			}
		})
	}
}

func TestNightlyVersion(t *testing.T) {
	named := Release{Tag: NightlyTag, Name: "MAYAK nightly 0.1.17-16-ge057eae"}
	if got := named.Version(); got != "0.1.17-16-ge057eae" {
		t.Fatalf("from the name: %q", got)
	}
	unnamed := Release{Tag: NightlyTag, Assets: []Asset{{Name: "Mayak-Setup-0.1.17-16-ge057eae-windows-amd64.exe"}, {Name: "Mayak-0.1.17-16-ge057eae-darwin-arm64.tar.gz"}}}
	if got := unnamed.Version(); got != "0.1.17-16-ge057eae" {
		t.Fatalf("from an archive: %q", got)
	}
	// A nightly sorts after its release and before the next one.
	if !IsNewer("0.1.17", "0.1.17-16-ge057eae") || !IsNewer("0.1.17-16-ge057eae", "0.1.18") || !IsNewer("0.1.17-15-gaaaaaaa", "0.1.17-16-ge057eae") {
		t.Fatal("nightly ordering")
	}
}
