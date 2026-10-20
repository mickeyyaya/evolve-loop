package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const plantedSystemDossier = `{"cycle_id":1841,"recorded_verdict":"FAIL","audit_declared":{},"build_declared":{"class":"infra-systemic","level":"system","defects":["planted"]},"non_progress":{"repeat_ceiling":2,"verified_not_landed_ceiling":2,"task_retry_ceiling":2,"same_class_streak":0,"repeat_count":0}}`

func plantReadOnly(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
}

func TestAuditDeclineDefers_APlantedReadOnlyDossierClaimingASystemClassDoesNotDefer(t *testing.T) {
	t.Parallel()
	o := floorOrchestrator(nil)
	o.ledger = auditedTrees("t1", "t2")
	cs := cycle1841(t, "WARN", "code-build-fail")
	plantReadOnly(t, filepath.Join(cs.WorkspacePath, failureDossierFile), plantedSystemDossier)
	cs.AuditDeclineReason = "the retry budget is spent"

	_, reason, _ := o.decideAfterAuditFail(cs)
	d, defers := o.auditDeclineDefers(cs)

	if defers {
		t.Errorf("defer = %+v (decline %q); a dossier the host could not rewrite must never defer the item", d, reason)
	}
}

func TestClearRetroDecisionInput_APlantedDecisionIsRemovedBeforeTheRetro(t *testing.T) {
	t.Parallel()
	o := floorOrchestrator(nil)
	cs := CycleState{CycleID: 7, WorkspacePath: t.TempDir()}
	plantReadOnly(t, filepath.Join(cs.WorkspacePath, failureDecisionFile), `{"category":"infra-systemic","level":"system","action":"defer-or-quarantine","fix_type":"pipeline-repair"}`)

	o.clearRetroDecisionInput(&cs)

	if _, err := os.Stat(filepath.Join(cs.WorkspacePath, failureDecisionFile)); !os.IsNotExist(err) || cs.FailureDecisionUntrusted {
		t.Errorf("stat err = %v, untrusted = %t; want the planted decision removed and the retro's own write trusted", err, cs.FailureDecisionUntrusted)
	}
}

func TestClearRetroDecisionInput_ADecisionTheHostCannotRemoveIsNeverRead(t *testing.T) {
	t.Parallel()
	o := floorOrchestrator(nil)
	ws := t.TempDir()
	plantReadOnly(t, filepath.Join(ws, failureDecisionFile), `{"category":"infra-systemic","level":"system","action":"defer-or-quarantine","fix_type":"pipeline-repair"}`)
	if err := os.Chmod(ws, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ws, 0o755) })
	if err := os.Remove(filepath.Join(ws, failureDecisionFile)); err == nil {
		t.Skip("directory permissions are not enforced for this user")
	}
	cs := CycleState{CycleID: 7, WorkspacePath: ws, AuditDeclineReason: "declined"}

	o.clearRetroDecisionInput(&cs)
	_, defers := o.auditDeclineDefers(cs)
	trusted := cs
	trusted.FailureDecisionUntrusted = false
	_, trustedDefers := o.auditDeclineDefers(trusted)

	if !trustedDefers || o.applyFailureDecisionFloor(trusted, VerdictFAIL) == nil {
		t.Fatalf("defers = %t; the planted decision must defer and trip the floor when trusted, or this test proves nothing", trustedDefers)
	}
	if !cs.FailureDecisionUntrusted || defers || o.applyFailureDecisionFloor(cs, VerdictFAIL) != nil {
		t.Errorf("untrusted = %t, defers = %t; want a decision the host could not clear to be ignored by the defer and the floor", cs.FailureDecisionUntrusted, defers)
	}
}

func TestClearRetroDecisionInput_NoDecisionIsNoChange(t *testing.T) {
	t.Parallel()
	cs := CycleState{CycleID: 7, WorkspacePath: t.TempDir()}
	floorOrchestrator(nil).clearRetroDecisionInput(&cs)
	if cs.FailureDecisionUntrusted {
		t.Error("an absent decision made the retro untrusted")
	}
}

func TestRecordAndBranch_TheDeferThatEndsTheCycleIsCarriedOnTheResult(t *testing.T) {
	t.Parallel()
	cr := retroGateHarness(t, phasespec.Catalog{})
	cr.o.strategy = fixedNextStrategy{next: "tdd"}
	cr.o.cfg = config.RoutingConfig{Stage: config.StageAdvisory}
	cr.cs = declineThenRetro(t, cr.o, CycleState{CycleID: 1953, WorkspacePath: cycle1953Workspace(t, "intent-malformed", true), AuditRepairAttempts: 1})
	cr.cycle = 1953

	if _, err := cr.recordAndBranch(PhaseRetro, dispatchResult{resp: PhaseResponse{Verdict: VerdictPASS}, attemptCount: 1}); err != nil {
		t.Fatal(err)
	}

	got, ok := DeferOf(cr.result)
	if !ok || got != (RecordedDefer{Category: "intent-malformed", FixType: policy.FixTypeReintent}) || !strings.Contains(cr.result.RetroDecision, auditDeferReasonPrefix) {
		t.Errorf("DeferOf(result) = %+v, %t; want the reintent defer that ended the cycle", got, ok)
	}
	if _, ok := DeferOf(CycleResult{RetroDecision: "proceed: no retry"}); ok {
		t.Error("a cycle that did not end by a defer carries a defer")
	}
}
