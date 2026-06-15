package main

import "strings"

// namedLookupKey maps a symbol to the key under which a test reference would
// appear in NamesReferencedInTests. Methods are referenced by their bare
// selector name (no receiver type is recoverable from the test AST without
// go/types), so a "T.Method" symbol is looked up as "Method".
func namedLookupKey(s Symbol) string {
	if s.Kind == KindMethod {
		if i := strings.LastIndex(s.Name, "."); i >= 0 {
			return s.Name[i+1:]
		}
	}
	return s.Name
}

// Report is the outcome of classifying a package's exported symbols against the
// two coverage signals (named-in-a-test AST, and >0% in `go tool cover -func`).
type Report struct {
	Covered     []Symbol
	Uncovered   []Symbol // exported, not named by any test
	FalseGreens []Symbol // func/method named by a test but 0% executed coverage
	Ignored     []Symbol // //apicover:ignore
}

// Classify applies the two-signal coverage check to a package's exported
// symbols. A func/method is Covered only when it is BOTH named by a test AST and
// shows >0% in `go tool cover -func`; named-but-0% (or no cover entry) is a
// FalseGreen — the failure mode a bare line-coverage number hides. Non-func
// symbols (types/vars/consts) have no coverage signal, so they are Covered iff
// named by a test. Symbols flagged //apicover:ignore are bucketed separately.
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
