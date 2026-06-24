package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// Reconcile-on-timeout (self-healing): when the bridge reports ErrArtifactTimeout
// but the agent's contracted deliverable is on disk and WELL-FORMED, the runner
// must trust the deliverable's verdict (via Classify) instead of synthesizing
// FAIL. This is the deeper fix behind the cycle-254/255 false-FAILs: a complete
// PASS audit report was discarded because the bridge gave up on the wait window.
// The verifyFn seam lets these tests drive the well-formedness branch directly
// without coupling to per-phase contract sections; the real deliverable.Verify +
// EGPS gate is exercised end-to-end in the audit package.

func verifyReturns(res deliverable.Result, err error) func(string, phasecontract.Roots) (deliverable.Result, error) {
	return func(string, phasecontract.Roots) (deliverable.Result, error) { return res, err }
}

// TestRun_Timeout_WellFormedPASS_ReconcilesToPass — the core fix: timeout +
// a well-formed deliverable whose Classify verdict is PASS → reconcile to PASS
// with a nil error, Reconciled=true, and Classify actually ran (proves the
// fall-through, not a bare sentinel read).
func TestRun_Timeout_WellFormedPASS_ReconcilesToPass(t *testing.T) {
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictPASS}
	fb := &fakeBridge{err: artifactTimeoutErr(), writeArtifact: "# audit\n<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"PASS\"} -->\n"}
	r := New(Options{
		Hooks:    hooks,
		Bridge:   fb,
		Prompts:  fakePromptsFS("evolve-auditor", "x"),
		VerifyFn: verifyReturns(deliverable.Result{OK: true}, nil),
	})

	resp, err := r.Run(context.Background(), core.PhaseRequest{Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("well-formed PASS deliverable on timeout must reconcile to a NIL error; got %v", err)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("verdict=%q, want PASS (reconciled)", resp.Verdict)
	}
	if !resp.Reconciled {
		t.Error("resp.Reconciled must be true on a reconciled timeout")
	}
	if hooks.classifyCalls != 1 {
		t.Errorf("Classify must run on the reconcile fall-through; got %d calls", hooks.classifyCalls)
	}
	if !hasWarningDiag(resp.Diagnostics) {
		t.Errorf("expected a warning diagnostic recording the reconciliation, got %+v", resp.Diagnostics)
	}
}

// TestRun_Timeout_SentinelFAIL_StaysFail — reconciliation only UPGRADES toward
// the agent's real verdict; it never downgrades. A well-formed deliverable whose
// Classify verdict is FAIL stays FAIL. Because the deliverable is COMPLETE, the
// phase is treated as a normal completed FAIL (nil error → routes as a real
// audit-fail, not an infra-timeout retry).
func TestRun_Timeout_SentinelFAIL_StaysFail(t *testing.T) {
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictFAIL}
	fb := &fakeBridge{err: artifactTimeoutErr(), writeArtifact: "# audit\nFAIL\n"}
	r := New(Options{
		Hooks:    hooks,
		Bridge:   fb,
		Prompts:  fakePromptsFS("evolve-auditor", "x"),
		VerifyFn: verifyReturns(deliverable.Result{OK: true}, nil),
	})

	resp, err := r.Run(context.Background(), core.PhaseRequest{Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("a reconciled FAIL is a COMPLETED phase — it must return nil error so it routes as a real audit-fail, not an infra-timeout retry; got %v", err)
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("verdict=%q, want FAIL (honored, not downgraded)", resp.Verdict)
	}
	if !resp.Reconciled {
		t.Error("resp.Reconciled must be true — reconcile engaged, then honored the agent's FAIL")
	}
	if hooks.classifyCalls != 1 {
		t.Errorf("Classify must run to read the agent's real verdict; got %d calls", hooks.classifyCalls)
	}
}

// TestRun_Timeout_NotWellFormed_StaysFail — a hung agent that wrote nothing /
// a partial / a malformed deliverable → Verify !OK → hard-FAIL, Classify NOT
// reached. This is the guard that reconciliation can't ship a hung agent.
func TestRun_Timeout_NotWellFormed_StaysFail(t *testing.T) {
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictPASS}
	fb := &fakeBridge{err: artifactTimeoutErr()} // no artifact written
	notOK := deliverable.Result{OK: false, Violations: []deliverable.Violation{{Code: deliverable.CodeMissingArtifact, Message: "deliverable not found"}}}
	r := New(Options{
		Hooks:    hooks,
		Bridge:   fb,
		Prompts:  fakePromptsFS("evolve-auditor", "x"),
		VerifyFn: verifyReturns(notOK, nil),
	})

	resp, err := r.Run(context.Background(), core.PhaseRequest{Workspace: t.TempDir()})
	if err == nil {
		t.Fatal("a malformed/absent deliverable on timeout must hard-fail")
	}
	if !errors.Is(err, core.ErrArtifactTimeout) {
		t.Errorf("error should wrap ErrArtifactTimeout; got %v", err)
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("verdict=%q, want FAIL", resp.Verdict)
	}
	if hooks.classifyCalls != 0 {
		t.Errorf("Classify must NOT run when the deliverable is not well-formed; got %d calls", hooks.classifyCalls)
	}
	if resp.Reconciled {
		t.Error("resp.Reconciled must be false when nothing was reconciled")
	}
}

