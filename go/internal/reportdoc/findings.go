package reportdoc

import (
	"regexp"
	"strings"
)

type Finding struct {
	ID       string `json:"id"`
	Severity string `json:"severity"`
	Title    string `json:"title"`
}

const severities = `CRITICAL|HIGH|MEDIUM|LOW`

var severityOrder = strings.Split(severities, "|")

func SeverityRank(severity string) int {
	for i, s := range severityOrder {
		if s == severity {
			return i
		}
	}
	return len(severityOrder)
}

var findingParen = regexp.MustCompile(`^###\s+\**([A-Z]+\d+)\**\s*\((` + severities + `)([^)]*)\)\s*(?:[—–-]+\s*(.*))?$`)

var findingDash = regexp.MustCompile(`^###\s+\**([A-Z]+\d+)\**\s*[—–:-]+\s*(?:(` + severities + `)\b\s*[·—–:-]*\s*)?(.*)$`)

var findingRow = regexp.MustCompile(`^\|\s*\**([A-Z]+\d+)\**\s*\|\s*\**(` + severities + `)\**\s*\|\s*([^|]*?)\s*\|`)

var findingTSV = regexp.MustCompile(`^\**(` + severities + `)\**\t+([^\t]*)`)

var severityByPrefix = map[byte]string{'C': "CRITICAL", 'H': "HIGH", 'M': "MEDIUM", 'L': "LOW"}

var verdictCanonicalRE = regexp.MustCompile(`(?m)^##[^\S\n]*Verdict[^\S\n]*\n\s*\*{0,2}(PASS|FAIL|WARN|SKIPPED)\*{0,2}`)

var verdictInlineRE = regexp.MustCompile(`(?m)^[^\S\n]*(?:##[^\S\n]*)?\*{0,2}Verdict\*{0,2}[^\S\n]*:[^\S\n]*\*{0,2}[^\S\n]*(PASS|FAIL|WARN|SKIPPED)\b`)

func Findings(markdown string) []Finding {
	visible, rawIdx := visibleLinesIdx(markdown)
	start, end, ok := issuesBounds(visible)
	if !ok {
		return scanFindings(visible, false)
	}
	out := scanFindings(visible[start:end], true)
	rawLines := strings.Split(markdown, "\n")
	rawEnd := len(rawLines)
	if end < len(rawIdx) {
		rawEnd = rawIdx[end]
	}
	return append(out, tsvFindings(rawLines[rawIdx[start]:rawEnd])...)
}

func issuesBounds(visible []string) (start, end int, ok bool) {
	for i, line := range visible {
		if strings.HasPrefix(strings.TrimSpace(line), "## Issues") {
			start = i
			end = len(visible)
			for j := i + 1; j < len(visible); j++ {
				if strings.HasPrefix(visible[j], "## ") {
					end = j
					break
				}
			}
			return start, end, true
		}
	}
	return 0, 0, false
}

func scanFindings(visible []string, inIssues bool) []Finding {
	var out []Finding
	for _, line := range visible {
		if f, ok := parseFindingLine(strings.TrimRight(line, " \t\r"), inIssues); ok {
			out = append(out, f)
		}
	}
	return out
}

func parseFindingLine(line string, inIssues bool) (Finding, bool) {
	if m := findingParen.FindStringSubmatch(line); m != nil {
		title := m[4]
		if strings.TrimSpace(title) == "" {
			title = strings.TrimPrefix(m[3], ",")
		}
		return Finding{ID: m[1], Severity: m[2], Title: cleanTitle(title)}, true
	}
	if m := findingRow.FindStringSubmatch(line); m != nil {
		return Finding{ID: m[1], Severity: m[2], Title: cleanTitle(m[3])}, true
	}
	if m := findingDash.FindStringSubmatch(line); m != nil {
		sev := m[2]
		if sev == "" {
			if !inIssues {
				return Finding{}, false
			}
			sev = severityByPrefix[m[1][0]]
		}
		return Finding{ID: m[1], Severity: sev, Title: cleanTitle(m[3])}, true
	}
	return Finding{}, false
}

func tsvFindings(issues []string) []Finding {
	var out []Finding
	inTSV := false
	for _, raw := range issues {
		line := strings.TrimRight(raw, " \r")
		switch {
		case !inTSV && strings.TrimSpace(line) == "```tsv":
			inTSV = true
		case inTSV && strings.HasPrefix(strings.TrimSpace(line), "```"):
			inTSV = false
		case inTSV:
			if m := findingTSV.FindStringSubmatch(line); m != nil {
				out = append(out, Finding{Severity: m[1], Title: cleanTitle(m[2])})
			}
		}
	}
	return out
}

func cleanTitle(s string) string {
	return strings.Trim(strings.TrimSpace(s), "*_ ")
}

func Verdict(markdown string) string {
	visible := strings.Join(visibleLines(markdown), "\n")
	if m := verdictCanonicalRE.FindStringSubmatch(visible); m != nil {
		return m[1]
	}
	if m := verdictInlineRE.FindStringSubmatch(visible); m != nil {
		return m[1]
	}
	return ""
}

var keyCuts = []string{" for the ", " (round", " round ", ":", ",", ". ", " — ", " – ", " - "}

const findingKeyMaxLen = 80

func FindingKey(title string) string {
	s := strings.ToLower(cleanTitle(title))
	for _, cut := range keyCuts {
		if i := strings.Index(s, cut); i > 0 {
			s = s[:i]
		}
	}
	var b strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	key := b.String()
	if len(key) > findingKeyMaxLen {
		key = key[:findingKeyMaxLen]
	}
	return key
}
