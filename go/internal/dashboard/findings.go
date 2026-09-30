package dashboard

import (
	"sort"

	"github.com/mickeyyaya/evolve-loop/go/internal/reportdoc"
)

type Finding = reportdoc.Finding

func parseFindings(markdown string) []Finding { return reportdoc.Findings(markdown) }

func parseVerdict(markdown string) string { return reportdoc.Verdict(markdown) }

func sortFindings(fs []Finding) {
	sort.SliceStable(fs, func(i, j int) bool {
		if ri, rj := reportdoc.SeverityRank(fs[i].Severity), reportdoc.SeverityRank(fs[j].Severity); ri != rj {
			return ri < rj
		}
		return fs[i].ID < fs[j].ID
	})
}

func diffRounds(prev, cur []Finding) (resolved, fresh, carried int) {
	byKey := make(map[string]int, len(prev))
	byIDAndSeverity := make(map[string]int, len(prev))
	for i, f := range prev {
		byKey[reportdoc.FindingKey(f.Title)] = i
		if f.ID != "" {
			byIDAndSeverity[f.ID+"|"+f.Severity] = i
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
			i, ok = byIDAndSeverity[f.ID+"|"+f.Severity]
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
