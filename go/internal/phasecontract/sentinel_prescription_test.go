package phasecontract

import "testing"

func TestSentinel_Prescription(t *testing.T) {
	t.Run("carries_prescription_verbatim", func(t *testing.T) {
		f := &FailureBlock{
			Class:        "risk-foreseen",
			Prescription: []string{"run `git add -f X` or dropIgnoredPaths will silently drop it"},
		}
		line := RenderVerdictSentinelWithFailure("audit", "WARN", f)
		s, ok := ParseVerdictSentinelFull("# Audit Report\n" + line + "\n")
		if !ok {
			t.Fatalf("WARN sentinel with prescription did not parse: %q", line)
		}
		if s.Verdict != "WARN" {
			t.Fatalf("verdict = %q, want WARN", s.Verdict)
		}
		if s.Failure == nil || len(s.Failure.Prescription) != 1 ||
			s.Failure.Prescription[0] != "run `git add -f X` or dropIgnoredPaths will silently drop it" {
			t.Errorf("failure.prescription = %+v, want the single entry verbatim", s.Failure)
		}
	})

	t.Run("omitted_prescription_is_not_synthesized", func(t *testing.T) {
		f := &FailureBlock{Class: "risk-foreseen"}
		line := RenderVerdictSentinelWithFailure("audit", "WARN", f)
		s, ok := ParseVerdictSentinelFull("# Audit Report\n" + line + "\n")
		if !ok {
			t.Fatalf("WARN sentinel without prescription did not parse: %q", line)
		}
		if s.Failure == nil {
			t.Fatalf("failure block must still be present (class was set); got nil")
		}
		if len(s.Failure.Prescription) != 0 {
			t.Errorf("prescription = %+v, want empty — omission must not be promoted to a phantom entry", s.Failure.Prescription)
		}
	})
}
