package recovery

// recovery_adversarial_test.go — cycle-281 test amplification.
// Targets uncovered branches:
//   - Recover with zero-valued input (kind="", CauseUnknown) → unknown-advise
//   - Recover with stuck_no_progress kind (alternative to stuck_no_output)
//   - Recover at exact budget boundary (Attempts == MaxAttempts) → advise
//   - NewChainStallPolicy(0) → defaults maxExtends to 6
//   - NewChainStallPolicy(-1) → also defaults to 6
//   - Promote(nil) / Promote(empty-substr) guard clauses
//   - PromoteSignature(empty-substr) → error
//   - PromoteAdvice: neutralization artifact rejections ([untrusted], ''')

import (
	"errors"
	"strings"
	"testing"
)

// TestRecover_ZeroValuedInput — adversarial: a zero-valued RecoverInput (no
// integrity, no busy, no known cause, no stall kind) falls through to the
// unknown-advise terminal handler.
func TestRecover_ZeroValuedInput(t *testing.T) {
	t.Parallel()
	d := Recover(RecoverInput{})
	if d.Action != ActionAdvise {
		t.Errorf("zero-valued input must reach the unknown-advise terminal; got %s via %s", d.Action, d.Handler)
	}
	if d.Handler == "" {
		t.Error("every decision must carry a handler name")
	}
	if d.Reason == "" {
		t.Error("every decision must carry a justification")
	}
}

// TestRecover_StuckNoProgressKind — adversarial: "stuck_no_progress" (the
// second kind alias) must be handled by stall-budget-extend, same as
// "stuck_no_output".
func TestRecover_StuckNoProgressKind(t *testing.T) {
	t.Parallel()
	d := Recover(RecoverInput{Kind: "stuck_no_progress", Cause: CauseUnknown, Attempts: 1, MaxAttempts: 6})
	if d.Action != ActionExtend {
		t.Errorf("stuck_no_progress within budget must extend; got %s via %s", d.Action, d.Handler)
	}
}

// TestRecover_AtExactBudgetBoundary — adversarial: Attempts == MaxAttempts is
// NOT < MaxAttempts, so stall-budget-extend does not fire; the terminal
// unknown-advise handler must claim it.
func TestRecover_AtExactBudgetBoundary(t *testing.T) {
	t.Parallel()
	d := Recover(RecoverInput{Kind: "stuck_no_output", Cause: CauseUnknown, Attempts: 6, MaxAttempts: 6})
	if d.Action != ActionAdvise {
		t.Errorf("Attempts==MaxAttempts must NOT extend (< is strict); got %s via %s", d.Action, d.Handler)
	}
}

// TestRecover_BusyWithNoKind — adversarial: busy flag with empty kind and
// no cause → busy-extend fires before the terminal.
func TestRecover_BusyWithNoKind(t *testing.T) {
	t.Parallel()
	d := Recover(RecoverInput{Busy: true})
	if d.Action != ActionExtend {
		t.Errorf("busy with no kind must still extend (agent mid-turn); got %s via %s", d.Action, d.Handler)
	}
}

// TestNewChainStallPolicy_ZeroAndNegativeDefaultToSix — adversarial: the guard
// clause that sets maxExtends=6 when the caller passes ≤0.
func TestNewChainStallPolicy_ZeroAndNegativeDefaultToSix(t *testing.T) {
	t.Parallel()
	for _, input := range []int{0, -1, -100} {
		p := NewChainStallPolicy(input)
		// A policy with effective maxExtends=6 will extend on 5 elapsed thresholds
		// (Attempts=5 < 6) and escalate at 6 elapsed (Attempts=6 == 6, not <).
		a5, _ := p.Decide(StallEvent{Kind: "stuck_no_output", IdleS: 5, ThresholdS: 1})
		if a5 != StallExtend {
			t.Errorf("NewChainStallPolicy(%d) at 5 extends must extend (defaulted to 6); got %s", input, a5)
		}
		a6, _ := p.Decide(StallEvent{Kind: "stuck_no_output", IdleS: 6, ThresholdS: 1})
		if a6 != StallEscalate {
			t.Errorf("NewChainStallPolicy(%d) at 6 extends must escalate (budget exhausted); got %s", input, a6)
		}
	}
}

// TestChainStallPolicy_ZeroThresholdNoAttempts — adversarial: when ThresholdS=0
// the policy must not divide-by-zero; Attempts defaults to 0 and the within-
// budget extend fires.
func TestChainStallPolicy_ZeroThresholdNoAttempts(t *testing.T) {
	t.Parallel()
	p := NewChainStallPolicy(6)
	a, _ := p.Decide(StallEvent{Kind: "stuck_no_output", IdleS: 9999, ThresholdS: 0})
	if a != StallExtend {
		t.Errorf("ThresholdS=0 must produce Attempts=0 (no division) → extend; got %s", a)
	}
}

