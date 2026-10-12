package app

import (
	"strings"
	"unicode"
)

// browserLinkLogMax is the longest line the shell may log about its link.
const browserLinkLogMax = 200

// BrowserLinkLog writes a line from the shell's Host–Client link
// (transport.js) to the log: a detection a Client got and how late, a quiet
// link made again. It tells a detection that came late from one that never
// came. Control characters are dropped and the line is cut short.
func (a *App) BrowserLinkLog(message string) {
	message = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, message)
	if r := []rune(message); len(r) > browserLinkLogMax {
		message = string(r[:browserLinkLogMax])
	}
	if message = strings.TrimSpace(message); message != "" {
		a.addLog("Debug", "Link", message)
	}
}
