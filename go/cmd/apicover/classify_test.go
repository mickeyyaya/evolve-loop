package main

import "testing"

func TestUncovered_FlagsReferencedButZeroCoverageFunc(t *testing.T) {
	syms := []Symbol{
		{Name: "Covered", Kind: KindFunc},
		{Name: "NamedButZero", Kind: KindFunc},
		{Name: "NeverNamed", Kind: KindFunc},
	}
	named := map[string]bool{"Covered": true, "NamedButZero": true}
	cover := map[string]float64{"Covered": 87.5, "NamedButZero": 0.0}

	rep := Classify(syms, named, cover)

	if !containsName(rep.Covered, "Covered") {
		t.Errorf("Covered (named + 87.5%%) should be covered; report=%s", reportString(rep))
	}
	if !containsName(rep.FalseGreens, "NamedButZero") {
		t.Errorf("NamedButZero (named in test, 0%% cover) should be a false-green; report=%s", reportString(rep))
	}
	if containsName(rep.Covered, "NamedButZero") {
		t.Errorf("NamedButZero must NOT count as covered")
	}
	if !containsName(rep.Uncovered, "NeverNamed") {
		t.Errorf("NeverNamed (not named by any test) should be uncovered; report=%s", reportString(rep))
	}
}

func TestClassify_MethodMatchedByBareName(t *testing.T) {
	// A test referencing obj.Method() yields the bare selector "Method" (no
	// receiver type), but the symbol is keyed "T.Method". The cover map is
	// joined on file:line in main, so it is keyed by the full symbol name.
	syms := []Symbol{{Name: "T.Method", Kind: KindMethod}}
	named := map[string]bool{"Method": true}
	cover := map[string]float64{"T.Method": 50.0}

	rep := Classify(syms, named, cover)
	if !containsName(rep.Covered, "T.Method") {
		t.Errorf("method should be Covered via the bare-name named signal + cover; report=%s", reportString(rep))
	}
}

func containsName(syms []Symbol, name string) bool {
	for _, s := range syms {
		if s.Name == name {
			return true
		}
	}
	return false
}

func reportString(r Report) string {
	return "covered=" + join(r.Covered) + " uncovered=" + join(r.Uncovered) +
		" falseGreens=" + join(r.FalseGreens) + " ignored=" + join(r.Ignored)
}

func join(syms []Symbol) string {
	out := ""
	for i, s := range syms {
		if i > 0 {
			out += ","
		}
		out += s.Name
	}
	return "[" + out + "]"
}
