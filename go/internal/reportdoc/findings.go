package reportdoc

import (
	"regexp"
	"strings"
)

// findings.go — the auditor's issue and verdict grammar, single-homed.
//
// The auditor persona fixes the vocabulary (IDs H1/M1/L1/C1, severities
// CRITICAL/HIGH/MEDIUM/LOW) but not the layout. Five layouts are live —
// four observed in recorded reports (cycles 1595, 1596, 1604, 1605, 1606) and
// the reference template's own (agents/evolve-auditor-reference.md):
//
//	### H1 (HIGH) — caller-proof hard floor violated          (heading, severity in parens)
//	### C1 — CRITICAL · the partial landing still consumes … (heading, severity after a dash)
//	### H3 — the validated triage hint skips the tdd phase    (heading, severity implied by the id letter)
//	| **H1** | **HIGH** | **Caller-proof hard floor …** | …  (table row: id, severity, defect; emphasis optional)
//	```tsv  Severity⇥Description⇥File⇥Line  HIGH⇥<issue>⇥…      (the reference template: a fenced TSV under ## Issues)
//
// and, when it does not emit the machine-readable sentinel
// (phasecontract.ParseVerdictSentinel — always authoritative, tried first by
// every consumer), declares the verdict as the canonical two-line heading
// `## Verdict` + `**PASS**` or as an inline `**Verdict: PASS**`.
//
// Consumers today: the audit gate's regex-on-prose fallback
// (phases/audit extractAuditVerdict → Verdict) and the dashboard's failure
// panel (Findings, Verdict, FindingKey). The repair-brief seed in core is the
// intended third consumer (research proposal R2) so that what the gate scores,
// what the operator is shown, and what the rebuilding agent is told cannot
// drift; until it lands, this package is still the only place the grammar is
// spelled.
//
// Like every extractor in this package, both functions scan only VISIBLE
// lines: fenced code, indented code and HTML comments are stripped first, so
// a template or example embedded in a report cannot declare a verdict or raise
// a phantom finding. The one deliberate exception is the reference template's
// own fenced TSV block directly under `## Issues`, which IS the issues table.

// Finding is one auditor issue. ID is empty for the TSV layout, which has no
// id column.
type Finding struct {
	ID       string `json:"id"`
	Severity string `json:"severity"`
	Title    string `json:"title"`
}

const severities = `CRITICAL|HIGH|MEDIUM|LOW`

// findingParen: `### H1 (HIGH[, qualifier]) [— title]`. Group 1 id, 2 severity,
// 3 qualifier (may be empty), 4 title (may be absent ⇒ the qualifier is the title).
var findingParen = regexp.MustCompile(`^###\s+\**([A-Z]+\d+)\**\s*\((` + severities + `)([^)]*)\)\s*(?:[—–-]+\s*(.*))?$`)

// findingDash: `### C1 — CRITICAL · title` or `### H3 — title` (severity then
// implied by the id letter). Group 2 is the optional severity token.
var findingDash = regexp.MustCompile(`^###\s+\**([A-Z]+\d+)\**\s*[—–:-]+\s*(?:(` + severities + `)\b\s*[·—–:-]*\s*)?(.*)$`)

// findingRow: a table row whose first two cells are the id and the severity,
// with optional `**` emphasis; the third cell is the defect text. Header and
// separator rows never match (no id).
var findingRow = regexp.MustCompile(`^\|\s*\**([A-Z]+\d+)\**\s*\|\s*\**(` + severities + `)\**\s*\|\s*([^|]*?)\s*\|`)

// findingTSV: a row of the reference template's `tsv` block — severity first,
// then the description, tab-separated.
var findingTSV = regexp.MustCompile(`^\**(` + severities + `)\**\t+([^\t]*)`)

// severityByPrefix maps the persona's id letters to their severity when a
// heading omits the token.
var severityByPrefix = map[byte]string{'C': "CRITICAL", 'H': "HIGH", 'M': "MEDIUM", 'L': "LOW"}

// verdictCanonicalRE matches the canonical two-line heading
// "## Verdict\n**PASS**" — bold optional, intervening blank lines tolerated.
var verdictCanonicalRE = regexp.MustCompile(`(?m)^##[^\S\n]*Verdict[^\S\n]*\n\s*\*{0,2}(PASS|FAIL|WARN|SKIPPED)\*{0,2}`)

// verdictInlineRE matches single-line variants agents emit when they don't
// follow the canonical heading: "**Verdict: PASS**", "**Verdict:** PASS",
// "## Verdict: PASS", "Verdict: PASS". Horizontal-whitespace classes keep the
// match on one line. Case-sensitive on "Verdict" (capital V) so it never
// matches the lowercase JSON key "verdict" in an embedded result blob. The
// colon is REQUIRED: every real inline form has one, and requiring it stops a
// prose line like "Verdict PASS is required before shipping." from being
// mis-read as a PASS declaration on the ship gate. The no-colon canonical
// "## Verdict\n**PASS**" shape is covered by verdictCanonicalRE above.
var verdictInlineRE = regexp.MustCompile(`(?m)^[^\S\n]*(?:##[^\S\n]*)?\*{0,2}Verdict\*{0,2}[^\S\n]*:[^\S\n]*\*{0,2}[^\S\n]*(PASS|FAIL|WARN|SKIPPED)\b`)

// Findings extracts the auditor's issues in any of the live layouts. Inside
// `## Issues` every layout is accepted, including the id-less forms (severity
// implied by an id letter; the template's TSV block) — heading and table
// findings first, in document order, then the TSV rows. Outside it — a report
// with no `## Issues` section is scanned whole — a line must carry an explicit
// severity token to count, so a `### S1 — …` heading in some other section is
// never a phantom finding. The section is located on the VISIBLE stream, so a
// quoted template inside a fence can never be mistaken for the issues section.
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

// issuesBounds locates the `## Issues` section on the visible-line stream:
// [start, end) covers the heading through the line before the next level-two
// heading (or the end of the document).
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

// parseFindingLine tries the heading and table layouts on one visible line.
// The severity-less dash heading is accepted only inside `## Issues`.
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

// tsvFindings parses the reference template's fenced ```tsv block(s) inside
// the raw lines of the `## Issues` section: `Severity⇥Description⇥File⇥Line`
// rows, header skipped. Only a fence whose info string is exactly `tsv` counts.
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

// cleanTitle trims whitespace and surrounding emphasis markers.
func cleanTitle(s string) string {
	return strings.Trim(strings.TrimSpace(s), "*_ ")
}

// Verdict returns PASS, FAIL, WARN or SKIPPED as the report's visible prose
// declares it, or "" when no recognisable declaration exists. The canonical
// heading form is tried before the inline form, exactly as the audit gate's
// fallback does. Callers that need the machine-readable sentinel must check
// phasecontract.ParseVerdictSentinel first; this is the prose fallback only.
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

// keyCuts are the clause boundaries after which a finding's title stops being
// its identity: auditors re-cite a defect with a new tail ("…, and unremediated
// across a repair dispatch", "… for the third consecutive round", "…: the
// evidence …") while the lead clause stays put.
var keyCuts = []string{" for the ", " (round", " round ", ":", ",", ". ", " — ", " – ", " - "}

// FindingKey reduces a finding title to a cross-round comparison key: the
// lead clause, lower-cased, alphanumerics only, capped at 80 characters — so
// a table cell's long defect text in round 1 and a terse heading restating the
// same lead in round 3 still meet.
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
	if len(key) > 80 {
		key = key[:80]
	}
	return key
}
