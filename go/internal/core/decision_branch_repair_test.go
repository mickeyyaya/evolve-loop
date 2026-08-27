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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// writeDisposition writes the retro agent's disposition.json. Only the fields
// the repair rule reads are required; the gate's full schema is exercised by
// disposition_gate_test.go and is deliberately not duplicated here.
const repairFixtureFingerprint = "audit|verdict-fail|deadbeef"

func writeDisposition(t *testing.T, dir, legitimacy string) {
	t.Helper()
	writeDispositionIdentity(t, dir, legitimacy, repairFixtureFingerprint, 1573)
}

// writeDispositionIdentity varies the identity fields so the cross-check can be
// exercised. A disposition is agent-authored PROSE about a failure; the
// fingerprint/recurrence are the only part of it a machine computed, which is why
// they are what must agree with the digest.
func writeDispositionIdentity(t *testing.T, dir, legitimacy, fingerprint string, cycle int) {
	t.Helper()
	body := fmt.Sprintf(`{"cycle":%d,"fingerprint":%q,"recurrence":0,`+
		`"legitimacy":%q,"root_cause":{"layer":"task-code","summary":"staged index carries an out-of-lane stub"},`+
		`"salvage":{"worktree_has_value":true,"pointer":"wt"},"urgency":"P2",`+
		`"justification":"the auditor was right","routing":"carryover","proposed_item":"x"}`,
		cycle, fingerprint, legitimacy)
	if err := os.WriteFile(filepath.Join(dir, "disposition.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write disposition: %v", err)
	}
}

