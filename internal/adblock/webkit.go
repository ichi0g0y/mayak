package adblock

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"regexp"
	"sort"
	"strings"
)

// The Mac's web view (WKWebView) cannot be asked about each request as
// WebView2 is (filter_windows.go), so there the lists become a WebKit content
// blocker (WKContentRuleList): network rules that WebKit's regular expressions
// can say, element hiding with selectors WebKit is sure to take, the
// exceptions after them, and the exempt sites last.

// maxWebKitRules stays under WebKit's limit of 150,000 rules in one list.
const maxWebKitRules = 140000

// cssBatch is how many selectors one hiding rule takes.
const cssBatch = 200

type webkitTrigger struct {
	URLFilter     string   `json:"url-filter"`
	CaseSensitive bool     `json:"url-filter-is-case-sensitive,omitempty"`
	ResourceType  []string `json:"resource-type,omitempty"`
	LoadType      []string `json:"load-type,omitempty"`
	IfDomain      []string `json:"if-domain,omitempty"`
	UnlessDomain  []string `json:"unless-domain,omitempty"`
}

type webkitAction struct {
	Type     string `json:"type"`
	Selector string `json:"selector,omitempty"`
}

type webkitRule struct {
	Trigger webkitTrigger `json:"trigger"`
	Action  webkitAction  `json:"action"`
}

// The resource types a rule without one takes: all but the page itself
// (a page opened on purpose is never blocked) and popups.
var webkitSubresources = []string{"image", "style-sheet", "script", "font", "raw", "svg-document", "media"}

var webkitTypes = map[string][]string{
	"script": {"script"}, "image": {"image"}, "stylesheet": {"style-sheet"}, "font": {"font"},
	"media": {"media"}, "xmlhttprequest": {"raw"}, "xhr": {"raw"}, "websocket": {"raw"},
	"ping": {"raw"}, "other": {"raw"}, "object": {"media"}, "subdocument": {"document"},
	"frame": {"document"},
}

// Options a rule may carry and still be said to WebKit; any other (redirect,
// removeparam, csp, …) leaves the rule out.
var webkitOptions = map[string]bool{
	"third-party": true, "3p": true, "first-party": true, "1p": true, "domain": true,
	"important": true, "match-case": true, "all": true,
}

// WebKitRules returns the lists as a WebKit content blocker (JSON) with every
// rule, and one with the network rules only (should WebKit refuse a
// selector), and a version that changes when either does. It is empty until
// the lists are there.
func (b *Blocker) WebKitRules() (all, network, version string) {
	var texts [][]byte
	for _, list := range b.lists {
		if text, err := os.ReadFile(b.path(list)); err == nil {
			texts = append(texts, text)
		}
	}
	if len(texts) == 0 {
		return "", "", ""
	}
	blocks, hides, exceptions := convertLists(texts)
	tail := append(exceptions, exemptRule())
	// Hiding gives way first when there are too many.
	if room := maxWebKitRules - len(tail); len(blocks) > room {
		blocks, hides = blocks[:room], nil
	} else if len(blocks)+len(hides) > room {
		hides = hides[:room-len(blocks)]
	}
	encode := func(parts ...[]webkitRule) string {
		var rules []webkitRule
		for _, p := range parts {
			rules = append(rules, p...)
		}
		out, _ := json.Marshal(rules)
		return string(out)
	}
	all = encode(blocks, hides, tail)
	network = encode(blocks, tail)
	sum := sha256.Sum256([]byte(all))
	return all, network, hex.EncodeToString(sum[:8])
}

// exemptRule lets the exempt sites (tarkov.dev) be, whatever came before.
func exemptRule() webkitRule {
	var domains []string
	for _, site := range Exempt {
		domains = append(domains, "*"+site)
	}
	return webkitRule{Trigger: webkitTrigger{URLFilter: ".*", IfDomain: domains}, Action: webkitAction{Type: "ignore-previous-rules"}}
}

