package core

// decision_branch_repair_test.go — RED contract for the audit-repair branch.
//
// This is the file that proves ADR-0072 survives the change. The repair loop
// narrows exactly ONE authority — an agent-authored floor category that is
// contradicted by BOTH the deterministic dossier candidate AND the agent's own
// disposition — and nothing else. Every other halt path must behave byte-identically,
// which is why the contrast case below writes the same failure-decision.json as
// TestDecideAfterRetroFloor_Cycle1001JudgmentHalt and differs ONLY by the presence
// of disposition.json.
//
// Live evidence this encodes (wave 3, 2026-08-27): cycles 1572/1573/1574 each
// halted under gate 2 on an agent "infra-systemic" category, while their
// failure-dossier.json recorded floor_candidate:"" and the same agent's
// disposition.json recorded legitimacy:"legit-rejection". 367 minutes of lane
// time produced three FAILs that two deterministic signals called task-level.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// writeDisposition writes the retro agent's disposition.json. Only the fields
// the repair rule reads are required; the gate's full schema is exercised by
// disposition_gate_test.go and is deliberately not duplicated here.
func writeDisposition(t *testing.T, dir, legitimacy string) {
	t.Helper()
	body := `{"cycle":1573,"fingerprint":"audit|verdict-fail|deadbeef","recurrence":0,` +
		`"legitimacy":"` + legitimacy + `","root_cause":{"layer":"task-code","summary":"staged index carries an out-of-lane stub"},` +
		`"salvage":{"worktree_has_value":true,"pointer":"wt"},"urgency":"P2",` +
		`"justification":"the auditor was right","routing":"carryover","proposed_item":"x"}`
	if err := os.WriteFile(filepath.Join(dir, "disposition.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write disposition: %v", err)
	}
}

// agentFloorClaim is the exact failure-decision.json body from the cycle-1001
// judgment-halt test. Shared by both cases below so the ONLY variable between
// "halts" and "repairs" is the disposition.
const agentFloorClaim = `{"category":"infra-systemic","level":"system","evidence":"prose-declared SYSTEM-class","action":"halt-and-diagnose","fix_type":"pipeline-repair"}`

// The wave-3 shape. Deterministic gate silent + agent claims a floor + the same
// agent's disposition says legit-rejection ⇒ the contradiction is recorded and
// the cycle is REPAIRED rather than halted.
func TestDecideAfterRetro_RepairsWhenAgentFloorIsContradicted(t *testing.T) {
	o := floorOrchestrator(fixedNextStrategy{next: "tdd"})
	o.maxAuditRepairAttempts = 2
	dir := t.TempDir()
	// code-audit-fail keeps the DETERMINISTIC dossier candidate empty.
	writeAuditWithFailure(t, dir, "FAIL", "code-audit-fail", "H1 staged out-of-lane phase stub")
	writeDecision(t, dir, agentFloorClaim)
	writeDisposition(t, dir, "legit-rejection")
	cs := CycleState{CycleID: 1573, WorkspacePath: dir}

	next, _, reason, sig := o.decideAfterRetro(cs, VerdictFAIL, nil)

	if sig != nil {
		t.Fatalf("a contradicted agent floor claim must NOT halt; sig=%+v", sig)
	}
	if next != PhaseTDD {
		t.Errorf("next = %s, want tdd (repair re-enters at the test-first phase)", next)
	}
	if !strings.Contains(reason, "audit-repair") {
		t.Errorf("reason = %q, want it to name the audit-repair branch", reason)
	}
}

// THE GUARD. Byte-identical inputs to the case above MINUS disposition.json.
// Absence of evidence must not grant repair, so this must still halt exactly as
// TestDecideAfterRetroFloor_Cycle1001JudgmentHalt does today.
func TestDecideAfterRetro_HaltsWhenNoDispositionContradictsTheAgent(t *testing.T) {
	o := floorOrchestrator(fixedNextStrategy{next: "tdd"})
	o.maxAuditRepairAttempts = 2
	dir := t.TempDir()
	writeAuditWithFailure(t, dir, "FAIL", "code-audit-fail", "H1 staged out-of-lane phase stub")
	writeDecision(t, dir, agentFloorClaim)
	// no disposition.json on purpose
	cs := CycleState{CycleID: 1001, WorkspacePath: dir}

	next, _, _, sig := o.decideAfterRetro(cs, VerdictFAIL, nil)

	if sig == nil || !sig.Halt {
		t.Fatalf("an uncontradicted agent floor claim must still halt; sig=%+v", sig)
	}
	if next != PhaseEnd {
		t.Errorf("next = %s, want end", next)
	}
}

