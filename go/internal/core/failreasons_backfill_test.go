package core

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

	empty := &CycleResult{FinalVerdict: VerdictFAIL}
	backfillFailReasons(empty, nil)
	if len(empty.FailReasons) == 0 {
		t.Fatal("reason-less FAIL sealed with null FailReasons — the exact class this closes")
	}
}

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

	warnOnly := &CycleResult{FinalVerdict: VerdictFAIL}
	backfillFailReasons(warnOnly, []phaseTimingEntry{
		{Phase: "triage", Verdict: VerdictFAIL, Diagnostics: []Diagnostic{{Severity: "warning", Message: "metrics file absent"}}},
	})
	if len(warnOnly.FailReasons) != 1 || !strings.Contains(warnOnly.FailReasons[0], "phase-infra class") {
		t.Errorf("warning-only FAIL must keep the explicit infra marker: %v", warnOnly.FailReasons)
	}
}
