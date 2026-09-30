package profiledetect

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// Stats are the numbers beside the level: raids (R), kills (K), the
// survival rate (S/R, percent), K/D and the time online (hours). A number
// not read is zero. (The leave rate is left out: OCR reads its "0%" as
// anything but.)
type Stats struct {
	Raids, Kills            int
	SurvivalRate, KD, Hours float64
}

var (
	percent   = regexp.MustCompile(`(\d{1,3})\s*%`)
	decimal   = regexp.MustCompile(`\d+[.,]\d+`)
	hours     = regexp.MustCompile(`(\d+[.,]\d+)\s*[hHч時]`)
	nickname  = regexp.MustCompile(`^[A-Za-z0-9_-]{3,15}$`)
	factionRe = regexp.MustCompile(`(?i)\b(BEAR|USEC)\b`)
)

// Level reads the level's number: 1 to 79, else 0.
func Level(text string) int {
	digits := onlyDigits(text)
	if n, err := strconv.Atoi(digits); err == nil && n >= 1 && n <= 79 {
		return n
	}
	return 0
}

// Experience reads a row of the experience (digits in groups, maybe after
// the edition's badge, "EXP+"): its number, and whether the row is one (at
// least four digits, most of what was read).
func Experience(text string) (int, bool) {
	digits := onlyDigits(text)
	letters := 0
	for _, r := range text {
		if unicode.IsLetter(r) {
			letters++
		}
	}
	if len(digits) < 4 || len(digits) > 10 || letters > len(digits)/2+4 {
		return 0, false
	}
	n, err := strconv.Atoi(digits)
	return n, err == nil
}

// Nickname reads a row of the nickname: its first word that can be an EFT
// nickname (3 to 15 letters, digits, "_" or "-"), past the icon before it.
func Nickname(text string) string {
	for _, word := range strings.Fields(text) {
		word = strings.TrimLeftFunc(word, func(r rune) bool { return !isNameRune(r) })
		if nickname.MatchString(word) && strings.ContainsFunc(word, unicode.IsLetter) {
			return word
		}
	}
	return ""
}

func isNameRune(r rune) bool {
	return r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-')
}

// Faction reads a faction's name: BEAR, USEC, or "".
func Faction(text string) string {
	if m := factionRe.FindStringSubmatch(text); m != nil {
		return strings.ToUpper(m[1])
	}
	return ""
}

// ReadStats reads the lines beside the level as one OCR engine read them:
// on the left the raids and the kills (the digits alone: an engine may read
// "462" as "46.2"); on the right the survival rate (a percentage up to 100),
// K/D (a number with a decimal point) and the time online (a number with a
// decimal point before "h", on the third line or, when its label takes the
// line, the fourth). The labels are the game's language: only the numbers
// and their units are read.
func ReadStats(left, right []string) Stats {
	var s Stats
	line := func(lines []string, i int) string {
		if i < len(lines) {
			return lines[i]
		}
		return ""
	}
	count := func(text string) int {
		digits := onlyDigits(text)
		if len(digits) == 0 || len(digits) > 6 {
			return 0
		}
		n, _ := strconv.Atoi(digits)
		return n
	}
	s.Raids = count(line(left, 0))
	s.Kills = count(line(left, 1))
	if m := percent.FindStringSubmatch(line(right, 0)); m != nil {
		if v := float(m[1]); v <= 100 {
			s.SurvivalRate = v
		}
	}
	s.KD = float(decimal.FindString(line(right, 1)))
	if m := hours.FindStringSubmatch(line(right, 2) + " " + line(right, 3)); m != nil {
		s.Hours = float(m[1])
	}
	return s
}

// Vote is the value most readings agree on, among those read (not zero):
// OCR engines misread different glyphs, so what two agree on is the value.
// A tie goes to the earlier reading.
func Vote[T comparable](readings []T) T {
	var zero, best T
	most := 0
	for i, v := range readings {
		if v == zero {
			continue
		}
		n := 0
		for _, w := range readings[i:] {
			if w == v {
				n++
			}
		}
		if n > most {
			best, most = v, n
		}
	}
	return best
}

// VoteStats votes each of the stats among the engines' readings.
func VoteStats(readings []Stats) Stats {
	pick := func(get func(Stats) float64) float64 {
		values := make([]float64, len(readings))
		for i, r := range readings {
			values[i] = get(r)
		}
		return Vote(values)
	}
	return Stats{
		Raids:        int(pick(func(s Stats) float64 { return float64(s.Raids) })),
		Kills:        int(pick(func(s Stats) float64 { return float64(s.Kills) })),
		SurvivalRate: pick(func(s Stats) float64 { return s.SurvivalRate }),
		KD:           pick(func(s Stats) float64 { return s.KD }),
		Hours:        pick(func(s Stats) float64 { return s.Hours }),
	}
}

func float(s string) float64 {
	f, _ := strconv.ParseFloat(strings.ReplaceAll(s, ",", "."), 64)
	return f
}

func onlyDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// LevelOf is the level a total of experience makes, from each level's
// experience over the one before it (tarkov.dev's playerLevels, level 1
// first): 0 when there is no table.
func LevelOf(exp int, steps []int) int {
	total, level := 0, 0
	for i, step := range steps {
		total += step
		if exp < total {
			break
		}
		level = i + 1
	}
	return level
}
