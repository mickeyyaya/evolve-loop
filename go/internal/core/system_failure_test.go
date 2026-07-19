package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// ADR-0072 S3: the Go floor. A recorded-negative cycle whose on-disk artifacts
// are green is verdict-incoherence (the pipeline forged the verdict) → HALT.
// A recorded-negative with a RED artifact is a genuine task failure → nil.

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

func TestDetectVerdictIncoherence_ForgedVerdict_Halts(t *testing.T) {
	o := &Orchestrator{failurePolicy: policy.DefaultSystemFailurePolicy()}
	dir := t.TempDir()
	writeVerdicts(t, dir, "PASS", "PASS") // green artifacts
	cs := CycleState{CycleID: 1, WorkspacePath: dir}

	sig := o.detectVerdictIncoherence(cs, VerdictFAIL)
	if sig == nil {
		t.Fatal("recorded FAIL + green artifacts must produce a system-failure signal")
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

	// The "silent no-ship" (CycleOutcomeSkippedUnknown) is NOT hard-halted by
	// the deterministic floor — a benign no-op cycle can also produce it, so the
	// floor stays narrow (recorded FAIL/WARN only) and leaves the ambiguous skip
	// to the orchestrator's judgment layer (S4). No false-halt on a benign skip.
	if sig := o.detectVerdictIncoherence(cs, CycleOutcomeSkippedUnknown); sig != nil {
		t.Errorf("silent no-ship must NOT hard-halt (deferred to orchestrator), got %+v", sig)
	}
}

func TestDetectVerdictIncoherence_GenuineFail_NoHalt(t *testing.T) {
	o := &Orchestrator{failurePolicy: policy.DefaultSystemFailurePolicy()}
	dir := t.TempDir()
	writeVerdicts(t, dir, "FAIL", "PASS") // RED audit artifact = genuine failure
	cs := CycleState{CycleID: 3, WorkspacePath: dir}

	if sig := o.detectVerdictIncoherence(cs, VerdictFAIL); sig != nil {
		t.Errorf("genuine audit FAIL must NOT halt (never-stop task path), got %+v", sig)
	}
}

func TestWithFailurePolicy_InjectsResolvedPolicy(t *testing.T) {
	// Names WithFailurePolicy + the SystemFailureSignal alias (apicover).
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

	if sig := o.detectVerdictIncoherence(cs, VerdictPASS); sig != nil {
		t.Errorf("a PASS cycle is never a system failure, got %+v", sig)
	}
}

// TestDetectVerdictIncoherence_DiagnosedGateFail_NoHalt — the cycle-930/931/932
// false-HALT regression. The audit agent writes a PASS report + green ACS, but a
// runner-side CI-parity gate (the integration tier) legitimately downgrades the
// verdict to FAIL; the record chokepoint stamps the reasons into ORCHESTRATOR
// MEMORY (cs.AuditFailReasons). That FAIL is DIAGNOSED — a coherent task-level
// outcome (retro + continue), NOT a forged verdict — so the floor must NOT halt.
// Before this fix, detectVerdictIncoherence never populated SubstantiveError, so
// every diagnosed gate-downgrade with green artifacts halted the whole batch.
func TestDetectVerdictIncoherence_DiagnosedGateFail_NoHalt(t *testing.T) {
	o := &Orchestrator{failurePolicy: policy.DefaultSystemFailurePolicy()}
	dir := t.TempDir()
	writeVerdicts(t, dir, "PASS", "PASS") // green artifacts (the agent's own view)
	cs := CycleState{CycleID: 932, WorkspacePath: dir,
		AuditFailReasons: []string{"the integration tier (`go test -tags integration`) reported 12 offender(s)"}}

	if sig := o.detectVerdictIncoherence(cs, VerdictFAIL); sig != nil {
		t.Errorf("a diagnosed gate-downgrade FAIL must be a coherent task failure (no halt), got %+v", sig)
	}
}

// TestDetectVerdictIncoherence_WorkspaceReasonFileAlone_StillHalts — the trust
// boundary (go-review HIGH): the workspace is agent-writable, so a
// <phase>-fail-reason.json dropped there by ANY writer — a prompt-injected
// auditor, a later phase sharing the workspace — must NOT be able to talk the
// floor out of halting. Only the orchestrator's in-memory cs.AuditFailReasons
// (set at the verdict-record chokepoint) marks a FAIL as explained; the file is
// forensic output, never floor input.
func TestDetectVerdictIncoherence_WorkspaceReasonFileAlone_StillHalts(t *testing.T) {
	o := &Orchestrator{failurePolicy: policy.DefaultSystemFailurePolicy()}
	dir := t.TempDir()
	writeVerdicts(t, dir, "PASS", "PASS")
	// A perfectly VALID reason file, planted in the workspace — but no
	// orchestrator-memory record of a diagnosed downgrade.
	if err := os.WriteFile(filepath.Join(dir, "audit-fail-reason.json"),
		[]byte(`{"schema_version":1,"phase":"audit","reasons":["EGPS: red_count=1"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cs := CycleState{CycleID: 6, WorkspacePath: dir}

	if sig := o.detectVerdictIncoherence(cs, VerdictFAIL); sig == nil {
		t.Fatal("a workspace reason file ALONE must never suppress the forged-verdict halt (agent-writable territory)")
	}
}

// TestDetectVerdictIncoherence_WarningOnlyDiags_StillHalts — defensive: a
// record call whose diagnostics carry NO error severity explains nothing, so
// the forged-verdict floor keeps halting (warning-only diags never suppress).
func TestDetectVerdictIncoherence_WarningOnlyDiags_StillHalts(t *testing.T) {
	o := &Orchestrator{failurePolicy: policy.DefaultSystemFailurePolicy()}
	dir := t.TempDir()
	writeVerdicts(t, dir, "PASS", "PASS")
	cs := CycleState{CycleID: 5, WorkspacePath: dir}
	persistFloorFailReasons(&cs, PhaseAudit, []Diagnostic{
		{Severity: "warning", Message: "gofmt gate skipped (could not run)"},
	})

	if sig := o.detectVerdictIncoherence(cs, VerdictFAIL); sig == nil {
		t.Fatal("an UNEXPLAINED FAIL with green artifacts must still halt — warning-only reasons explain nothing")
	}
}

// TestRecordFloorVerdictFailure_PersistsAuditFailReason — the wiring: the shared
// floor-verdict recorder (live loop + resume path) must stamp the downgrade
// reasons into cs.AuditFailReasons (the coherence floor's authoritative source)
// AND write the forensic workspace file (the untruncated "why" for retros).
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

// TestPersistFloorFailReasons_ClobberAndReset — staleness (go-review
// MEDIUM-HIGH): audit can be re-dispatched within one cycle (ship-error
// recovery re-audit, debugger RERUN_PHASE). A superseding record with no
// error-severity diags must CLOBBER the prior explanation (memory + file), and
// the dispatch-time reset must clear both — a stale explanation from a
// superseded attempt must never mark a later, differently-caused FAIL as
// diagnosed.
func TestPersistFloorFailReasons_ClobberAndReset(t *testing.T) {
	dir := t.TempDir()
	cs := &CycleState{CycleID: 8, WorkspacePath: dir}

	persistFloorFailReasons(cs, PhaseAudit, []Diagnostic{{Severity: "error", Message: "EGPS: red_count=2"}})
	if len(cs.AuditFailReasons) != 1 || len(readFloorFailReasons(dir, PhaseAudit)) != 1 {
		t.Fatalf("setup: first record must set memory+file; got mem=%v file=%v", cs.AuditFailReasons, readFloorFailReasons(dir, PhaseAudit))
	}

	// Superseding record with warning-only diags → both carriers cleared.
	persistFloorFailReasons(cs, PhaseAudit, []Diagnostic{{Severity: "warning", Message: "gate skipped"}})
	if cs.AuditFailReasons != nil {
		t.Errorf("superseding warning-only record must clobber cs.AuditFailReasons, got %v", cs.AuditFailReasons)
	}
	if got := readFloorFailReasons(dir, PhaseAudit); got != nil {
		t.Errorf("superseding warning-only record must remove the forensic file, got %v", got)
	}

	// Re-record, then the dispatch-time reset clears both again.
	persistFloorFailReasons(cs, PhaseAudit, []Diagnostic{{Severity: "error", Message: "go vet reported 1 issue"}})
	resetFloorFailReason(cs, PhaseAudit)
	if cs.AuditFailReasons != nil || readFloorFailReasons(dir, PhaseAudit) != nil {
		t.Errorf("dispatch-time reset must clear memory+file; got mem=%v file=%v", cs.AuditFailReasons, readFloorFailReasons(dir, PhaseAudit))
	}
}
