package apicover

import "strings"

func namedLookupKey(s Symbol) string {
	if s.Kind == KindMethod {
		if i := strings.LastIndex(s.Name, "."); i >= 0 {
			return s.Name[i+1:]
		}
	}
	return s.Name
}

type Report struct {
	Covered     []Symbol
	Uncovered   []Symbol
	FalseGreens []Symbol
	Ignored     []Symbol
}

func Classify(syms []Symbol, namedInTests map[string]bool, coverPct map[string]float64) Report {
	var r Report
	for _, s := range syms {
		if s.Ignored {
			r.Ignored = append(r.Ignored, s)
			continue
		}
		named := namedInTests[namedLookupKey(s)]
		if s.Kind == KindFunc || s.Kind == KindMethod {
			pct, hasCover := coverPct[s.Name]
			switch {
			case named && hasCover && pct > 0:
				r.Covered = append(r.Covered, s)
			case named:
				r.FalseGreens = append(r.FalseGreens, s)
			default:
				r.Uncovered = append(r.Uncovered, s)
			}
			continue
		}
		if named {
			r.Covered = append(r.Covered, s)
		} else {
			r.Uncovered = append(r.Uncovered, s)
		}
	}
	return r
}
