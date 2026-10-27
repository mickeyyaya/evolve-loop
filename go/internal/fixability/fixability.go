package fixability

import (
	"path"
	"regexp"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
)

const ReasonHandoffBlocker = "handoff_blocker"

var (
	testName  = regexp.MustCompile(`\bTest[A-Za-z0-9_]+`)
	provenRed = regexp.MustCompile(`(?i)\b(?:tests?|predicates?)(?:\s+\(\d+\))?\s+(?:fail|fails|failed)\b`)
)

type Input struct {
	Defects          []string
	RedPaths         []string
	GateFailures     []string
	OpenConsoleItems []string
	Salvage          string
}

type Finding struct {
	ID           string
	Severity     string
	Text         string
	ConsoleItems []string
}

type Decision struct {
	Stop         bool
	Reason       string
	Lane         []Finding
	Handoff      []Finding
	Blockers     []Finding
	ConsoleItems []string
	Salvage      string
}

func Classify(in Input) Decision {
	var lane []Finding
	var handoff []Finding
	var blockers []Finding
	var consoleItems []string

	for _, defect := range in.Defects {
		f, paths := parseFinding(defect, in.OpenConsoleItems)
		if isHandoffFinding(f, paths) {
			handoff = append(handoff, f)
			if isBlockerFinding(f, paths, in.RedPaths) || provenRed.MatchString(f.Text) || gateNames(f, paths, in.GateFailures) {
				blockers = append(blockers, f)
				consoleItems = appendUnique(consoleItems, f.ConsoleItems...)
			}
		} else {
			lane = append(lane, f)
		}
	}

	d := Decision{
		Lane:         lane,
		Handoff:      handoff,
		Blockers:     blockers,
		ConsoleItems: consoleItems,
	}

	if len(blockers) > 0 {
		d.Stop = true
		d.Reason = ReasonHandoffBlocker
		d.Salvage = in.Salvage
	}

	return d
}

func parseFinding(raw string, openConsoleItems []string) (Finding, []string) {
	trimmed := strings.TrimSpace(raw)
	id := ""
	if idx := strings.Index(trimmed, ":"); idx != -1 {
		id = strings.TrimSpace(trimmed[:idx])
	}
	sev := severityFromID(id)
	f := Finding{
		ID:           id,
		Severity:     sev,
		Text:         trimmed,
		ConsoleItems: matchConsoleItems(trimmed, openConsoleItems),
	}
	return f, extractPaths(trimmed)
}

func severityFromID(id string) string {
	if len(id) == 0 {
		return ""
	}
	switch id[0] {
	case 'C':
		return "CRITICAL"
	case 'H':
		return "HIGH"
	case 'M':
		return "MEDIUM"
	case 'L':
		return "LOW"
	default:
		return ""
	}
}

func matchConsoleItems(text string, openItems []string) []string {
	var matched []string
	for _, item := range openItems {
		if item != "" && idBoundary.mentions(text, item) {
			matched = append(matched, item)
		}
	}
	return matched
}

func extractPaths(text string) []string {
	var paths []string
	seen := make(map[string]bool)
	for _, field := range strings.Fields(text) {
		token := strings.Trim(field, " \t\r\n,;()\"'`")
		pathPart, _, _ := strings.Cut(token, ":")
		pathPart = strings.Trim(pathPart, " \t\r\n,;()\"'`")
		if strings.Contains(pathPart, "/") {
			clean := path.Clean(strings.TrimPrefix(pathPart, "/"))
			if clean != "." && clean != "" && !seen[clean] {
				seen[clean] = true
				paths = append(paths, clean)
			}
		}
	}
	return paths
}

func isHandoffFinding(f Finding, paths []string) bool {
	if len(f.ConsoleItems) > 0 {
		return true
	}
	for _, p := range paths {
		if guards.IsProtectedSurface(p) {
			return true
		}
	}
	return false
}

func isBlockerFinding(f Finding, paths []string, redPaths []string) bool {
	if f.Severity == "CRITICAL" || f.Severity == "HIGH" {
		return true
	}
	for _, p := range paths {
		for _, red := range redPaths {
			if matchPathSegment(p, red) {
				return true
			}
		}
	}
	return false
}

func matchPathSegment(a, b string) bool {
	a = path.Clean(strings.TrimPrefix(a, "/"))
	b = path.Clean(strings.TrimPrefix(b, "/"))
	if a == b {
		return true
	}
	if strings.HasPrefix(a, b+"/") || strings.HasPrefix(b, a+"/") {
		return true
	}
	return false
}

func appendUnique(items []string, add ...string) []string {
	for _, a := range add {
		if a == "" {
			continue
		}
		found := false
		for _, it := range items {
			if it == a {
				found = true
				break
			}
		}
		if !found {
			items = append(items, a)
		}
	}
	return items
}

type boundary struct{ joinsBefore, joinsAfter func(byte) bool }

var (
	idBoundary   = boundary{isIDByte, isIDByte}
	pathBoundary = boundary{isPathLeadByte, isPathTailByte}
	wordBoundary = boundary{isWordByte, isWordByte}
)

func gateNames(f Finding, paths []string, gateFailures []string) bool {
	tests := testName.FindAllString(f.Text, -1)
	for _, gate := range gateFailures {
		for _, p := range paths {
			if pathBoundary.mentions(gate, p) || citesPackageOf(gate, p) {
				return true
			}
		}
		for _, name := range tests {
			if wordBoundary.mentions(gate, name) {
				return true
			}
		}
	}
	return false
}

func citesPackageOf(gate, p string) bool {
	dir := path.Dir(p)
	return strings.HasSuffix(p, ".go") && dir != "." && pathBoundary.mentions(gate, dir)
}

func (b boundary) mentions(text, token string) bool {
	for start := 0; ; {
		i := strings.Index(text[start:], token)
		if i < 0 {
			return false
		}
		at, end := start+i, start+i+len(token)
		if (at == 0 || !b.joinsBefore(text[at-1])) && (end == len(text) || !b.joinsAfter(text[end])) {
			return true
		}
		start = at + 1
	}
}
func isWordByte(b byte) bool {
	return b == '_' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

func isIDByte(b byte) bool {
	return b == '-' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z'
}

func isPathLeadByte(b byte) bool {
	return isWordByte(b) || b == '-' || b == '.'
}

func isPathTailByte(b byte) bool {
	return isWordByte(b) || b == '-' || b == '/'
}