// Gate 1 stays absolute: a deterministic floor candidate cannot be bought off
// with a friendly disposition.
func TestDecideAfterRetro_DeterministicFloorBeatsLegitRejection(t *testing.T) {
	o := floorOrchestrator(fixedNextStrategy{next: "tdd"})
	o.maxAuditRepairAttempts = 2
	dir := t.TempDir()
	// infra-systemic here DOES produce a deterministic dossier candidate.
	writeAuditWithFailure(t, dir, "FAIL", "infra-systemic", "all CLI families exhausted")
	writeDisposition(t, dir, "legit-rejection")
	cs := CycleState{CycleID: 1001, WorkspacePath: dir}

	next, _, _, sig := o.decideAfterRetro(cs, VerdictFAIL, nil)

	if sig == nil || !sig.Halt {
		t.Fatalf("deterministic floor candidate must halt regardless of disposition; sig=%+v", sig)
	}
	if next != PhaseEnd {
		t.Errorf("next = %s, want end", next)
	}
}

// The cap is real: at MaxAttempts the cycle falls through to today's behavior.
func TestDecideAfterRetro_RepairStopsAtTheCap(t *testing.T) {
	o := floorOrchestrator(fixedNextStrategy{next: "tdd"})
	o.maxAuditRepairAttempts = 2
	dir := t.TempDir()
	writeAuditWithFailure(t, dir, "FAIL", "code-audit-fail", "H1 staged out-of-lane phase stub")
	writeDecision(t, dir, agentFloorClaim)
	writeDisposition(t, dir, "legit-rejection")
	cs := CycleState{CycleID: 1573, WorkspacePath: dir, AuditRepairAttempts: 2}

	next, _, _, sig := o.decideAfterRetro(cs, VerdictFAIL, nil)

	if next == PhaseTDD {
		t.Error("next = tdd at the cap; repair must stop after MaxAttempts")
	}
	if sig == nil || !sig.Halt {
		t.Fatalf("at the cap the uncontradicted-agent halt applies again; sig=%+v", sig)
	}
}

// Repair disabled by configuration (cap 0) behaves exactly as today.
func TestDecideAfterRetro_CapZeroDisablesRepair(t *testing.T) {
	o := floorOrchestrator(fixedNextStrategy{next: "tdd"})
	o.maxAuditRepairAttempts = 0
	dir := t.TempDir()
	writeAuditWithFailure(t, dir, "FAIL", "code-audit-fail", "H1 staged out-of-lane phase stub")
	writeDecision(t, dir, agentFloorClaim)
	writeDisposition(t, dir, "legit-rejection")
	cs := CycleState{CycleID: 1573, WorkspacePath: dir}

	_, _, _, sig := o.decideAfterRetro(cs, VerdictFAIL, nil)

	if sig == nil || !sig.Halt {
		t.Fatalf("cap 0 disables repair, so the agent floor claim halts; sig=%+v", sig)
	}
}

// THE LIVE PATH. decideAfterRetroRouted is what the loop actually calls; every
// test above drives the deterministic decideAfterRetro underneath it. A repair
// granted deterministically and then handed to the router can be OVERRIDDEN by
// the strategy — eating the one bounded recovery the grant exists to provide,
// with every unit test still green. That is the unit-green-is-not-live-green
// shape, and it is why the sibling bookkeeping regrade returns ABOVE the router
// with the comment "letting the strategy override it to tdd/end would eat the
// one bounded re-audit the grant exists to provide".
//
// The strategy here proposes "end" — the override that would silently discard
// the repair.
func TestDecideAfterRetroRouted_RepairSurvivesTheRouter(t *testing.T) {
	o := floorOrchestrator(fixedNextStrategy{next: "end"})
	o.maxAuditRepairAttempts = 2
	dir := t.TempDir()
	writeAuditWithFailure(t, dir, "FAIL", "code-audit-fail", "H1 staged out-of-lane phase stub")
	writeDecision(t, dir, agentFloorClaim)
	writeDisposition(t, dir, "legit-rejection")
	cs := CycleState{CycleID: 1573, WorkspacePath: dir}

	next, _, reason, sig := o.decideAfterRetroRouted(
		context.Background(), 1573, cs, 1, VerdictFAIL, nil, router.RouteInput{})

	if sig != nil {
		t.Fatalf("a contradicted agent floor claim must not halt on the live path; sig=%+v", sig)
	}
	if next != PhaseTDD {
		t.Errorf("next = %s, want tdd — the router ate the repair grant", next)
	}
	if !strings.Contains(reason, "audit-repair") {
		t.Errorf("reason = %q, want the audit-repair contract string preserved through the routed path", reason)
	}
}
