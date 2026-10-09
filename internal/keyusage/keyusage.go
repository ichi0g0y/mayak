// Package keyusage tells keys with no use from the official EFT Wiki: a key
// whose door is always unlocked, or one that opens nothing. tarkov.dev's
// maps place the locks a key opens, but a key with no place there may still
// have a use it lacks (Labs keycards, safes, car keys), so its places alone
// cannot say; the wiki's key pages say it in words ("|usage =" in their
// infobox, "|node =" the item's id, the same as tarkov.dev's).
package keyusage

import (
	"regexp"
	"strings"
)

// Kind is what a key's wiki usage says of it.
type Kind string

const (
	// Unclassified is a usage that says neither: the key shows nothing.
	Unclassified Kind = ""
	// AlwaysOpen is a key whose door or safe is always unlocked.
	AlwaysOpen Kind = "open"
	// OpensNothing is a key that opens no lock.
	OpensNothing Kind = "none"
)

// The wordings seen on 2026-10-08 (26 of 247 key pages): "Room 323 is always
// unlocked", "the safe at the gas station is always unlocked"; "This key
// does currently not open any lock", "This keycard does not open any lock",
// "This key has no usage". Close variants still match; anything else stays
// unclassified, as no note is better than a wrong one.
var (
	alwaysOpen   = regexp.MustCompile(`\balways (?:unlocked|open)\b`)
	opensNothing = regexp.MustCompile(`\b(?:does(?:n't| not)(?: currently)?|does currently not|currently does not|cannot|can't)\s+(?:open|unlock)\s+any\b|\b(?:has )?no (?:known )?(?:usage|use)\b`)
	// wikiLink is "[[Page]]" or "[[Page|text]]".
	wikiLink = regexp.MustCompile(`\[\[(?:[^\]|]*\|)?([^\]]*)\]\]`)
)

// Classify reads a key's wiki usage. "Always unlocked" wins over "no usage"
// (the Health Resort keys say both).
func Classify(usage string) Kind {
	text := strings.ToLower(wikiLink.ReplaceAllString(usage, "$1"))
	text = strings.Join(strings.Fields(strings.ReplaceAll(text, "’", "'")), " ")
	switch {
	case alwaysOpen.MatchString(text):
		return AlwaysOpen
	case opensNothing.MatchString(text):
		return OpensNothing
	}
	return Unclassified
}

var (
	nodeField  = regexp.MustCompile(`(?i)\|\s*node\s*=\s*([0-9a-f]{24})\b`)
	usageField = regexp.MustCompile(`(?is)\|\s*usage\s*=(.*?)(?:\n\s*\||\n\s*\}\})`)
)

// parsePage reads a key page's item id and usage from its wikitext ("" when
// it has none).
func parsePage(content string) (node, usage string) {
	if m := nodeField.FindStringSubmatch(content); m != nil {
		node = strings.ToLower(m[1])
	}
	if m := usageField.FindStringSubmatch(content); m != nil {
		usage = strings.TrimSpace(m[1])
	}
	return node, usage
}
