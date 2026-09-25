package app

import (
	"net/url"
	"testing"
)

func TestQuestPageDestinations(t *testing.T) {
	dev := "https://tarkov.dev/task/debut"
	for _, tc := range []struct{ site, want string }{
		{"", dev}, {"unknown", dev}, {"tarkov-dev", dev},
		{"official-wiki", "https://escapefromtarkov.fandom.com/wiki/Debut"},
		{"japanese-wiki", "https://wikiwiki.jp/eft/Prapor/Debut"},
	} {
		if got := questPageURL(tc.site, "Debut", "Prapor", dev); got != tc.want {
			t.Fatalf("%s: %s", tc.site, got)
		}
	}
	for _, site := range []string{"official-wiki", "japanese-wiki"} {
		parsed, err := url.Parse(questPageURL(site, "A/B #1?", "Prapor", dev))
		if err != nil || parsed.Fragment != "" || parsed.RawQuery != "" {
			t.Fatal("task name changed URL structure", err)
		}
	}
}
