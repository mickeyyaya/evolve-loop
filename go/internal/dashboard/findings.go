package dashboard

import (
	"sort"

	"github.com/mickeyyaya/evolve-loop/go/internal/reportdoc"
)

// Finding is one auditor issue heading. The grammar is single-homed in
// reportdoc (the repair-brief seed in core reads the same functions), so the
// panel and the rebuilding agent can never disagree about what a finding is.
type Finding = reportdoc.Finding

// severityRank orders findings highest severity first; unknown severities sort
// after the known ones so nothing is dropped.
var severityRank = map[string]int{"CRITICAL": 0, "HIGH": 1, "MEDIUM": 2, "LOW": 3}

func parseFindings(markdown string) []Finding { return reportdoc.Findings(markdown) }

func parseVerdict(markdown string) string { return reportdoc.Verdict(markdown) }

// sortFindings orders by severity rank, then by id, in place.
func sortFindings(fs []Finding) {
	rank := func(sev string) int {
		if r, ok := severityRank[sev]; ok {
			return r
		}
		return len(severityRank)
	}
	sort.SliceStable(fs, func(i, j int) bool {
		if ri, rj := rank(fs[i].Severity), rank(fs[j].Severity); ri != rj {
			return ri < rj
		}
		return fs[i].ID < fs[j].ID
	})
}

// diffRounds compares one audit round's findings with the previous round's.
// A finding is carried when its lead clause (reportdoc.FindingKey) OR its
// id+severity appears in the previous round — auditors keep the id when they
// re-cite a defect (1605: H1 across three rounds) but reword the title, and
// occasionally renumber (1604: D1 → C1) while keeping the wording. The counts
// feed the repair-round history line ("r2 FAIL (3: 5 resolved, 1 new)").
func diffRounds(prev, cur []Finding) (resolved, fresh, carried int) {
	// Both lookups resolve to the SAME prior index, and a prior finding is
	// consumed at most once: a second current finding that merely reuses an
	// old id (or old wording) for a new defect is new, not a second carry.
	// The id match is only meaningful when the layout has ids: the reference
	// template's TSV rows carry none, and "|HIGH" would otherwise match any
	// HIGH to any other.
	byKey := make(map[string]int, len(prev))
	byID := make(map[string]int, len(prev))
	for i, f := range prev {
		byKey[reportdoc.FindingKey(f.Title)] = i
		if f.ID != "" {
			byID[f.ID+"|"+f.Severity] = i
		}
	}
	used := make([]bool, len(prev))
	claim := func(i int, ok bool) bool {
		if !ok || used[i] {
			return false
		}
		used[i] = true
		return true
	}
	for _, f := range cur {
		i, ok := byKey[reportdoc.FindingKey(f.Title)]
		if claim(i, ok) {
			carried++
			continue
		}
		if f.ID != "" {
			i, ok = byID[f.ID+"|"+f.Severity]
			if claim(i, ok) {
				carried++
				continue
			}
		}
		fresh++
	}
	for _, u := range used {
		if !u {
			resolved++
		}
	}
	return resolved, fresh, carried
}
