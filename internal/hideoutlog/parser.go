package hideoutlog

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

const maxBuffer = 256 << 10

var logStamp = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\.\d{3})`)
var endpoint = regexp.MustCompile(`/client/hideout/[a-zA-Z0-9/_-]+`)
var safeAction = regexp.MustCompile(`^Hideout[A-Za-z0-9_]{1,80}$`)

func LogTime(line string) time.Time {
	match := logStamp.FindString(line)
	t, _ := time.ParseInLocation("2006-01-02 15:04:05.000", match, time.Local)
	return t
}

type Parser struct {
	buffer  string
	payload []byte
	depth   int
	quoted  bool
	escaped bool
	failed  bool
	stamp   time.Time
}

// Feed preserves partial lines and JSON documents across arbitrary read boundaries.
func (p *Parser) Feed(chunk string) []Event {
	var out []Event
	for len(chunk) > 0 {
		n := len(chunk)
		if n > 8192 {
			n = 8192
		}
		p.buffer += chunk[:n]
		chunk = chunk[n:]
		for {
			end := strings.IndexByte(p.buffer, '\n')
			if end < 0 {
				break
			}
			line := strings.TrimSuffix(p.buffer[:end], "\r")
			p.buffer = p.buffer[end+1:]
			out = append(out, p.line(line)...)
		}
		if len(p.buffer) > maxBuffer {
			p.buffer = ""
			p.payload = nil
			p.depth = 0
			p.quoted = false
			p.escaped = false
			p.failed = false
		}
	}

	// A JSON document may finish before EFT appends its trailing newline.
	tail := strings.TrimSpace(p.buffer)
	if len(p.payload) > 0 && (strings.HasSuffix(tail, "}") || strings.HasSuffix(tail, "]")) {
		candidate := append(append([]byte(nil), p.payload...), p.buffer...)
		if json.Valid(candidate) {
			line := p.buffer
			p.buffer = ""
			out = append(out, p.line(line)...)
		}
	}
	return out
}

func (p *Parser) line(line string) []Event {
	stamp := LogTime(line)
	if !stamp.IsZero() {
		p.stamp = stamp
		p.payload = nil
		p.depth = 0
		p.quoted = false
		p.escaped = false
		p.failed = false
	}
	if strings.Contains(line, "Inventory queue failed on the following commands") {
		p.failed = true
		if i := strings.Index(line, "commands:"); i >= 0 {
			line = strings.TrimSpace(line[i+len("commands:"):])
		} else {
			return nil
		}
	}
	if path := endpoint.FindString(line); path != "" {
		kind, status := "metadata-request", "info"
		lower := strings.ToLower(line)
		prefix := strings.ToLower(line[:strings.Index(line, path)])
		if strings.Contains(prefix, "|error|") || strings.Contains(prefix, "error:") || strings.Contains(prefix, "failed") {
			kind, status = "request-failed", "failed"
		} else if strings.Contains(lower, "response") {
			kind = "metadata-response"
		}
		return []Event{{Kind: kind, Action: path, AreaType: -1, OccurredAt: p.stamp, Status: status, Confidence: "observed"}}
	}
	if len(p.payload) == 0 {
		i := strings.IndexAny(line, "[{")
		if i < 0 {
			return nil
		}
		line = line[i:]
	}
	p.payload = append(p.payload, line...)
	p.payload = append(p.payload, '\n')
	for _, ch := range line {
		if p.quoted {
			if p.escaped {
				p.escaped = false
			} else if ch == '\\' {
				p.escaped = true
			} else if ch == '"' {
				p.quoted = false
			}
			continue
		}
		switch ch {
		case '"':
			p.quoted = true
		case '{', '[':
			p.depth++
		case '}', ']':
			p.depth--
		}
	}
	if len(p.payload) > maxBuffer {
		p.payload = nil
		p.depth = 0
		p.quoted = false
		p.escaped = false
		p.failed = false
		return nil
	}
	if p.depth > 0 || p.quoted {
		return nil
	}
	if !json.Valid(p.payload) {
		p.payload = nil
		p.depth = 0
		p.failed = false
		return nil
	}
	var raw any
	_ = json.Unmarshal([]byte(p.payload), &raw)
	p.payload = nil
	p.depth = 0
	p.quoted = false
	p.escaped = false
	var out []Event
	var visit func(any)
	visit = func(value any) {
		switch v := value.(type) {
		case []any:
			for _, item := range v {
				visit(item)
			}
		case map[string]any:
			if action, ok := v["Action"].(string); ok && safeAction.MatchString(action) {
				e := Event{Kind: "unknown-action", Action: action, AreaType: -1, OccurredAt: p.stamp, Status: "unknown", Confidence: "unconfirmed"}
				if area, ok := v["areaType"].(float64); ok {
					e.AreaType = int(area)
				}
				if ts, ok := v["timestamp"].(float64); ok {
					e.ActionTimestamp = int64(ts)
				}
				if p.failed {
					e.Kind = "queue-failed"
					e.Status = "failed"
					e.Confidence = "observed"
					if action == "HideoutUpgradeComplete" {
						e.Kind = "upgrade-failed"
					}
				}
				out = append(out, e)
			}
			for _, nested := range v {
				switch nested.(type) {
				case []any, map[string]any:
					visit(nested)
				}
			}
		}
	}
	visit(raw)
	p.failed = false
	return out
}
