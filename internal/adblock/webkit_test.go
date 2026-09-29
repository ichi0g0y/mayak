package adblock

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestWebKitPatterns(t *testing.T) {
	for _, c := range []struct {
		pattern, want string
		matches       []string
		misses        []string
	}{
		{"||ads.example.com^", `^[^:]+://+([^:/]+\.)?ads\.example\.com`, []string{"https://ads.example.com/x.js", "https://a.ads.example.com/"}, []string{"https://notads.example.org/"}},
		{"/banner/*/ad_", `/banner/.*/ad_`, []string{"https://x.com/banner/1/ad_2.gif"}, []string{"https://x.com/banner/ad.gif"}},
		{"|https://track.", `^https://track\.`, []string{"https://track.example/"}, []string{"http://x/https://track."}},
		{"&ad_type=|", `&ad_type=$`, []string{"https://x/?a=1&ad_type="}, []string{"https://x/?ad_type=1"}},
		{"||cdn.example^*/ads.js", `^[^:]+://+([^:/]+\.)?cdn\.example[/:?=&].*/ads\.js`, []string{"https://cdn.example/v1/ads.js"}, []string{"https://cdn.example.org/ads.js"}},
	} {
		got, ok := webkitPattern(c.pattern)
		if !ok || got != c.want {
			t.Fatalf("%s: %q, %v", c.pattern, got, ok)
		}
		re := regexp.MustCompile(got)
		for _, u := range c.matches {
			if !re.MatchString(u) {
				t.Errorf("%s does not match %s", got, u)
			}
		}
		for _, u := range c.misses {
			if re.MatchString(u) {
				t.Errorf("%s matches %s", got, u)
			}
		}
	}
	if _, ok := webkitPattern("||例え.jp^"); ok {
		t.Error("a pattern out of ASCII was taken")
	}
}

func TestWebKitNetworkRules(t *testing.T) {
	rule, exception, ok := networkRule("||ads.example^$script,third-party,domain=site.com|~sub.site.com")
	if !ok || exception || rule.Action.Type != "block" || len(rule.Trigger.ResourceType) != 1 || rule.Trigger.ResourceType[0] != "script" ||
		rule.Trigger.LoadType[0] != "third-party" || len(rule.Trigger.IfDomain) != 1 || rule.Trigger.IfDomain[0] != "*site.com" {
		t.Fatalf("rule %+v", rule)
	}
	rule, exception, ok = networkRule("@@||cdn.example^$image")
	if !ok || !exception || rule.Action.Type != "ignore-previous-rules" {
		t.Fatalf("exception %+v", rule)
	}
	// Never the page itself unless asked; no option WebKit cannot say.
	rule, _, _ = networkRule("||tracker.example^")
	for _, typ := range rule.Trigger.ResourceType {
		if typ == "document" {
			t.Fatal("a plain rule blocks pages")
		}
	}
	for _, line := range []string{"||x.example^$redirect=noop.js", "/ads[0-9]+/", "||x.example^$popup", "@@||site.example^$document", "*$image"} {
		if _, _, ok := networkRule(line); ok {
			t.Errorf("%s was taken", line)
		}
	}
	rule, _, _ = networkRule("||x.example^$~script")
	for _, typ := range rule.Trigger.ResourceType {
		if typ == "script" {
			t.Fatal("~script still blocks scripts")
		}
	}
}

func TestWebKitSelectors(t *testing.T) {
	for _, ok := range []string{".ad-slot", "#banner", `div[id^="ad-"]`, "aside:not(.x) > .ad", `a[href*="click:here"]`} {
		if !safeSelector(ok) {
			t.Errorf("%s left out", ok)
		}
	}
	for _, bad := range []string{".x:has(.ad)", "div:-abp-contains(Ad)", ".x::before", "+js(noeval)", ".x { color: red }", "span:contains(広告)"} {
		if safeSelector(bad) {
			t.Errorf("%s taken", bad)
		}
	}
}

func TestWebKitRulesFromLists(t *testing.T) {
	dir := t.TempDir()
	list := "! comment\n||ads.example^\n##.ad-slot\nsite.com##.sponsor\nsite.com#@#.ad-slot\n@@||ads.example/ok.js\n/regex[0-9]/\n"
	if err := os.WriteFile(filepath.Join(dir, "easylist.txt"), []byte(list), 0o600); err != nil {
		t.Fatal(err)
	}
	b := New(dir, func(string, string) {})
	b.lists = []List{{ID: 1, Name: "easylist"}}
	all, network, version := b.WebKitRules()
	var rules, net []webkitRule
	if json.Unmarshal([]byte(all), &rules) != nil || json.Unmarshal([]byte(network), &net) != nil || version == "" {
		t.Fatalf("rules %s", all)
	}
	kinds := []string{}
	for _, r := range rules {
		kinds = append(kinds, r.Action.Type)
	}
	// Blocks, then hiding, then exceptions, and the exempt sites last.
	want := []string{"block", "css-display-none", "css-display-none", "ignore-previous-rules", "ignore-previous-rules"}
	if len(kinds) != len(want) {
		t.Fatalf("kinds %v", kinds)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("kinds %v", kinds)
		}
	}
	if last := rules[len(rules)-1]; last.Trigger.IfDomain[0] != "*tarkov.dev" {
		t.Fatalf("last rule %+v", last)
	}
	if len(net) != 3 {
		t.Fatalf("network only: %d rules", len(net))
	}
	if all2, _, version2 := b.WebKitRules(); all2 != all || version2 != version {
		t.Fatal("the same lists gave other rules")
	}
}