// writeFailureDigest writes the DETERMINISTIC failure identity the disposition
// must agree with.
func writeFailureDigest(t *testing.T, dir, fingerprint string, cycle int) {
	t.Helper()
	body := fmt.Sprintf(`{"cycle":%d,"fingerprint":%q,"pre_class":"verdict-fail","recurrence":0}`, cycle, fingerprint)
	if err := os.WriteFile(filepath.Join(dir, "failure-digest.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write failure-digest: %v", err)
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
	o.workflowConfig.MaxAuditRepairAttempts = 2
	dir := t.TempDir()
	// code-audit-fail keeps the DETERMINISTIC dossier candidate empty.
	writeAuditWithFailure(t, dir, "FAIL", "code-audit-fail", "H1 staged out-of-lane phase stub")
	writeDecision(t, dir, agentFloorClaim)
	writeDisposition(t, dir, "legit-rejection")
	writeFailureDigest(t, dir, repairFixtureFingerprint, 1573)
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
	o.workflowConfig.MaxAuditRepairAttempts = 2
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
	o.workflowConfig.MaxAuditRepairAttempts = 2
	dir := t.TempDir()
	// infra-systemic here DOES produce a deterministic dossier candidate.
	writeAuditWithFailure(t, dir, "FAIL", "infra-systemic", "all CLI families exhausted")
	writeDisposition(t, dir, "legit-rejection")
	writeFailureDigest(t, dir, repairFixtureFingerprint, 1001)
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
	o.workflowConfig.MaxAuditRepairAttempts = 2
	dir := t.TempDir()
	writeAuditWithFailure(t, dir, "FAIL", "code-audit-fail", "H1 staged out-of-lane phase stub")
	writeDecision(t, dir, agentFloorClaim)
	writeDisposition(t, dir, "legit-rejection")
	writeFailureDigest(t, dir, repairFixtureFingerprint, 1573)
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
	o.workflowConfig.MaxAuditRepairAttempts = 0
	dir := t.TempDir()
	writeAuditWithFailure(t, dir, "FAIL", "code-audit-fail", "H1 staged out-of-lane phase stub")
	writeDecision(t, dir, agentFloorClaim)
	writeDisposition(t, dir, "legit-rejection")
	writeFailureDigest(t, dir, repairFixtureFingerprint, 1573)
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
	o.workflowConfig.MaxAuditRepairAttempts = 2
	dir := t.TempDir()
	writeAuditWithFailure(t, dir, "FAIL", "code-audit-fail", "H1 staged out-of-lane phase stub")
	writeDecision(t, dir, agentFloorClaim)
	writeDisposition(t, dir, "legit-rejection")
	writeFailureDigest(t, dir, repairFixtureFingerprint, 1573)
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

// ---- C1: the disposition is agent-authored PROSE and must not be trusted on its
// own word. crossCheckAgainstDigest exists precisely to stop an agent inventing a
// failure identity, and the repair rule was reading legitimacy straight past it.
// A fabricated, stale, or copied disposition could therefore convert a genuine
// ADR-0072 system-failure HALT into a granted repair.

func TestDecideAfterRetro_RefusesRepairOnFabricatedFailureIdentity(t *testing.T) {
	o := floorOrchestrator(fixedNextStrategy{next: "tdd"})
	o.workflowConfig.MaxAuditRepairAttempts = 2
	dir := t.TempDir()
	writeAuditWithFailure(t, dir, "FAIL", "code-audit-fail", "H1 staged out-of-lane phase stub")
	writeDecision(t, dir, agentFloorClaim)
	// legit-rejection, but naming a failure identity no assembler ever computed.
	writeDispositionIdentity(t, dir, "legit-rejection", "audit|verdict-fail|fabricated", 1573)
	writeFailureDigest(t, dir, repairFixtureFingerprint, 1573)
	cs := CycleState{CycleID: 1573, WorkspacePath: dir}

	next, _, _, sig := o.decideAfterRetro(cs, VerdictFAIL, nil)

	if sig == nil || !sig.Halt {
		t.Fatalf("a disposition whose fingerprint disagrees with the digest must not buy a repair; sig=%+v", sig)
	}
	if next == PhaseTDD {
		t.Error("next = tdd on a fabricated failure identity")
	}
}

// A disposition left over from a DIFFERENT cycle in the same workspace is stale
// evidence about someone else's failure.
func TestDecideAfterRetro_RefusesRepairOnForeignCycleDisposition(t *testing.T) {
	o := floorOrchestrator(fixedNextStrategy{next: "tdd"})
	o.workflowConfig.MaxAuditRepairAttempts = 2
	dir := t.TempDir()
	writeAuditWithFailure(t, dir, "FAIL", "code-audit-fail", "H1 staged out-of-lane phase stub")
	writeDecision(t, dir, agentFloorClaim)
	writeDispositionIdentity(t, dir, "legit-rejection", repairFixtureFingerprint, 1499)
	writeFailureDigest(t, dir, repairFixtureFingerprint, 1573)
	cs := CycleState{CycleID: 1573, WorkspacePath: dir}

	_, _, _, sig := o.decideAfterRetro(cs, VerdictFAIL, nil)

	if sig == nil || !sig.Halt {
		t.Fatalf("a disposition naming a different cycle must not buy a repair; sig=%+v", sig)
	}
}

// No digest at all means the identity is UNVERIFIABLE, which is not the same as
// verified-good. Absence of evidence never grants repair.
func TestDecideAfterRetro_RefusesRepairWhenIdentityIsUnverifiable(t *testing.T) {
	o := floorOrchestrator(fixedNextStrategy{next: "tdd"})
	o.workflowConfig.MaxAuditRepairAttempts = 2
	dir := t.TempDir()
	writeAuditWithFailure(t, dir, "FAIL", "code-audit-fail", "H1 staged out-of-lane phase stub")
	writeDecision(t, dir, agentFloorClaim)
	writeDisposition(t, dir, "legit-rejection")
	// no failure-digest.json on purpose
	cs := CycleState{CycleID: 1573, WorkspacePath: dir}

	_, _, _, sig := o.decideAfterRetro(cs, VerdictFAIL, nil)

	if sig == nil || !sig.Halt {
		t.Fatalf("an unverifiable disposition identity must not buy a repair; sig=%+v", sig)
	}
}

// M5: when the budget is spent, the HALT must still say WHY it is the same
// classifier-contradiction class that motivated this feature. Without this an
// operator sees a bare "orchestrator-classified infra-systemic" and has no way to
// tell it from an ordinary system failure — the contradiction was computed,
// documented as "a forensics signal", and then discarded.
func TestDecideAfterRetro_ExhaustedRepairHaltNamesTheContradiction(t *testing.T) {
	o := floorOrchestrator(fixedNextStrategy{next: "tdd"})
	o.workflowConfig.MaxAuditRepairAttempts = 2
	dir := t.TempDir()
	writeAuditWithFailure(t, dir, "FAIL", "code-audit-fail", "H1 staged out-of-lane phase stub")
	writeDecision(t, dir, agentFloorClaim)
	writeDisposition(t, dir, "legit-rejection")
	writeFailureDigest(t, dir, repairFixtureFingerprint, 1573)
	cs := CycleState{CycleID: 1573, WorkspacePath: dir, AuditRepairAttempts: 2}

	_, _, _, sig := o.decideAfterRetro(cs, VerdictFAIL, nil)

	if sig == nil {
		t.Fatal("exhausted repair with an uncorroborated agent floor claim must halt")
	}
	if !strings.Contains(sig.Evidence, "contradict") {
		t.Errorf("halt evidence does not name the classifier contradiction:\n%s", sig.Evidence)
	}
	if !strings.Contains(sig.Evidence, "exhausted") {
		t.Errorf("halt evidence does not say the repair budget was spent:\n%s", sig.Evidence)
	}
}
