package trackerlog

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var versionPattern = regexp.MustCompile(`(?i)^log_\d{4}\.\d{2}\.\d{2}_\d{2}-\d{2}-\d{2}_(.+)$`)

type HistoryBreakpoint struct {
	ID      string
	Version string
	StartAt time.Time
}

// HistoryBreakpoints returns the first matching session for each EFT build.
// A profile identity must match all three values so logs from another account
// or progression mode can never be written to the selected Tracker profile.
func HistoryBreakpoints(root, accountID, profileID, mode string) []HistoryBreakpoint {
	sessions := matchingSessions(root, accountID, profileID, mode)
	seen := make(map[string]bool)
	result := make([]HistoryBreakpoint, 0)
	for _, session := range sessions {
		version := sessionVersion(filepath.Base(session.path))
		if seen[version] {
			continue
		}
		seen[version] = true
		result = append(result, HistoryBreakpoint{ID: filepath.Base(session.path), Version: version, StartAt: session.start})
	}
	return result
}

// TaskHistory returns the last observed state for every task from the selected
// breakpoint onward. It reads only EFT-created log files and does not modify them.
func TaskHistory(root, breakpointID, accountID, profileID, mode string) (map[string]string, error) {
	if filepath.Base(breakpointID) != breakpointID || breakpointID == "." || breakpointID == "" {
		return nil, errors.New("invalid log breakpoint")
	}
	sessions := matchingSessions(root, accountID, profileID, mode)
	start := -1
	for index, session := range sessions {
		if filepath.Base(session.path) == breakpointID {
			start = index
			break
		}
	}
	if start < 0 {
		return nil, errors.New("log breakpoint does not belong to this EFT profile")
	}
	return sessionTaskStates(sessions[start:]), nil
}

// ProfileTaskHistory returns the last observed state for every task of a
// profile, from its first session on, how many sessions it read, and the
// last Prestige its logs record in mode. A Prestige resets the progress of
// the same profile (its ID stays), so the task changes before it are left
// out, and so are those before from (a day set by hand: a Prestige whose
// logs are not on this PC). It finds the profile's sessions once
// (HistoryBreakpoints then TaskHistory would read every log twice).
func ProfileTaskHistory(root, accountID, profileID, mode string, from time.Time) (map[string]string, int, time.Time) {
	sessions := matchingSessions(root, accountID, profileID, mode)
	var prestige time.Time
	var changes []Event
	for _, session := range sessions {
		text := sessionNotifications(session)
		for _, event := range Prestiges(text) {
			if event.Mode == mode && event.At.After(prestige) {
				prestige = event.At
			}
		}
		parser := TaskParser{}
		for _, event := range parser.Feed(text) {
			if event.Kind != TaskChanged || !session.played(event.At) {
				continue
			}
			// A notification whose line had no stamp is the session's.
			if event.At.IsZero() {
				event.At = session.start
			}
			changes = append(changes, event)
		}
	}
	cutoff := from
	if prestige.After(cutoff) {
		cutoff = prestige
	}
	states := make(map[string]string)
	for _, event := range changes {
		if !event.At.Before(cutoff) {
			states[event.TaskID] = event.TaskState
		}
	}
	return states, len(sessions), prestige
}

// sessionTaskStates reads the task changes of sessions, in order: a later
// state of a task replaces an earlier one.
func sessionTaskStates(sessions []historySession) map[string]string {
	states := make(map[string]string)
	for _, session := range sessions {
		parser := TaskParser{}
		for _, event := range parser.Feed(sessionNotifications(session)) {
			if event.Kind == TaskChanged && session.played(event.At) {
				states[event.TaskID] = event.TaskState
			}
		}
	}
	return states
}

// sessionNotifications is the text of a session's log with its
// notifications: the output log has them both ways (TaskParser) and the
// Prestiges; the push-notifications log only one way, which a session may
// lack.
func sessionNotifications(session historySession) string {
	path := findLog(session.path, "output")
	if path == "" {
		path = findLog(session.path, "push-notifications")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

type historySession struct {
	path  string
	start time.Time
	// spans are when the profile was the one played in the session, and
	// others whether the session played another one too (a session can
	// switch mode: PvE, then Seasonal, then PvP).
	spans  []identitySpan
	others bool
}

// identitySpan is a time of a session in which one profile was played:
// from its profile line to the next mode line or another profile's line
// (to zero: the session's end).
type identitySpan struct{ from, to time.Time }

// played reports whether a notification at at belongs to the session's
// profile. One with no stamp does when the session played no other.
func (s historySession) played(at time.Time) bool {
	if at.IsZero() {
		return !s.others
	}
	for _, span := range s.spans {
		if !at.Before(span.from) && (span.to.IsZero() || at.Before(span.to)) {
			return true
		}
	}
	return false
}

func matchingSessions(root, accountID, profileID, mode string) []historySession {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	result := make([]historySession, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		application := findLog(dir, "application")
		data, readErr := os.ReadFile(application)
		if readErr != nil {
			continue
		}
		spans, others := identitySpans(string(data), accountID, profileID, mode)
		if len(spans) == 0 {
			continue
		}
		start := entryTime(entry.Name())
		if start.IsZero() {
			if info, infoErr := entry.Info(); infoErr == nil {
				start = info.ModTime()
			}
		}
		result = append(result, historySession{path: dir, start: start, spans: spans, others: others})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].start.Before(result[j].start) })
	return result
}

// identitySpans reads an application log for when the identity was played,
// and whether another identity was played too, as IdentityParser reads
// it: a mode line, then the profile lines of that mode.
func identitySpans(text, accountID, profileID, mode string) (spans []identitySpan, others bool) {
	want := normalizeMode(mode)
	var current, profile, account string
	var at time.Time
	open := false
	for _, line := range strings.Split(text, "\n") {
		if stamp := lineTime(line); !stamp.IsZero() {
			at = stamp
		}
		if match := modePattern.FindStringSubmatch(line); len(match) == 2 {
			current, profile, account = normalizeMode(match[1]), "", ""
		} else if match := profilePattern.FindStringSubmatch(line); len(match) == 3 {
			profile, account = match[1], match[2]
		} else {
			continue
		}
		played := current == want && profile == profileID && account == accountID
		if played && !open {
			spans, open = append(spans, identitySpan{from: at}), true
		}
		if !played && open {
			spans[len(spans)-1].to, open = at, false
		}
		if !played && current != "" && profile != "" && account != "" {
			others = true
		}
	}
	return spans, others
}

func sessionVersion(name string) string {
	match := versionPattern.FindStringSubmatch(name)
	if len(match) == 2 {
		return match[1]
	}
	return "unknown"
}

func entryTime(name string) time.Time {
	prefix := strings.TrimPrefix(name, "log_")
	parts := strings.SplitN(prefix, "_", 3)
	if len(parts) < 2 {
		return time.Time{}
	}
	stamp, _ := time.ParseInLocation("2006.01.02_15-04-05", parts[0]+"_"+parts[1], time.Local)
	return stamp
}
