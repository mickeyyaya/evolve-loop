package core

// failreasons_backfill_test.go — RED contract for "no FAIL without a reason"
// (inbox null-failreasons-capture 0.85): cycles failing at phase-infra level
// (launch refusal, timeout, abort) sealed FinalVerdict FAIL with FailReasons
// null — retros fingerprinted nothing (content-free identity, the bc2e3236
// gate-block class in task-FAIL form) and the breaker could not see genuine
// recurrence. Three instances in one night. The seal now backfills from the
// trusted phase-timing record (orchestrator memory, never a workspace file).

import (
	"strings"
	"testing"
)

func TestBackfillFailReasons_FromAbortReasons(t *testing.T) {
	t.Parallel()
	result := &CycleResult{FinalVerdict: VerdictFAIL}
	backfillFailReasons(result, []phaseTimingEntry{
		{Phase: "build", Verdict: VerdictPASS},
		{Phase: "audit", Verdict: VerdictFAIL, AbortReason: "bridge launch refused: fleet mode: explicit worktree required"},
	})
	if len(result.FailReasons) != 1 {
		t.Fatalf("FailReasons = %v, want the one abort-derived reason", result.FailReasons)
	}
	r := result.FailReasons[0]
	if !strings.Contains(r, "audit") || !strings.Contains(r, "bridge launch refused") {
		t.Errorf("reason %q must carry the originating phase and the error head", r)
	}
}

func TestBackfillFailReasons_UnexplainedMarkerNeverNull(t *testing.T) {
	t.Parallel()
	result := &CycleResult{FinalVerdict: VerdictFAIL}
	backfillFailReasons(result, []phaseTimingEntry{{Phase: "build", Verdict: VerdictFAIL}})
	if len(result.FailReasons) == 0 {
		t.Fatal("a sealed FAIL with FailReasons null must be structurally impossible")
	}
	if !strings.Contains(result.FailReasons[0], "build") {
		t.Errorf("the fallback marker should still name the failing phase: %q", result.FailReasons[0])
	}

	// Even with NO failing phase on record (pure infra death before dispatch),
	// the seal writes an explicit unexplained marker rather than null.
	empty := &CycleResult{FinalVerdict: VerdictFAIL}
	backfillFailReasons(empty, nil)
	if len(empty.FailReasons) == 0 {
		t.Fatal("reason-less FAIL sealed with null FailReasons — the exact class this closes")
	}
}

// A RECOVERED transient's abort entry (ship-recovery records the error it
// recovered from, then continues) must not mask the phase that actually
// failed later (diff-review MEDIUM).
func TestBackfillFailReasons_RecoveredAbortDoesNotMaskRealFailure(t *testing.T) {
	t.Parallel()
	result := &CycleResult{FinalVerdict: VerdictFAIL}
	backfillFailReasons(result, []phaseTimingEntry{
		{Phase: "ship", AbortReason: "ship error E_PUSH: recovering via build (attempt 1/2)"},
		{Phase: "build", Verdict: VerdictFAIL},
	})
	joined := strings.Join(result.FailReasons, "\n")
	if !strings.Contains(joined, "ship error E_PUSH") || !strings.Contains(joined, "phase build:") {
		t.Fatalf("recovered transient masked the real failing phase (or vice versa):\n%s", joined)
	}
}

func TestBackfillFailReasons_NoOpWhenExplainedOrNotFAIL(t *testing.T) {
	t.Parallel()
	explained := &CycleResult{FinalVerdict: VerdictFAIL, FailReasons: []string{"audit gate: disposition-preflight: MISSING"}}
	backfillFailReasons(explained, []phaseTimingEntry{{Phase: "audit", AbortReason: "noise"}})
	if len(explained.FailReasons) != 1 || !strings.Contains(explained.FailReasons[0], "disposition-preflight") {
		t.Errorf("an already-explained FAIL must not be rewritten: %v", explained.FailReasons)
	}
	pass := &CycleResult{FinalVerdict: VerdictPASS}
	backfillFailReasons(pass, []phaseTimingEntry{{Phase: "build", AbortReason: "noise"}})
	if len(pass.FailReasons) != 0 {
		t.Errorf("a PASS cycle must not gain fail reasons: %v", pass.FailReasons)
	}
}

// A phase that returns FAIL by its OWN Classify — triage's protected-surface
// admission rejection (cycles 1634 and 1636, 2026-09-12/13) — recorded no
// abort reason, so the seal labelled it "phase-infra class": a deterministic,
// reasoned rejection paged as infrastructure. The C1 record now carries the
// phase's own diagnostics; the seal names the error-severity ones and keeps the
// infra marker only for a FAIL that truly recorded nothing.
func TestBackfillFailReasons_PhaseOwnDiagnosticsNameTheReason(t *testing.T) {
	t.Parallel()
	result := &CycleResult{FinalVerdict: VerdictFAIL}
	backfillFailReasons(result, []phaseTimingEntry{
		{Phase: "scout", Verdict: VerdictPASS},
		{Phase: "triage", Verdict: VerdictFAIL, Diagnostics: []Diagnostic{
			{Severity: "warning", Message: "phase-tracker metrics file absent"},
			{Severity: "error", Message: `top_n card "phase-stub-shape-rule-at-ship-staging" names protected surface "go/internal/phases/ship/gitops.go" — control-plane changes go through the console route (operator-gated), not lane top_n`},
		}},
	})
	if len(result.FailReasons) != 1 {
		t.Fatalf("FailReasons = %v, want exactly the triage reason", result.FailReasons)
	}
	r := result.FailReasons[0]
	if !strings.HasPrefix(r, "phase triage: ") || !strings.Contains(r, "names protected surface") {
		t.Errorf("reason must carry the phase and its own error diagnostic: %q", r)
	}
	if strings.Contains(r, "phase-infra class") || strings.Contains(r, "metrics file absent") {
		t.Errorf("a reasoned FAIL must not be labelled infra, and warnings are not reasons: %q", r)
	}

	// Warnings alone explain nothing: the infra marker stays for that shape.
	warnOnly := &CycleResult{FinalVerdict: VerdictFAIL}
	backfillFailReasons(warnOnly, []phaseTimingEntry{
		{Phase: "triage", Verdict: VerdictFAIL, Diagnostics: []Diagnostic{{Severity: "warning", Message: "metrics file absent"}}},
	})
	if len(warnOnly.FailReasons) != 1 || !strings.Contains(warnOnly.FailReasons[0], "phase-infra class") {
		t.Errorf("warning-only FAIL must keep the explicit infra marker: %v", warnOnly.FailReasons)
	}
}
