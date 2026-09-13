package core

import "testing"

// finalizeOutcome unit tests — the cycle-level verdict disambiguator
// extracted from RunCycle. Driven by (lastPhaseVerdict, retroDecision,
// shipped). See cycle_outcome.go for the routing rules; the composed wiring
// (ship latch → finalizeCycle → label) is pinned by
// shipped_via_build_own_ship_test.go and the TestOrchestrator_HappyPath_*
// tests in orchestrator_test.go.

func TestFinalizeOutcome_KeepsFAILUnchanged(t *testing.T) {
	t.Parallel()
	o := &Orchestrator{}
	if got := o.finalizeOutcome(VerdictFAIL, "", false); got != VerdictFAIL {
		t.Errorf("FAIL must pass through, got %q", got)
	}
}

func TestFinalizeOutcome_KeepsPASSUnchanged(t *testing.T) {
	t.Parallel()
	o := &Orchestrator{}
	if got := o.finalizeOutcome(VerdictPASS, "ship: ok", true); got != VerdictPASS {
		t.Errorf("PASS must pass through, got %q", got)
	}
}

func TestFinalizeOutcome_KeepsWARNUnchanged(t *testing.T) {
	t.Parallel()
	o := &Orchestrator{}
	if got := o.finalizeOutcome(VerdictWARN, "", false); got != VerdictWARN {
		t.Errorf("WARN must pass through, got %q", got)
	}
}

func TestFinalizeOutcome_SkippedWithOwnShip_ShippedViaBuild(t *testing.T) {
	t.Parallel()
	o := &Orchestrator{}
	if got := o.finalizeOutcome(VerdictSKIPPED, "proceed: fluent mode", true); got != CycleOutcomeShippedViaBuild {
		t.Errorf("SKIPPED + this cycle's own ship must be SHIPPED_VIA_BUILD, got %q", got)
	}
}

func TestFinalizeOutcome_SkippedWithoutOwnShip_NeverShippedViaBuild(t *testing.T) {
	t.Parallel()
	// Cycle 1630: a sibling lane's landing moved HEAD, but this cycle never
	// shipped. The label must not be inferred from anything but the latch.
	o := &Orchestrator{}
	if got := o.finalizeOutcome(VerdictSKIPPED, "proceed: fluent mode", false); got != CycleOutcomeSkippedUnknown {
		t.Errorf("SKIPPED without this cycle's own ship must be SKIPPED_UNKNOWN, got %q", got)
	}
}

func TestFinalizeOutcome_SkippedWithRetroAdvisory_SkippedAuditAdvisory(t *testing.T) {
	t.Parallel()
	o := &Orchestrator{}
	decision := "proceed: fluent mode (set workflow.strict_audit in policy.json for legacy blocking): would-have-blocked: BLOCK-CODE — 16 non-expired code-audit-fail entries (within 30d retention)"
	if got := o.finalizeOutcome(VerdictSKIPPED, decision, false); got != CycleOutcomeSkippedAuditAdvisory {
		t.Errorf("SKIPPED + would-have-blocked must be SKIPPED_AUDIT_ADVISORY, got %q", got)
	}
}

func TestFinalizeOutcome_SkippedBareNoSignal_SkippedUnknown(t *testing.T) {
	t.Parallel()
	o := &Orchestrator{}
	if got := o.finalizeOutcome(VerdictSKIPPED, "", false); got != CycleOutcomeSkippedUnknown {
		t.Errorf("SKIPPED with no signal must be SKIPPED_UNKNOWN, got %q", got)
	}
}

func TestFinalizeOutcome_OwnShipTrumpsAdvisory(t *testing.T) {
	t.Parallel()
	// If this cycle shipped AND retro had an advisory, the ship wins — the
	// cycle DID deliver value, the advisory is informational.
	o := &Orchestrator{}
	decision := "proceed: fluent mode: would-have-blocked: BLOCK-CODE"
	if got := o.finalizeOutcome(VerdictSKIPPED, decision, true); got != CycleOutcomeShippedViaBuild {
		t.Errorf("own ship should trump advisory, got %q", got)
	}
}