// TestPromote_NilReceiverNoOp — adversarial: calling Promote on a nil
// *FatalPaneDetector must not panic (the nil guard must fire and return).
func TestPromote_NilReceiverNoOp(t *testing.T) {
	t.Parallel()
	var nilD *FatalPaneDetector
	// Must not panic.
	nilD.Promote(FatalSignature{Substr: "something", Cause: CauseDeadShell, Note: "test"})
}

// TestPromote_EmptySubstrNoOp — adversarial: promoting an empty substring is
// a no-op; the detector state must be unchanged.
func TestPromote_EmptySubstrNoOp(t *testing.T) {
	t.Parallel()
	d := SeedDetector()
	pane := "some novel pane content for empty substr test"
	if _, _, ok := d.Detect(pane); ok {
		t.Skip("fixture pane is not novel; fix the test corpus")
	}
	d.Promote(FatalSignature{Substr: "", Cause: CauseDeadShell, Note: "empty-substr guard"})
	if _, _, ok := d.Detect(pane); ok {
		t.Error("empty-substr promotion must be a no-op — novel pane must stay undetected")
	}
}

// TestPromoteSignature_EmptySubstrErrors — adversarial: PromoteSignature with
// an empty substring must return an error (caller must not promote a
// degenerate signature durably).
func TestPromoteSignature_EmptySubstrErrors(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, err := PromoteSignature(dir, FatalSignature{Substr: "", Cause: CauseDeadShell, Note: "empty"})
	if err == nil {
		t.Error("PromoteSignature(empty Substr) = nil error, want error")
	}
}

// TestPromoteAdvice_NeutralizationArtifactsRejected — adversarial: each of the
// three neutralization artifacts must independently cause rejection.
func TestPromoteAdvice_NeutralizationArtifactsRejected(t *testing.T) {
	t.Parallel()
	d := SeedDetector()
	dir := t.TempDir()
	artifacts := []string{"[REDACTED]", "[untrusted]", "'''"}
	for _, artifact := range artifacts {
		pane := "pane content with " + artifact + " embedded inside it for testing purposes"
		err := PromoteAdvice(d, dir, FailureAdvice{
			Cause:         string(CauseDeadShell),
			PaneSubstr:    pane,
			Justification: "test",
		})
		if err == nil {
			t.Errorf("PromoteAdvice with artifact %q must be rejected; got nil error", artifact)
		}
	}
}

// TestPromoteAdvice_SubstrTooShortRejected — adversarial: a substring shorter
// than minPromotedSubstrLen must be rejected to prevent false-positive bombs.
func TestPromoteAdvice_SubstrTooShortRejected(t *testing.T) {
	t.Parallel()
	d := SeedDetector()
	dir := t.TempDir()
	tooShort := strings.Repeat("x", minPromotedSubstrLen-1)
	err := PromoteAdvice(d, dir, FailureAdvice{
		Cause:         string(CauseDeadShell),
		PaneSubstr:    tooShort,
		Justification: "test",
	})
	if err == nil {
		t.Errorf("PromoteAdvice with too-short substring (%d chars) must be rejected", len(tooShort))
	}
}

// TestPromoteAdvice_OutOfVocabularyCauseRejected — adversarial: a cause string
// not in the typed vocabulary must be rejected (hallucinated judgment guard).
func TestPromoteAdvice_OutOfVocabularyCauseRejected(t *testing.T) {
	t.Parallel()
	d := SeedDetector()
	dir := t.TempDir()
	validLen := strings.Repeat("x", minPromotedSubstrLen+5)
	err := PromoteAdvice(d, dir, FailureAdvice{
		Cause:         "not_a_real_cause_value_xyz",
		PaneSubstr:    validLen,
		Justification: "test",
	})
	if err == nil {
		t.Error("PromoteAdvice with out-of-vocabulary cause must be rejected")
	}
	if !strings.Contains(err.Error(), "vocabulary") {
		t.Errorf("error must mention vocabulary; got %q", err.Error())
	}
}

// TestPromoteAdvice_ValidHappyPath — adversarial (positive): a well-formed
// advice with a valid cause, long-enough substring, and no artifacts must
// promote without error and register in-memory.
func TestPromoteAdvice_ValidHappyPath(t *testing.T) {
	t.Parallel()
	d := SeedDetector()
	dir := t.TempDir()
	paneSubstr := "totally novel pane content that has never been seen before in this cycle"
	if _, _, ok := d.Detect(paneSubstr); ok {
		t.Skip("fixture pane already matches a seed — pick a different test string")
	}
	err := PromoteAdvice(d, dir, FailureAdvice{
		Cause:         string(CauseDeadShell),
		PaneSubstr:    paneSubstr,
		Justification: "adversarial happy-path test",
	})
	if err != nil {
		t.Fatalf("PromoteAdvice(valid) = %v, want nil", err)
	}
	// Must be detectable in-memory immediately after promotion.
	if _, _, ok := d.Detect(paneSubstr); !ok {
		t.Error("promoted signature must be immediately detectable in-memory")
	}
}

// Ensure PromoteAdvice is accessible (compilation canary).
var _ = PromoteAdvice
var _ = errors.New
