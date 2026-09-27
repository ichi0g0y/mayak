// Package sysfonts lists the font families installed on this computer, for
// the snap notes' text.
package sysfonts

import (
	"sort"
	"strings"
)

// clean sorts the names, without duplicates, the vertical variants of CJK
// fonts ("@MS Gothic") and blanks.
func clean(names []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" || strings.HasPrefix(name, "@") || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i]) < strings.ToLower(out[j]) })
	return out
}
