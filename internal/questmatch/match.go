package questmatch

import (
	"regexp"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

type Quest struct {
	ID, Name, Trader, Map, NormalizedName string
	Aliases                               []string
}
type Result struct {
	Quest      Quest
	Confidence float64
}

var nonWord = regexp.MustCompile(`[^\p{L}\p{N}]+`)
var partThree = regexp.MustCompile(`(?i)(part\s+)ろ($|\s)`)

func Normalize(s string) string {
	s = strings.ToLower(norm.NFKC.String(s))
	// Preserve Japanese ろ; only the English Part suffix is a digit confusion.
	s = partThree.ReplaceAllString(s, "${1}3${2}")
	s = strings.NewReplacer("0", "o", "1", "i", "|", "i").Replace(s)
	s = strings.TrimSpace(nonWord.ReplaceAllString(s, " "))
	// Windows OCR inserts spaces between Japanese glyphs. Keep English word boundaries.
	runes := []rune(s)
	var out strings.Builder
	for i, r := range runes {
		if r == ' ' && i > 0 && i+1 < len(runes) && (japanese(runes[i-1]) || japanese(runes[i+1])) {
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}
func japanese(r rune) bool { return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana) }
func Match(raw string, quests []Quest) []Result {
	if Normalize(raw) == "" {
		return nil
	}
	out := make([]Result, 0, len(quests))
	for _, q := range quests {
		score := Similarity(raw, q.Name)
		for _, alias := range q.Aliases {
			if candidate := Similarity(raw, alias); candidate > score {
				score = candidate
			}
		}
		out = append(out, Result{q, score})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Confidence == out[j].Confidence {
			return out[i].Quest.Name < out[j].Quest.Name
		}
		return out[i].Confidence > out[j].Confidence
	})
	return out
}

// Similarity returns an OCR-tolerant normalized score between two names.
func Similarity(raw, targetName string) float64 {
	n := Normalize(raw)
	target := Normalize(targetName)
	if n == "" || target == "" {
		return 0
	}
	queries := []string{n}
	// A crop edge or the inspect magnifier icon can be read as a standalone I,
	// or as a few short scraps (". だ ん") before the title: try without up to
	// three leading tokens of one or two characters.
	if strings.HasPrefix(n, "i ") {
		queries = append(queries, strings.TrimSpace(strings.TrimPrefix(n, "i ")))
	}
	fields := strings.Fields(raw)
	for k := 1; k <= 3 && k < len(fields) && len([]rune(fields[k-1])) <= 2; k++ {
		if rest := Normalize(strings.Join(fields[k:], " ")); rest != "" {
			queries = append(queries, rest)
		}
	}
	score := 0.0
	for queryIndex, query := range queries {
		maxLength := len([]rune(query))
		if targetLength := len([]rune(target)); targetLength > maxLength {
			maxLength = targetLength
		}
		candidate := 1 - distance(query, target)/float64(maxLength)
		if queryIndex > 0 {
			candidate -= .001
		}
		if candidate > score {
			score = candidate
		}
		// Windows OCR can drop the beginning of text that follows a vertical
		// table divider. A long contained fragment is still strong evidence, but
		// short generic fragments such as "Part 1" must not auto-match.
		queryLength := len([]rune(query))
		targetLength := len([]rune(target))
		if queryLength >= 7 && strings.Contains(target, query) {
			partial := .84 + .12*float64(queryLength)/float64(targetLength)
			if partial > score {
				score = partial
			}
		}
		// The other way round, the game can add to a title (a weapon's own name
		// after its item name): a reading that starts with the whole name is
		// strong evidence for it, more than a similar name of the same length.
		if targetLength >= 7 && strings.HasPrefix(query, target+" ") {
			full := .84 + .12*float64(targetLength)/float64(queryLength)
			if full > score {
				score = full
			}
		}
	}
	return score
}
func distance(a, b string) float64 {
	x, y := []rune(a), []rune(b)
	prev := make([]float64, len(y)+1)
	for j := range prev {
		prev[j] = float64(j)
	}
	for i, ca := range x {
		cur := make([]float64, len(y)+1)
		cur[0] = float64(i + 1)
		for j, cb := range y {
			cost := substitutionCost(ca, cb)
			cur[j+1] = min(cur[j]+1, prev[j+1]+1, prev[j]+cost)
		}
		prev = cur
	}
	return prev[len(y)]
}
func substitutionCost(a, b rune) float64 {
	if a == b {
		return 0
	}
	// Frequent OCR confusions in EFT's narrow uppercase UI font.
	if (a == 'i' && b == 'l') || (a == 'l' && b == 'i') {
		return .2
	}
	return 1
}
func min(v ...float64) float64 {
	m := v[0]
	for _, x := range v[1:] {
		if x < m {
			m = x
		}
	}
	return m
}
