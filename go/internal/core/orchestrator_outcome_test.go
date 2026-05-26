package core

import "testing"

// finalizeOutcome unit tests — the cycle-level verdict disambiguator
// extracted from RunCycle. Driven by (lastPhaseVerdict, retroDecision,
// preCycleHEAD, postCycleHEAD). See orchestrator.go for the routing
// rules. Wiring (gitHEAD seam + capture-on-start) is covered by the
// existing TestOrchestrator_HappyPath_* tests in orchestrator_test.go.

func TestFinalizeOutcome_KeepsFAILUnchanged(t *testing.T) {
	o := &Orchestrator{}
	if got := o.finalizeOutcome(VerdictFAIL, "", "abc", "abc"); got != VerdictFAIL {
		t.Errorf("FAIL must pass through, got %q", got)
	}
}

func TestFinalizeOutcome_KeepsPASSUnchanged(t *testing.T) {
	o := &Orchestrator{}
	if got := o.finalizeOutcome(VerdictPASS, "ship: ok", "abc", "abc"); got != VerdictPASS {
		t.Errorf("PASS must pass through, got %q", got)
	}
}

func TestFinalizeOutcome_KeepsWARNUnchanged(t *testing.T) {
	o := &Orchestrator{}
	if got := o.finalizeOutcome(VerdictWARN, "", "abc", "abc"); got != VerdictWARN {
		t.Errorf("WARN must pass through, got %q", got)
	}
}

func TestFinalizeOutcome_SkippedWithHeadMoved_ShippedViaBuild(t *testing.T) {
	o := &Orchestrator{}
	if got := o.finalizeOutcome(VerdictSKIPPED, "proceed: fluent mode", "abc123", "def456"); got != CycleOutcomeShippedViaBuild {
		t.Errorf("SKIPPED + HEAD moved must be SHIPPED_VIA_BUILD, got %q", got)
	}
}

func TestFinalizeOutcome_SkippedWithRetroAdvisory_SkippedAuditAdvisory(t *testing.T) {
	o := &Orchestrator{}
	decision := "proceed: fluent mode (set EVOLVE_STRICT_AUDIT=1 for legacy blocking): would-have-blocked: BLOCK-CODE — 16 non-expired code-audit-fail entries (within 30d retention)"
	if got := o.finalizeOutcome(VerdictSKIPPED, decision, "abc", "abc"); got != CycleOutcomeSkippedAuditAdvisory {
		t.Errorf("SKIPPED + would-have-blocked must be SKIPPED_AUDIT_ADVISORY, got %q", got)
	}
}

func TestFinalizeOutcome_SkippedBareNoSignal_SkippedUnknown(t *testing.T) {
	o := &Orchestrator{}
	if got := o.finalizeOutcome(VerdictSKIPPED, "", "abc", "abc"); got != CycleOutcomeSkippedUnknown {
		t.Errorf("SKIPPED with no signal must be SKIPPED_UNKNOWN, got %q", got)
	}
}

func TestFinalizeOutcome_HeadMovedTrumpsAdvisory(t *testing.T) {
	// If HEAD moved AND retro had an advisory, the commit wins — the
	// cycle DID ship value, the advisory is informational.
	o := &Orchestrator{}
	decision := "proceed: fluent mode: would-have-blocked: BLOCK-CODE"
	if got := o.finalizeOutcome(VerdictSKIPPED, decision, "abc", "def"); got != CycleOutcomeShippedViaBuild {
		t.Errorf("HEAD moved should trump advisory, got %q", got)
	}
}

func TestFinalizeOutcome_EmptyHeads_FallsBackToUnknown(t *testing.T) {
	// gitHEAD seam might return "" on probe failure — treat as "no
	// movement detected" rather than crashing.
	o := &Orchestrator{}
	if got := o.finalizeOutcome(VerdictSKIPPED, "", "", ""); got != CycleOutcomeSkippedUnknown {
		t.Errorf("empty heads with no signal = SKIPPED_UNKNOWN, got %q", got)
	}
}
