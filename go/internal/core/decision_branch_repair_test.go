package core

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

const repairFixtureFingerprint = "audit|verdict-fail|deadbeef"

// writeDisposition writes only the fields the repair rule reads; the gate's
// full schema is exercised by disposition_gate_test.go and is deliberately
// not duplicated here.
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

func writeFailureDigest(t *testing.T, dir, fingerprint string, cycle int) {
	t.Helper()
	body := fmt.Sprintf(`{"cycle":%d,"fingerprint":%q,"pre_class":"verdict-fail","recurrence":0}`, cycle, fingerprint)
	if err := os.WriteFile(filepath.Join(dir, "failure-digest.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write failure-digest: %v", err)
	}
}

// agentFloorClaim is the exact failure-decision.json body
// TestDecideAfterRetroFloor_Cycle1001JudgmentHalt uses, shared here so the
// only variable between "halts" and "repairs" is the disposition.
const agentFloorClaim = `{"category":"infra-systemic","level":"system","evidence":"prose-declared SYSTEM-class","action":"halt-and-diagnose","fix_type":"pipeline-repair"}`

func TestDecideAfterRetro_RepairsWhenAgentFloorIsContradicted(t *testing.T) {
	o := floorOrchestrator(fixedNextStrategy{next: "tdd"})
	dir := t.TempDir()
	// code-audit-fail keeps the deterministic dossier candidate empty.
	writeAuditWithFailure(t, dir, "FAIL", "code-audit-fail", "H1 staged out-of-lane phase stub")
	writeDecision(t, dir, agentFloorClaim)
	writeDisposition(t, dir, "legit-rejection")
	writeFailureDigest(t, dir, repairFixtureFingerprint, 1573)
	cs := CycleState{CycleID: 1573, WorkspacePath: dir}

	_, _, _, sig := o.decideAfterRetro(cs, VerdictFAIL, nil)

	if sig != nil {
		t.Fatalf("a contradicted agent floor claim must NOT halt; sig=%+v", sig)
	}
}

// Byte-identical to the case above, minus disposition.json.
func TestDecideAfterRetro_HaltsWhenNoDispositionContradictsTheAgent(t *testing.T) {
	o := floorOrchestrator(fixedNextStrategy{next: "tdd"})
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

func TestDecideAfterRetro_DeterministicFloorBeatsLegitRejection(t *testing.T) {
	o := floorOrchestrator(fixedNextStrategy{next: "tdd"})
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

func TestDecideAfterRetro_RefusesRepairOnFabricatedFailureIdentity(t *testing.T) {
	o := floorOrchestrator(fixedNextStrategy{next: "tdd"})
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

func TestDecideAfterRetro_RefusesRepairOnForeignCycleDisposition(t *testing.T) {
	o := floorOrchestrator(fixedNextStrategy{next: "tdd"})
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

func TestDecideAfterRetro_RefusesRepairWhenIdentityIsUnverifiable(t *testing.T) {
	o := floorOrchestrator(fixedNextStrategy{next: "tdd"})
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