// TestRun_OptionalPhase_Timeout_WellFormedPASS_ReconcilesToPass — optional phases
// reconcile UP past the old unconditional WARN when the deliverable is clean.
func TestRun_OptionalPhase_Timeout_WellFormedPASS_ReconcilesToPass(t *testing.T) {
	hooks := &fakeHooks{phase: "build-planner", agent: "evolve-build-planner", model: "opus", prompt: "x", verdict: core.VerdictPASS}
	fb := &fakeBridge{err: artifactTimeoutErr(), writeArtifact: "# plan\n"}
	r := New(Options{
		Hooks:    hooks,
		Bridge:   fb,
		Prompts:  fakePromptsFS("evolve-build-planner", "x"),
		Optional: true,
		VerifyFn: verifyReturns(deliverable.Result{OK: true}, nil),
	})

	resp, err := r.Run(context.Background(), core.PhaseRequest{Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("optional+well-formed-PASS must reconcile to nil error; got %v", err)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("verdict=%q, want PASS (reconciled up from WARN)", resp.Verdict)
	}
	if !resp.Reconciled {
		t.Error("resp.Reconciled must be true")
	}
}

// TestRun_NonTimeoutError_Timeout_StaysFail_Unchanged — guards against
// over-broadening: a NON-timeout bridge error never consults the deliverable;
// it hard-fails exactly as before (Classify not called, not reconciled).
func TestRun_NonTimeoutError_StaysFail_Unchanged(t *testing.T) {
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictPASS}
	fb := &fakeBridge{err: errors.New("bridge: launch exit=2"), writeArtifact: "# audit\nPASS\n"} // safety-gate, not a timeout
	verifyCalled := false
	r := New(Options{
		Hooks:   hooks,
		Bridge:  fb,
		Prompts: fakePromptsFS("evolve-auditor", "x"),
		VerifyFn: func(string, phasecontract.Roots) (deliverable.Result, error) {
			verifyCalled = true
			return deliverable.Result{OK: true}, nil
		},
	})

	resp, err := r.Run(context.Background(), core.PhaseRequest{Workspace: t.TempDir()})
	if err == nil {
		t.Fatal("a non-timeout bridge error must still hard-fail")
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("verdict=%q, want FAIL", resp.Verdict)
	}
	if verifyCalled {
		t.Error("deliverable must NOT be consulted for a non-timeout error")
	}
	if hooks.classifyCalls != 0 {
		t.Errorf("Classify must not run; got %d calls", hooks.classifyCalls)
	}
	if resp.Reconciled {
		t.Error("resp.Reconciled must be false")
	}
}

func hasWarningDiag(diags []core.Diagnostic) bool {
	for _, d := range diags {
		if d.Severity == "warning" {
			return true
		}
	}
	return false
}

// TestNew_DefaultVerifyFnIsCatalogAware — the reconcile default must resolve
// contracts under the SAME policy as the host gate and the agent self-check
// (merged catalog), not BuiltinResolver-only: a user/minted phase (e.g. an
// advisor-inserted mutation-gate) whose artifact survived a bridge timeout
// was unresolvable and synthesized FAIL.
func TestNew_DefaultVerifyFnIsCatalogAware(t *testing.T) {
	root := t.TempDir()
	regDir := filepath.Join(root, "docs", "architecture")
	if err := os.MkdirAll(regDir, 0o755); err != nil {
		t.Fatal(err)
	}
	registry := `{"phases":[]}`
	if err := os.WriteFile(filepath.Join(regDir, "phase-registry.json"), []byte(registry), 0o644); err != nil {
		t.Fatal(err)
	}
	userDir := filepath.Join(root, ".evolve", "phases", "widget-scan")
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		t.Fatal(err)
	}
	spec := `{"name":"widget-scan","archetype":"evaluate","agent":"evolve-widget-scan",
		"outputs":{"files":[".evolve/runs/cycle-{cycle}/widget-scan-report.md"]}}`
	if err := os.WriteFile(filepath.Join(userDir, "phase.json"), []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}

	hooks := &fakeHooks{phase: "widget-scan", agent: "evolve-widget-scan", model: "sonnet", prompt: "x", verdict: core.VerdictPASS}
	b := New(Options{Hooks: hooks, Bridge: &fakeBridge{}, Prompts: fakePromptsFS("evolve-widget-scan", "x")})

	roots := phasecontract.Roots{
		Workspace: filepath.Join(root, ".evolve", "runs", "cycle-7"),
		EvolveDir: filepath.Join(root, ".evolve"),
	}
	if _, err := b.verifyFn("widget-scan", roots); err != nil {
		t.Fatalf("default verifyFn must resolve a user phase via the merged catalog "+
			"(reconcile parity with the gate + self-check); got: %v", err)
	}
}
