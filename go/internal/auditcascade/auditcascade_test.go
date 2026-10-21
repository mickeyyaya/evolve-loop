package auditcascade

import (
	"reflect"
	"testing"
)

var rule = Rule{RiskPaths: []string{"go/internal/core/", "go/internal/guards/"}, MaxChangedFiles: 3, EscalateVerdicts: []string{"FAIL", "WARN"}}

func TestDecide_EscalatesOnARiskPathALargeDiffOrAFlaggedVerdictAndSkipsATrivialCleanDiff(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   Inputs
		want Decision
	}{
		{"trivial clean diff", Inputs{Changed: []string{"docs/a.md"}, Verdict: "PASS", Known: true}, Decision{}},
		{"high-risk path", Inputs{Changed: []string{"docs/a.md", "go/internal/core/x.go"}, Verdict: "PASS", Known: true},
			Decision{Escalate: true, Reasons: []string{"risk path go/internal/core/x.go"}}},
		{"large diff", Inputs{Changed: []string{"a", "b", "c", "d"}, Verdict: "PASS", Known: true},
			Decision{Escalate: true, Reasons: []string{"4 changed files > 3"}}},
		{"flagged verdict", Inputs{Changed: []string{"docs/a.md"}, Verdict: "WARN", Known: true},
			Decision{Escalate: true, Reasons: []string{"verdict WARN"}}},
		{"unknown diff fails closed", Inputs{Verdict: "PASS"}, Decision{Escalate: true, Reasons: []string{"the diff is unknown"}}},
		{"every reason at once", Inputs{Changed: []string{"go/internal/guards/g.go", "go/internal/core/c.go", "x", "y"}, Verdict: "FAIL", Known: true},
			Decision{Escalate: true, Reasons: []string{"verdict FAIL", "risk path go/internal/guards/g.go", "4 changed files > 3"}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Decide(c.in, rule); !reflect.DeepEqual(got, c.want) {
				t.Errorf("Decide = %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestDecide_AZeroFileBoundNeverEscalatesOnSize(t *testing.T) {
	t.Parallel()
	got := Decide(Inputs{Changed: []string{"a", "b"}, Verdict: "PASS", Known: true}, Rule{})
	if got.Escalate {
		t.Errorf("Decide = %+v, want no escalation when the rule sets no bound", got)
	}
}

func TestRecord_CarriesTheDecisionBesideTheAuditFacts(t *testing.T) {
	t.Parallel()
	rec := Record{Stage: "shadow", Cycle: 7, Verdict: "PASS", ChangedFiles: 1, Decision: Decision{Escalate: true, Reasons: []string{"x"}}}
	if !rec.Escalate || rec.Reasons[0] != "x" {
		t.Errorf("Record = %+v", rec)
	}
	if RecordFile != "audit-cascade-shadow.json" {
		t.Errorf("RecordFile = %q, the name the dossier soak reads", RecordFile)
	}
}