// convertLists turns filter list texts into block rules, element hiding
// rules and exceptions, in that order of use.
func convertLists(texts [][]byte) (blocks, hides, exceptions []webkitRule) {
	generic := []string{}
	specific := map[string][]string{} // "a.com,~b.com" → selectors
	seenSelector := map[string]bool{}
	for _, text := range texts {
		for _, line := range strings.Split(string(text), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || line[0] == '!' || line[0] == '[' {
				continue
			}
			if i := strings.Index(line, "##"); i >= 0 {
				domains, selector := line[:i], strings.TrimSpace(line[i+2:])
				if !safeSelector(selector) {
					continue
				}
				if domains == "" {
					if !seenSelector[selector] {
						seenSelector[selector] = true
						generic = append(generic, selector)
					}
				} else {
					specific[domains] = append(specific[domains], selector)
				}
				continue
			}
			// Other cosmetic kinds (#@#, #?#, #$#, #%#, $$) are not for WebKit.
			if strings.Contains(line, "#@#") || strings.Contains(line, "#?#") || strings.Contains(line, "#$#") ||
				strings.Contains(line, "#%#") || strings.Contains(line, "$$") || strings.Contains(line, "$@$") {
				continue
			}
			rule, exception, ok := networkRule(line)
			if !ok {
				continue
			}
			if exception {
				exceptions = append(exceptions, rule)
			} else {
				blocks = append(blocks, rule)
			}
		}
	}
	for i := 0; i < len(generic); i += cssBatch {
		end := min(i+cssBatch, len(generic))
		hides = append(hides, webkitRule{Trigger: webkitTrigger{URLFilter: ".*"}, Action: webkitAction{Type: "css-display-none", Selector: strings.Join(generic[i:end], ", ")}})
	}
	keys := make([]string, 0, len(specific))
	for k := range specific {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		ifDomain, unlessDomain, ok := webkitDomains(strings.Split(k, ","))
		if !ok {
			continue
		}
		selectors := specific[k]
		for i := 0; i < len(selectors); i += cssBatch {
			end := min(i+cssBatch, len(selectors))
			hides = append(hides, webkitRule{
				Trigger: webkitTrigger{URLFilter: ".*", IfDomain: ifDomain, UnlessDomain: unlessDomain},
				Action:  webkitAction{Type: "css-display-none", Selector: strings.Join(selectors[i:end], ", ")},
			})
		}
	}
	return blocks, hides, exceptions
}

// networkRule converts one network rule; ok is false for one WebKit cannot
// say (a regular expression, an option it has no trigger for, a pattern
// that would match everything).
func networkRule(line string) (rule webkitRule, exception, ok bool) {
	if strings.HasPrefix(line, "@@") {
		exception = true
		line = line[2:]
	}
	pattern, options := line, ""
	if i := strings.LastIndex(line, "$"); i >= 0 && !strings.HasPrefix(line, "/") {
		pattern, options = line[:i], line[i+1:]
	}
	if strings.HasPrefix(pattern, "/") && strings.HasSuffix(pattern, "/") && len(pattern) > 1 {
		return rule, false, false
	}
	trigger := webkitTrigger{}
	var types []string
	var notTypes []string
	if options != "" {
		for _, option := range strings.Split(options, ",") {
			option = strings.TrimSpace(option)
			name, value, _ := strings.Cut(option, "=")
			negated := strings.HasPrefix(name, "~")
			name = strings.TrimPrefix(name, "~")
			switch {
			case webkitTypes[name] != nil:
				if negated {
					notTypes = append(notTypes, webkitTypes[name]...)
				} else {
					types = append(types, webkitTypes[name]...)
				}
			case name == "third-party" || name == "3p":
				trigger.LoadType = []string{map[bool]string{false: "third-party", true: "first-party"}[negated]}
			case name == "first-party" || name == "1p":
				trigger.LoadType = []string{map[bool]string{false: "first-party", true: "third-party"}[negated]}
			case name == "domain":
				var good bool
				trigger.IfDomain, trigger.UnlessDomain, good = webkitDomains(strings.Split(value, "|"))
				if !good {
					return rule, false, false
				}
			case name == "match-case":
				trigger.CaseSensitive = true
			case webkitOptions[name]:
			default:
				return rule, false, false
			}
		}
	}
	switch {
	case len(types) > 0:
		trigger.ResourceType = unique(types)
	case len(notTypes) > 0:
		for _, t := range webkitSubresources {
			if !contains(notTypes, t) {
				trigger.ResourceType = append(trigger.ResourceType, t)
			}
		}
	default:
		trigger.ResourceType = webkitSubresources
	}
	filter, good := webkitPattern(pattern)
	if !good {
		return rule, false, false
	}
	// A pattern that matches anything blocks nothing on purpose without a
	// domain to keep it to.
	if filter == ".*" && trigger.IfDomain == nil && !exception {
		return rule, false, false
	}
	trigger.URLFilter = filter
	action := webkitAction{Type: "block"}
	if exception {
		action.Type = "ignore-previous-rules"
	}
	return webkitRule{Trigger: trigger, Action: action}, exception, true
}

