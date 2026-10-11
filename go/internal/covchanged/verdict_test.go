package covchanged

import (
	"reflect"
	"testing"
)

func gap(key string, covered, stmts int) Func {
	return Func{File: "p/a.go", Line: 3, Name: key, Key: key, Statements: stmts, Covered: covered, Profiled: true}
}

func TestJudge_RefusesEveryChangedFunctionBelowFullAndCountsTheRest(t *testing.T) {
	report := Report{Funcs: []Func{gap("p.New", 2, 3), gap("p.Done", 4, 4), {Key: "p.Unseen"}, gap("p.Empty", 0, 0)}}

	v := Judge(report)

	if v.Clean() || !reflect.DeepEqual(keys(v.Refused), []string{"p.New", "p.Unseen"}) {
		t.Errorf("refused %v clean %v, want [p.New p.Unseen] and a refusal", keys(v.Refused), v.Clean())
	}
	if v.Changed != 4 || v.Full != 2 {
		t.Errorf("changed %d full %d, want 4 and 2", v.Changed, v.Full)
	}
	if clean := Judge(Report{Funcs: []Func{gap("p.Done", 4, 4)}}); !clean.Clean() || clean.Full != 1 {
		t.Errorf("a change at 100%% must pass: %+v", clean)
	}
}

func TestJudge_CarriesTheExemptionsAndReportsEmptyListsNotNull(t *testing.T) {
	ex := []Exemption{{File: "p/x_linux.go", Reason: ReasonBuildConstraints}}

	v := Judge(Report{Exempt: ex})

	if !reflect.DeepEqual(v.Exempt, ex) || v.Refused == nil || !v.Clean() {
		t.Errorf("Judge = %+v, want the exemptions, a non-nil empty refusal list and a clean verdict", v)
	}
}

func TestVerdictProblems_NameEachRefusalWithItsPercentOrNoProfile(t *testing.T) {
	v := Verdict{Refused: []Func{gap("p.F", 2, 3), {File: "p/b.go", Line: 9, Key: "p.G"}}}

	got := v.Problems()

	want := []string{"REFUSED p/a.go:3 p.F 66.7% (2/3)", "REFUSED p/b.go:9 p.G no profile"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Problems = %q, want %q", got, want)
	}
	if s := gap("p.A", 1, 2).String(); s != "p/a.go:3 p.A 50.0% (1/2)" {
		t.Errorf("String = %q", s)
	}
}

func keys(funcs []Func) []string {
	out := []string{}
	for _, f := range funcs {
		out = append(out, f.Key)
	}
	return out
}
