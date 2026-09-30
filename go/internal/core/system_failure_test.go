package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// writeVerdicts writes a fully-valid audit-report.md (the required ## Verdict
// section + a PASS-vocabulary sentinel) plus the acs-verdict.json.
func writeVerdicts(t *testing.T, dir, audit, acs string) {
	t.Helper()
	if audit != "" {
		body := "## Verdict\n<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"" + audit + "\"} -->\n"
		if err := os.WriteFile(filepath.Join(dir, "audit-report.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if acs != "" {
		if err := os.WriteFile(filepath.Join(dir, "acs-verdict.json"), []byte(`{"verdict":"`+acs+`"}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// okStubVerifier is a controllable ContractVerifier double: VerifyDeliverable
// returns ok verbatim, distinct from correction_ladder_test.go's path-aware
// fakeVerifier — this one needs no filesystem fixture.
type okStubVerifier struct{ ok bool }

func (v okStubVerifier) VerifyDeliverable(_ context.Context, _ ReviewInput) (ContractVerification, error) {
	return ContractVerification{OK: v.ok}, nil
}

func TestDetectVerdictIncoherence_ForgedVerdict_Halts(t *testing.T) {
	o := &Orchestrator{failurePolicy: policy.DefaultSystemFailurePolicy()}
	dir := t.TempDir()
	writeVerdicts(t, dir, "PASS", "PASS")
	cs := CycleState{CycleID: 1, WorkspacePath: dir}

	sig, reconciled := o.detectVerdictIncoherence(context.Background(), cs, VerdictFAIL)
	if reconciled {
		t.Fatal("no verifier configured must NOT reconcile (cannot prove the deliverable is valid)")
	}
	if sig == nil {
		t.Fatal("recorded FAIL + green artifacts + unverifiable deliverable must produce a system-failure signal")
	}
	if !sig.Halt {
		t.Error("verdict-incoherence must be a floor HALT")
	}
	if sig.Category != "verdict-incoherence" {
		t.Errorf("category = %q, want verdict-incoherence", sig.Category)
	}
	if sig.Level != policy.LevelSystem {
		t.Errorf("level = %q, want system", sig.Level)
	}
}

func TestDetectVerdictIncoherence_SilentNoShip_DefersToOrchestrator(t *testing.T) {
	o := &Orchestrator{failurePolicy: policy.DefaultSystemFailurePolicy()}
	dir := t.TempDir()
	writeVerdicts(t, dir, "PASS", "PASS")
	cs := CycleState{CycleID: 2, WorkspacePath: dir}

	if sig, _ := o.detectVerdictIncoherence(context.Background(), cs, CycleOutcomeSkippedUnknown); sig != nil {
		t.Errorf("silent no-ship must NOT hard-halt (deferred to orchestrator), got %+v", sig)
	}
}

func TestDetectVerdictIncoherence_GenuineFail_NoHalt(t *testing.T) {
	o := &Orchestrator{failurePolicy: policy.DefaultSystemFailurePolicy()}
	dir := t.TempDir()
	writeVerdicts(t, dir, "FAIL", "PASS")
	cs := CycleState{CycleID: 3, WorkspacePath: dir}

	if sig, _ := o.detectVerdictIncoherence(context.Background(), cs, VerdictFAIL); sig != nil {
		t.Errorf("genuine audit FAIL must NOT halt (never-stop task path), got %+v", sig)
	}
}

func TestDetectVerdictIncoherence_ReconcileUsesFullVerify(t *testing.T) {
	newOrch := func(verifyOK bool) *Orchestrator {
		return &Orchestrator{
			failurePolicy:    policy.DefaultSystemFailurePolicy(),
			contractVerifier: okStubVerifier{ok: verifyOK},
		}
	}

	valid := t.TempDir()
	writeVerdicts(t, valid, "PASS", "PASS")
	sig, reconciled := newOrch(true).detectVerdictIncoherence(context.Background(), CycleState{CycleID: 1, WorkspacePath: valid}, VerdictFAIL)
	if sig != nil || !reconciled {
		t.Errorf("green artifacts + valid deliverable must reconcile (nil signal), got sig=%+v reconciled=%v", sig, reconciled)
	}

	forged := t.TempDir()
	writeVerdicts(t, forged, "PASS", "PASS")
	sig, reconciled = newOrch(false).detectVerdictIncoherence(context.Background(), CycleState{CycleID: 2, WorkspacePath: forged}, VerdictFAIL)
	if reconciled {
		t.Error("a PASS-sentinel-tagged report that does not fully verify must NOT reconcile (would launder forgery)")
	}
	if sig == nil || !sig.Halt {
		t.Errorf("unverified deliverable must still halt, got sig=%+v", sig)
	}
}

func TestWithFailurePolicy_InjectsResolvedPolicy(t *testing.T) {
	o := &Orchestrator{}
	WithFailurePolicy(policy.DefaultSystemFailurePolicy())(o)
	if !o.failurePolicy.IsFloor(policy.CategoryVerdictIncoherence) {
		t.Error("WithFailurePolicy did not inject the resolved policy (floor missing)")
	}
	sig := SystemFailureSignal{Category: policy.CategoryVerdictIncoherence, Level: policy.LevelSystem, Halt: true}
	if !sig.Halt {
		t.Error("SystemFailureSignal.Halt not set")
	}
}

func TestDetectVerdictIncoherence_PassVerdict_NoHalt(t *testing.T) {
	o := &Orchestrator{failurePolicy: policy.DefaultSystemFailurePolicy()}
	dir := t.TempDir()
	writeVerdicts(t, dir, "PASS", "PASS")
	cs := CycleState{CycleID: 4, WorkspacePath: dir}

	if sig, _ := o.detectVerdictIncoherence(context.Background(), cs, VerdictPASS); sig != nil {
		t.Errorf("a PASS cycle is never a system failure, got %+v", sig)
	}
}

func TestDetectVerdictIncoherence_DiagnosedGateFail_NoHalt(t *testing.T) {
	o := &Orchestrator{failurePolicy: policy.DefaultSystemFailurePolicy()}
	dir := t.TempDir()
	writeVerdicts(t, dir, "PASS", "PASS")
	cs := CycleState{CycleID: 932, WorkspacePath: dir,
		AuditFailReasons: []string{"the integration tier (`go test -tags integration`) reported 12 offender(s)"}}

	if sig, _ := o.detectVerdictIncoherence(context.Background(), cs, VerdictFAIL); sig != nil {
		t.Errorf("a diagnosed gate-downgrade FAIL must be a coherent task failure (no halt), got %+v", sig)
	}
}

func TestDetectVerdictIncoherence_WorkspaceReasonFileAlone_StillHalts(t *testing.T) {
	o := &Orchestrator{failurePolicy: policy.DefaultSystemFailurePolicy(), contractVerifier: okStubVerifier{ok: false}}
	dir := t.TempDir()
	writeVerdicts(t, dir, "PASS", "PASS")
	if err := os.WriteFile(filepath.Join(dir, "audit-fail-reason.json"),
		[]byte(`{"schema_version":1,"phase":"audit","reasons":["EGPS: red_count=1"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cs := CycleState{CycleID: 6, WorkspacePath: dir}

	if sig, _ := o.detectVerdictIncoherence(context.Background(), cs, VerdictFAIL); sig == nil {
		t.Fatal("a workspace reason file ALONE must never suppress the forged-verdict halt (agent-writable territory)")
	}
}

func TestDetectVerdictIncoherence_WarningOnlyDiags_StillHalts(t *testing.T) {
	o := &Orchestrator{failurePolicy: policy.DefaultSystemFailurePolicy(), contractVerifier: okStubVerifier{ok: false}}
	dir := t.TempDir()
	writeVerdicts(t, dir, "PASS", "PASS")
	cs := CycleState{CycleID: 5, WorkspacePath: dir}
	persistFloorFailReasons(&cs, PhaseAudit, []Diagnostic{
		{Severity: "warning", Message: "gofmt gate skipped (could not run)"},
	})

	if sig, _ := o.detectVerdictIncoherence(context.Background(), cs, VerdictFAIL); sig == nil {
		t.Fatal("an UNEXPLAINED FAIL with green artifacts must still halt — warning-only reasons explain nothing")
	}
}

func TestDetectVerdictIncoherence_ShipPhaseExplainedFail_NoHalt(t *testing.T) {
	o := &Orchestrator{failurePolicy: policy.DefaultSystemFailurePolicy()}
	dir := t.TempDir()
	writeVerdicts(t, dir, "PASS", "PASS")
	cs := CycleState{CycleID: 1329, WorkspacePath: dir,
		ShipFailReasons: []string{
			"repo-contract scanner pack RED in the lane worktree (exit status 1) — pushing would red main; " +
				"fix the violation in-lane (the four suites: phasespec, profiles, phasecoherence, routingtest)",
		}}

	if sig, _ := o.detectVerdictIncoherence(context.Background(), cs, VerdictFAIL); sig != nil {
		t.Errorf("a diagnosed ship-phase failure (green audit + green ACS, real ship-gate rejection) must be "+
			"a coherent task failure (no halt), got %+v", sig)
	}
}

func TestDetectVerdictIncoherence_AuditPhaseBehaviorUnchangedByShipField(t *testing.T) {
	o := &Orchestrator{failurePolicy: policy.DefaultSystemFailurePolicy(), contractVerifier: okStubVerifier{ok: false}}
	dir := t.TempDir()
	writeVerdicts(t, dir, "PASS", "PASS")
	cs := CycleState{CycleID: 1330, WorkspacePath: dir}

	if sig, _ := o.detectVerdictIncoherence(context.Background(), cs, VerdictFAIL); sig == nil {
		t.Fatal("with neither AuditFailReasons nor ShipFailReasons populated, an unexplained FAIL with green " +
			"artifacts must still halt — the new carrier must not weaken the existing forgery floor")
	}
}

func TestRecordFloorVerdictFailure_PersistsAuditFailReason(t *testing.T) {
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, nil)
	dir := t.TempDir()
	cs := &CycleState{CycleID: 7, WorkspacePath: dir}
	state := &State{}
	diags := []Diagnostic{
		{Severity: "error", Message: "the integration tier reported 3 offender(s)"},
		{Severity: "warning", Message: "apicover gate skipped"},
	}

	o.recordFloorVerdictFailure(context.Background(), CycleRequest{}, 7, PhaseAudit, state, cs, diags)

	if len(cs.AuditFailReasons) != 1 || !strings.Contains(cs.AuditFailReasons[0], "integration tier") {
		t.Fatalf("cs.AuditFailReasons = %v, want exactly the 1 error-severity diagnostic (the floor's authoritative source)", cs.AuditFailReasons)
	}
	reasons := readFloorFailReasons(dir, PhaseAudit)
	if len(reasons) != 1 || !strings.Contains(reasons[0], "integration tier") {
		t.Errorf("forensic file reasons = %v, want the same 1 diagnostic persisted", reasons)
	}
}

func TestPersistFloorFailReasons_ClobberAndReset(t *testing.T) {
	dir := t.TempDir()
	cs := &CycleState{CycleID: 8, WorkspacePath: dir}

	persistFloorFailReasons(cs, PhaseAudit, []Diagnostic{{Severity: "error", Message: "EGPS: red_count=2"}})
	if len(cs.AuditFailReasons) != 1 || len(readFloorFailReasons(dir, PhaseAudit)) != 1 {
		t.Fatalf("setup: first record must set memory+file; got mem=%v file=%v", cs.AuditFailReasons, readFloorFailReasons(dir, PhaseAudit))
	}

	persistFloorFailReasons(cs, PhaseAudit, []Diagnostic{{Severity: "warning", Message: "gate skipped"}})
	if cs.AuditFailReasons != nil {
		t.Errorf("superseding warning-only record must clobber cs.AuditFailReasons, got %v", cs.AuditFailReasons)
	}
	if got := readFloorFailReasons(dir, PhaseAudit); got != nil {
		t.Errorf("superseding warning-only record must remove the forensic file, got %v", got)
	}

	persistFloorFailReasons(cs, PhaseAudit, []Diagnostic{{Severity: "error", Message: "go vet reported 1 issue"}})
	resetFloorFailReason(cs, PhaseAudit)
	if cs.AuditFailReasons != nil || readFloorFailReasons(dir, PhaseAudit) != nil {
		t.Errorf("dispatch-time reset must clear memory+file; got mem=%v file=%v", cs.AuditFailReasons, readFloorFailReasons(dir, PhaseAudit))
	}
}
