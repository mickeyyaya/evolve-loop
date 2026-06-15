package phaseio

import "testing"

func TestVerdict_IsValid(t *testing.T) {
	valid := []Verdict{VerdictPASS, VerdictFAIL, VerdictWARN, VerdictSKIPPED}
	for _, v := range valid {
		if !v.IsValid() {
			t.Errorf("%q should be valid", v)
		}
	}
	for _, v := range []Verdict{"", "pass", "PASSED", "ok"} {
		if v.IsValid() {
			t.Errorf("%q should be invalid", v)
		}
	}
}

func TestPhaseOutput_IsPass(t *testing.T) {
	if !(PhaseOutput{Verdict: VerdictPASS}).IsPass() {
		t.Errorf("PASS output should IsPass")
	}
	for _, v := range []Verdict{VerdictFAIL, VerdictWARN, VerdictSKIPPED, ""} {
		if (PhaseOutput{Verdict: v}).IsPass() {
			t.Errorf("%q output should not IsPass", v)
		}
	}
}

func TestPhaseOutput_HasFailure(t *testing.T) {
	if (PhaseOutput{}).HasFailure() {
		t.Errorf("no failure block → HasFailure false")
	}
	out := PhaseOutput{
		Verdict: VerdictFAIL,
		Failure: &FailureBlock{
			Class:   "build_error",
			Summary: "compile failed",
			Defects: []Defect{{Severity: "HIGH", Title: "undefined symbol", Detail: "x not declared"}},
		},
	}
	if !out.HasFailure() {
		t.Errorf("failure block present → HasFailure true")
	}
	if out.Failure.Defects[0].Severity != "HIGH" {
		t.Errorf("defect round-trip: %+v", out.Failure.Defects[0])
	}
}

func TestPhaseOutput_Fields_RoundTrip(t *testing.T) {
	out := PhaseOutput{
		Phase:           "audit",
		Verdict:         VerdictPASS,
		Signals:         map[string]any{"audit.red_count": float64(0)},
		NextPhase:       "ship",
		CommitSHA:       "abc123",
		WorktreeTreeSHA: "tree789",
		Reconciled:      true,
	}
	if out.Phase != "audit" || !out.IsPass() || out.NextPhase != "ship" ||
		out.CommitSHA != "abc123" || out.WorktreeTreeSHA != "tree789" || !out.Reconciled {
		t.Fatalf("PhaseOutput round-trip: %+v", out)
	}
	if v, ok := out.Signals["audit.red_count"]; !ok || v != float64(0) {
		t.Fatalf("signal round-trip: (%v, %v)", v, ok)
	}
}
