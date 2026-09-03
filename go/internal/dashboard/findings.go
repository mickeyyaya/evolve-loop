package dashboard

import (
	"sort"

	"github.com/mickeyyaya/evolve-loop/go/internal/reportdoc"
)

// Finding is one auditor issue heading. The grammar is single-homed in
// reportdoc (the repair-brief seed in core reads the same functions), so the
// panel and the rebuilding agent can never disagree about what a finding is.
type Finding = reportdoc.Finding

func parseFindings(markdown string) []Finding { return reportdoc.Findings(markdown) }

func parseVerdict(markdown string) string { return reportdoc.Verdict(markdown) }

// sortFindings orders by reportdoc.SeverityRank (highest first; unknown
// severities after the known ones so nothing is dropped), then by id, in place.
func sortFindings(fs []Finding) {
	sort.SliceStable(fs, func(i, j int) bool {
		if ri, rj := reportdoc.SeverityRank(fs[i].Severity), reportdoc.SeverityRank(fs[j].Severity); ri != rj {
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
