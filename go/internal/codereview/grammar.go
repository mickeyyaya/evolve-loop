package codereview

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/qualityindex"
	"github.com/mickeyyaya/evolve-loop/go/internal/reportdoc"
)

const FindingsSection = "Findings"

var findingID = regexp.MustCompile(`\bCR\d+\b`)

var findingOpener = regexp.MustCompile(`^(?:#+|[-*+]|\d+[.)])\s+\**CR\d+\b`)

const findingHeadingForm = "`### CR<n> (SEVERITY) — <title>` with the severity in capitals"

func ValidateReport(report string, thresholds qualityindex.Thresholds) []string {
	plan, planProblems := qualityindex.ParsePlan(report)
	scores, scoreProblems := qualityindex.ParseScores(report)
	problems := append(append(planProblems, scoreProblems...), findingsProblems(report)...)
	if len(problems) > 0 {
		return problems
	}
	problems = qualityindex.Agree(plan, scores)
	return append(problems, uncitedGaps(scores, thresholds, Parse(report))...)
}

func uncitedGaps(scores qualityindex.Scores, thresholds qualityindex.Thresholds, findings []Finding) []string {
	var problems []string
	for _, key := range qualityindex.Keys() {
		s := scores[key]
		if s.NA || s.Value >= thresholds[key] {
			continue
		}
		if !citesFindingOn(s.Rationale, key, findings) {
			problems = append(problems, fmt.Sprintf("dimension %s scores %d, below its threshold %d, but its rationale cites no finding on %s: cite the finding's id (`%d — CR2: …`) or raise one, so the builder has a fix to act on", key, s.Value, thresholds[key], key, s.Value))
		}
	}
	return problems
}

func citesFindingOn(rationale, dimension string, findings []Finding) bool {
	for _, id := range findingID.FindAllString(rationale, -1) {
		if slices.ContainsFunc(findings, func(f Finding) bool { return f.ID == id && f.Dimension == dimension }) {
			return true
		}
	}
	return false
}

func findingsProblems(report string) []string {
	body, found, err := reportdoc.Section(report, FindingsSection)
	if err != nil {
		return []string{err.Error()}
	}
	if !found {
		return []string{"the report has no ## " + FindingsSection + " section"}
	}
	var problems []string
	parsed := 0
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "### ") && len(reportdoc.Findings(line)) == 1:
			parsed++
		case strings.HasPrefix(line, "### ") || findingOpener.MatchString(line):
			problems = append(problems, fmt.Sprintf("## %s line %q is not a finding heading: write %s", FindingsSection, line, findingHeadingForm))
		}
	}
	if parsed == 0 && len(problems) == 0 && !strings.EqualFold(strings.TrimSuffix(body, "."), "none") {
		problems = append(problems, fmt.Sprintf("## %s holds no finding and is not `None.`: write each finding as %s, or `None.` when there is none", FindingsSection, findingHeadingForm))
	}
	return problems
}
