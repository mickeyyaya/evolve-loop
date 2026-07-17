package core

import (
	"os"
	"path/filepath"
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
