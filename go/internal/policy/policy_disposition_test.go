package policy

import "testing"

func TestFailureDispositionConfig_DefaultsAndOverride(t *testing.T) {
	t.Parallel()
	def := Policy{}.FailureDispositionConfig()
	if def.Stage != "shadow" || def.Threshold != 2 || def.Step != 0.03 || def.Cap != 0.99 {
		t.Fatalf("defaults = %+v, want {shadow 2 0.03 0.99}", def)
	}
	if def.Enforce() {
		t.Fatal("the shadow default must not enforce")
	}

	proj := Policy{Chronicle: &ChroniclePolicy{Escalation: "enforce"}}.FailureDispositionConfig()
	if !proj.Enforce() {
		t.Fatalf("chronicle.escalation=enforce did not project: %+v", proj)
	}

	over := Policy{
		Chronicle:          &ChroniclePolicy{Escalation: "enforce"},
		FailureDisposition: &FailureDispositionPolicy{Stage: "shadow", Threshold: 5},
	}.FailureDispositionConfig()
	if over.Enforce() || over.Threshold != 5 || over.Step != 0.03 || over.Cap != 0.99 {
		t.Fatalf("override = %+v, want stage shadow, threshold 5, defaults elsewhere", over)
	}

	full := Policy{FailureDisposition: &FailureDispositionPolicy{Stage: "enforce", Threshold: 3, Step: 0.05, Cap: 0.95}}.FailureDispositionConfig()
	if !full.Enforce() || full.Threshold != 3 || full.Step != 0.05 || full.Cap != 0.95 {
		t.Fatalf("full override = %+v", full)
	}
}

func TestFailureDispositionConfig_UnknownStageIsReportOnly(t *testing.T) {
	t.Parallel()
	c := Policy{FailureDisposition: &FailureDispositionPolicy{Stage: "banana"}}.FailureDispositionConfig()
	if c.Enforce() {
		t.Fatal("an unknown stage must be report-only")
	}
}