// webkitPattern turns an EasyList pattern into WebKit's regular expression
// (which has no alternation, no counted repeats and no Unicode).
func webkitPattern(pattern string) (string, bool) {
	var sb strings.Builder
	switch {
	case strings.HasPrefix(pattern, "||"):
		sb.WriteString(`^[^:]+://+([^:/]+\.)?`)
		pattern = pattern[2:]
	case strings.HasPrefix(pattern, "|"):
		sb.WriteString("^")
		pattern = pattern[1:]
	}
	end := ""
	if strings.HasSuffix(pattern, "|") {
		end = "$"
		pattern = strings.TrimSuffix(pattern, "|")
	}
	pattern = strings.TrimSuffix(strings.TrimPrefix(pattern, "*"), "*")
	for i, r := range pattern {
		switch {
		case r > 127:
			return "", false
		case r == '*':
			sb.WriteString(".*")
		case r == '^':
			// A separator; at the end it may also be the end of the address.
			if i == len(pattern)-1 {
				continue
			}
			sb.WriteString(`[/:?=&]`)
		case strings.ContainsRune(`.+?()[]{}\|$`, r):
			sb.WriteByte('\\')
			sb.WriteRune(r)
		default:
			sb.WriteRune(r)
		}
	}
	sb.WriteString(end)
	out := sb.String()
	if out == "" || out == "^" {
		out = ".*"
	}
	return out, true
}

// webkitDomains turns a rule's domains ("a.com", "~b.com") into WebKit's
// if-domain or unless-domain (which cannot both be given; the domains the
// rule is for win). ok is false for none WebKit can say.
func webkitDomains(domains []string) (ifDomain, unlessDomain []string, ok bool) {
	for _, d := range domains {
		d = strings.ToLower(strings.TrimSpace(d))
		negated := strings.HasPrefix(d, "~")
		d = strings.TrimPrefix(d, "~")
		if d == "" || !plainDomain.MatchString(d) {
			continue
		}
		if negated {
			unlessDomain = append(unlessDomain, "*"+d)
		} else {
			ifDomain = append(ifDomain, "*"+d)
		}
	}
	if len(ifDomain) > 0 {
		return ifDomain, nil, true
	}
	return nil, unlessDomain, len(unlessDomain) > 0
}

var plainDomain = regexp.MustCompile(`^[a-z0-9.-]+$`)

// safeSelector keeps to selectors WebKit is sure to take: no extended
// syntax (:has, :-abp-…, :contains) and no pseudo-class but a few plain ones.
var (
	pseudoClass   = regexp.MustCompile(`:([a-zA-Z-]+)`)
	plainPseudos  = map[string]bool{"not": true, "first-child": true, "last-child": true, "nth-child": true, "nth-of-type": true, "first-of-type": true, "last-of-type": true, "only-child": true, "empty": true}
	selectorChars = regexp.MustCompile(`^[\x20-\x7e]+$`)
	attributes    = regexp.MustCompile(`\[[^\]]*\]`)
)

func safeSelector(selector string) bool {
	if selector == "" || len(selector) > 500 || !selectorChars.MatchString(selector) || strings.ContainsAny(selector, "{}\\") ||
		strings.Contains(selector, "::") || strings.HasPrefix(selector, "+js") || strings.HasPrefix(selector, "^") {
		return false
	}
	// Pseudo-classes outside attribute values.
	outside := attributes.ReplaceAllString(selector, "[]")
	for _, m := range pseudoClass.FindAllStringSubmatch(outside, -1) {
		if !plainPseudos[m[1]] {
			return false
		}
	}
	return true
}

func unique(list []string) []string {
	var out []string
	for _, s := range list {
		if !contains(out, s) {
			out = append(out, s)
		}
	}
	return out
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
