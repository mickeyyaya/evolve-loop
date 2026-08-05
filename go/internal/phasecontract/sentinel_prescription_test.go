package phasecontract

import "testing"

// sentinel_prescription_test.go — RED contract for cycle-1327's
// `audit-warn-prescription-gate` (batch-integrity-review-2026-08-04.md F3,
// weight 0.91): a WARN audit can prescribe a remediation in prose with no
// machine-readable trace, so the fix is never enforced and silently vanishes
// (cycle-1258 lesson). This file pins the wire-schema half: FailureBlock gains
// a `prescription` channel, distinct from `defects`, that round-trips through
// the sentinel exactly like every other field ADR-0039 §7 added.

// TestSentinel_Prescription — Criterion 1 of
// .evolve/evals/audit-warn-prescription-gate.md. A WARN sentinel whose failure
// block carries a non-empty `prescription` array must parse it verbatim onto
// VerdictSentinel.Failure.Prescription. Negative: an omitted `prescription`
// field must parse to a nil/empty slice, never a synthesized entry — an
// implementer defaulting the omitted case to some placeholder would be
// inventing agent output the auditor never wrote.
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
