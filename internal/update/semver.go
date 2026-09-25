package update

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Semver is a release version: MAJOR.MINOR.PATCH, an optional pre-release
// ("beta.1", which sorts before the release) and an optional commit count
// after the release tag, as `git describe` writes it ("0.1.0-3-g1a2b3c4",
// which sorts after the release).
type Semver struct {
	Major, Minor, Patch int
	Pre                 string
	Post                int
}

var describeSuffix = regexp.MustCompile(`^(\d+)-g[0-9a-f]+$`)

// ParseSemver reads "v1.2.3", "1.2.3", "1.2.3-beta.1" and "1.2.3-4-gabcdef0".
func ParseSemver(value string) (Semver, error) {
	raw := strings.TrimPrefix(strings.TrimSpace(value), "v")
	if raw == "" {
		return Semver{}, fmt.Errorf("empty version")
	}
	core, suffix, _ := strings.Cut(raw, "-")
	if build := strings.Index(suffix, "+"); build >= 0 {
		suffix = suffix[:build]
	}
	if build := strings.Index(core, "+"); build >= 0 {
		core = core[:build]
	}
	parts := strings.Split(core, ".")
	if len(parts) < 2 || len(parts) > 3 {
		return Semver{}, fmt.Errorf("not a version: %q", value)
	}
	var numbers [3]int
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return Semver{}, fmt.Errorf("not a version: %q", value)
		}
		numbers[i] = n
	}
	version := Semver{Major: numbers[0], Minor: numbers[1], Patch: numbers[2]}
	if match := describeSuffix.FindStringSubmatch(suffix); match != nil {
		version.Post, _ = strconv.Atoi(match[1])
	} else {
		version.Pre = suffix
	}
	return version, nil
}

// Compare orders versions: -1 when a is older than b, 0 when equal, 1 when newer.
func Compare(a, b Semver) int {
	for _, pair := range [][2]int{{a.Major, b.Major}, {a.Minor, b.Minor}, {a.Patch, b.Patch}, {a.Post, b.Post}} {
		if pair[0] != pair[1] {
			if pair[0] < pair[1] {
				return -1
			}
			return 1
		}
	}
	switch {
	case a.Pre == b.Pre:
		return 0
	case a.Pre == "":
		return 1
	case b.Pre == "":
		return -1
	}
	return strings.Compare(a.Pre, b.Pre)
}

// String writes the version back ("1.2.3", "1.2.3-beta.1", "1.2.3+3").
func (v Semver) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Pre != "" {
		s += "-" + v.Pre
	}
	if v.Post > 0 {
		s += fmt.Sprintf("+%d", v.Post)
	}
	return s
}

// IsNewer reports whether candidate is a newer release than current. A
// current that is not a version (a development build) counts as older than
// every release.
func IsNewer(current, candidate string) bool {
	next, err := ParseSemver(candidate)
	if err != nil {
		return false
	}
	now, err := ParseSemver(current)
	if err != nil {
		return true
	}
	return Compare(next, now) > 0
}
