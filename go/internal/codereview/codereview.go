// Package codereview is the kernel half of the code-review phase: it parses the review's findings and records them in the defect ledger.
// See docs/architecture/packages/internal-codereview.md.
package codereview

import (
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/defectledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/reportdoc"
)

const PhaseName = "code-review"

const ShadowReason = "code-review shadow stage (ADR-0124): recorded for the shadow-to-enforce measurement, not owed by the build"

const missingField = "(missing)"

type Finding struct {
	ID, Severity, Title                          string
	Dimension, Location, Scenario, Evidence, Fix string
}

var fieldKeys = []string{"dimension", "location", "scenario", "evidence", "fix"}

func Parse(report string) []Finding {
	body, found, err := reportdoc.Section(report, FindingsSection)
	if !found || err != nil {
		return nil
	}
	var out []Finding
	for _, block := range headingBlocks(body) {
		if f, ok := parseBlock(block); ok {
			out = append(out, f)
		}
	}
	return out
}

func headingBlocks(body string) []string {
	var blocks []string
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "### ") || len(blocks) == 0 {
			blocks = append(blocks, line)
			continue
		}
		blocks[len(blocks)-1] += "\n" + line
	}
	return blocks
}

func parseBlock(block string) (Finding, bool) {
	heading, rest, _ := strings.Cut(block, "\n")
	parsed := reportdoc.Findings(heading)
	if len(parsed) != 1 {
		return Finding{}, false
	}
	f := Finding{ID: parsed[0].ID, Severity: parsed[0].Severity, Title: parsed[0].Title}
	fields, err := reportdoc.Fields(rest, fieldKeys...)
	if err != nil {
		f.Scenario = strings.TrimSpace(rest)
		return f, true
	}
	f.Dimension, f.Location, f.Scenario = strings.ToLower(fields["dimension"]), fields["location"], fields["scenario"]
	f.Evidence, f.Fix = fields["evidence"], fields["fix"]
	return f, true
}

func Verdict(findings []Finding, threshold string) string {
	if len(findings) == 0 {
		return "PASS"
	}
	for _, f := range findings {
		if reportdoc.SeverityRank(f.Severity) <= reportdoc.SeverityRank(threshold) {
			return "FAIL"
		}
	}
	return "WARN"
}

func Rows(findings []Finding, round int) []defectledger.Entry {
	rows := make([]defectledger.Entry, len(findings))
	for i, f := range findings {
		rows[i] = defectledger.Entry{
			Text: f.rowText(), Status: defectledger.StatusDeferred, Reason: ShadowReason,
			Source: PhaseName, Round: round, Severity: f.Severity, Dimension: f.Dimension,
		}
	}
	return rows
}

func (f Finding) rowText() string {
	return fmt.Sprintf("[%s] %s %s — %s | scenario: %s | evidence: %s | fix: %s",
		f.Severity, orMissing(f.Dimension), orMissing(f.Location), f.Title,
		orMissing(f.Scenario), orMissing(f.Evidence), orMissing(f.Fix))
}

func orMissing(s string) string {
	if strings.TrimSpace(s) == "" {
		return missingField
	}
	return s
}

func RoundFrom(completed []string) int {
	round := 0
	for _, phase := range completed {
		if phase == PhaseName {
			round++
		}
	}
	return round
}
