package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

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
		SleepFn:  func(time.Duration) {}, // skip the real settle-retry delay on the miss path
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

// noisyStdoutBridge simulates a non-timeout completion (err=nil) where the
// agent wrote a well-formed deliverable to disk but the captured stdout
// scrollback is noisy — e.g. it contains the Deliverable Contract's own
// prompt-echoed PASS/FAIL example sentinel lines. This is the cycle-603
// failure mode: NOT a timeout, so the existing reconcile fallback (gated on
// ErrArtifactTimeout, runner.go:585) never engages, and classification falls
// straight through to raw bres.Stdout (runner.go:655-662).
type noisyStdoutBridge struct {
	fileContent string
	stdout      string
}

func (b *noisyStdoutBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	if req.ArtifactPath != "" {
		_ = os.MkdirAll(filepath.Dir(req.ArtifactPath), 0o755)
		_ = os.WriteFile(req.ArtifactPath, []byte(b.fileContent), 0o644)
	}
	return core.BridgeResponse{Stdout: b.stdout}, nil
}

func (b *noisyStdoutBridge) Probe(_ context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

// TestRun_NonTimeout_WellFormedDeliverable_PrefersFileOverNoisyStdout —
// cycle-603: a non-timeout completion whose captured stdout contains BOTH a
// PASS-example and a FAIL-example contract-style sentinel line (the
// Deliverable Contract's own printed examples, not the agent's real verdict)
// must not classify off that noise. When the on-disk deliverable exists and
// verifies well-formed (OK), the runner must prefer the file — generalizing
// the already-tested timeout-reconcile pattern to every completion path, not
// just ErrArtifactTimeout.
func TestRun_NonTimeout_WellFormedDeliverable_PrefersFileOverNoisyStdout(t *testing.T) {
	genuine := "# audit\n<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"PASS\"} -->\n"
	noisyStdout := "Deliverable Contract example (PASS):\n" +
		"<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"PASS\"} -->\n" +
		"Deliverable Contract example (FAIL):\n" +
		"<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"FAIL\"} -->\n" +
		"(these are prompt-echoed examples, not the agent's real report)\n"
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictPASS}
	nb := &noisyStdoutBridge{fileContent: genuine, stdout: noisyStdout}
	r := New(Options{
		Hooks:    hooks,
		Bridge:   nb,
		Prompts:  fakePromptsFS("evolve-auditor", "x"),
		VerifyFn: verifyReturns(deliverable.Result{OK: true}, nil),
	})

	resp, err := r.Run(context.Background(), core.PhaseRequest{Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if hooks.gotArtifact != genuine {
		t.Errorf("Classify received noisy stdout instead of the well-formed deliverable file;\n got %q\nwant %q", hooks.gotArtifact, genuine)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("verdict=%q, want PASS", resp.Verdict)
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

// TestRun_Timeout_DeliverableSettlesOnRetry_ReconcilesToPass — cycles 824/825
// (width-2 storm): a next-phase (retrospective) context-cancel tore down the
// audit bridge session and was LAUNDERED into ErrArtifactTimeout at the exact
// instant the auditor's PASS deliverable was still SETTLING to disk. A
// single-shot verify at that instant reads a half-written file, so the mandatory
// audit phase hard-FAILs and a genuinely-PASS audited cycle (build PASS, tdd
// RED->green, adversarial PASS, audit PASS 0.95) is discarded and requeued from
// scratch. The reconcile must re-verify across a bounded settle window so the
// settled deliverable is caught and the cycle reconciles to the agent's real PASS
// — honoring the reconcile block's own documented intent (trust a deliverable
// written just as the bridge gave up on the wait window).
func TestRun_Timeout_DeliverableSettlesOnRetry_ReconcilesToPass(t *testing.T) {
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictPASS}
	fb := &fakeBridge{err: artifactTimeoutErr(), writeArtifact: "# audit\n<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"PASS\"} -->\n"}
	// The deliverable is still settling: the first two verifies miss (file
	// mid-write), the third — within the settle window — catches the well-formed
	// PASS deliverable.
	calls := 0
	settling := func(string, phasecontract.Roots) (deliverable.Result, error) {
		calls++
		if calls < 3 {
			return deliverable.Result{OK: false}, nil
		}
		return deliverable.Result{OK: true}, nil
	}
	r := New(Options{
		Hooks:    hooks,
		Bridge:   fb,
		Prompts:  fakePromptsFS("evolve-auditor", "x"),
		VerifyFn: settling,
		SleepFn:  func(time.Duration) {}, // deterministic: no real settle delay in tests
	})

	resp, err := r.Run(context.Background(), core.PhaseRequest{Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("a deliverable that settles within the retry window must reconcile to a NIL error; got %v", err)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("verdict=%q, want PASS (reconciled after settle-retry)", resp.Verdict)
	}
	if !resp.Reconciled {
		t.Error("resp.Reconciled must be true once the settling deliverable is caught")
	}
	if calls < 3 {
		t.Errorf("settle-retry must re-verify a settling deliverable; got only %d verify calls", calls)
	}
}

// TestRun_Timeout_DeliverableNeverSettles_StillFailsBounded — the settle-retry is
// BOUNDED and never manufactures a PASS. A genuinely absent / never-settling
// deliverable (a hung agent, not a settle race) still hard-FAILs after a fixed
// number of re-verifies: the loop must not spin, and reconciliation can only
// UPGRADE a timeout toward a real deliverable — never invent one.
func TestRun_Timeout_DeliverableNeverSettles_StillFailsBounded(t *testing.T) {
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictPASS}
	fb := &fakeBridge{err: artifactTimeoutErr()}
	calls := 0
	neverOK := func(string, phasecontract.Roots) (deliverable.Result, error) {
		calls++
		return deliverable.Result{OK: false}, nil
	}
	r := New(Options{
		Hooks:    hooks,
		Bridge:   fb,
		Prompts:  fakePromptsFS("evolve-auditor", "x"),
		VerifyFn: neverOK,
		SleepFn:  func(time.Duration) {},
	})

	resp, err := r.Run(context.Background(), core.PhaseRequest{Workspace: t.TempDir()})
	if err == nil {
		t.Fatal("a never-settling deliverable on a mandatory phase must still hard-FAIL")
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("verdict=%q, want FAIL (real timeout, not a settle race)", resp.Verdict)
	}
	if want := 1 + reconcileSettleRetries; calls != want {
		t.Errorf("settle-retry must be bounded: got %d verify calls, want %d (initial + %d retries)", calls, want, reconcileSettleRetries)
	}
}
