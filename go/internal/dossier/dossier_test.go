package dossier

import (
	"strings"
	"testing"
)

func passDossier() *Dossier {
	return &Dossier{
		Cycle:        7,
		Goal:         "reduce flags",
		FinalVerdict: VerdictPass,
		Phases:       []PhaseRecord{{Name: "build", Verdict: VerdictPass}, {Name: "audit", Verdict: VerdictPass}},
	}
}

func failDossier() *Dossier {
	return &Dossier{
		Cycle:        8,
		Goal:         "add campaign driver",
		FinalVerdict: VerdictFail,
		Phases:       []PhaseRecord{{Name: "build", Verdict: VerdictPass}, {Name: "audit", Verdict: VerdictFail}},
		Defects:      []Defect{{ID: "H1", Severity: "HIGH", Summary: "unbounded fan-out", Fix: "bound Verify + finite Concurrency"}},
		Carryover:    []Carryover{{ID: "c8-fix-H1", Action: "bound fan-out", Priority: "P0"}},
	}
}

func TestValidate_OK(t *testing.T) {
	if err := passDossier().Validate(); err != nil {
		t.Errorf("PASS dossier: %v", err)
	}
	if err := failDossier().Validate(); err != nil {
		t.Errorf("FAIL dossier (with defects+carryover): %v", err)
	}
}

func TestValidate_Errors(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Dossier)
		want   string
	}{
		{"cycle zero", func(d *Dossier) { d.Cycle = 0 }, "cycle must be >= 1"},
		{"empty goal", func(d *Dossier) { d.Goal = "  " }, "goal is empty"},
		{"bad verdict", func(d *Dossier) { d.FinalVerdict = "OK" }, "final_verdict"},
		{"no phases", func(d *Dossier) { d.Phases = nil }, "no phases"},
		{"phase empty name", func(d *Dossier) { d.Phases[0].Name = "" }, "empty name"},
		{"phase bad verdict", func(d *Dossier) { d.Phases[0].Verdict = "MAYBE" }, "verdict"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := passDossier()
			tc.mutate(d)
			err := d.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("want error containing %q, got %v", tc.want, err)
			}
		})
	}
}

// TestValidate_FailMustCarryExperience is the load-bearing invariant: a FAILED
// cycle cannot be recorded without WHY (>=1 defect) and the fix work (>=1
// carryover) — so failed verdicts are first-class, compounding experience.
func TestValidate_FailMustCarryExperience(t *testing.T) {
	noDefects := failDossier()
	noDefects.Defects = nil
	if err := noDefects.Validate(); err == nil || !strings.Contains(err.Error(), "defect") {
		t.Errorf("FAIL without defects must error, got %v", err)
	}
	noCarryover := failDossier()
	noCarryover.Carryover = nil
	if err := noCarryover.Validate(); err == nil || !strings.Contains(err.Error(), "carryover") {
		t.Errorf("FAIL without carryover must error, got %v", err)
	}
}

func TestValidate_DefectAndCarryoverWellFormed(t *testing.T) {
	d := failDossier()
	d.Defects[0].Summary = ""
	if err := d.Validate(); err == nil || !strings.Contains(err.Error(), "defect") {
		t.Errorf("malformed defect must error, got %v", err)
	}
	d = failDossier()
	d.Carryover[0].Action = ""
	if err := d.Validate(); err == nil || !strings.Contains(err.Error(), "carryover") {
		t.Errorf("malformed carryover must error, got %v", err)
	}
}
