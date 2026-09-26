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

func verifyReturns(res deliverable.Result, err error) func(string, phasecontract.Roots) (deliverable.Result, error) {
	return func(phase string, roots phasecontract.Roots) (deliverable.Result, error) {
		if err != nil {
			return res, err
		}
		return verifiedFrom(res, phase, roots), nil
	}
}

// verifiedFrom stamps the judged path and bytes as deliverable.Verify does, because the runner classifies those bytes.
// Without a file the Result stays path-less, as a NoArtifact contract presents, so the pane stays the verdict source.
func verifiedFrom(res deliverable.Result, phase string, roots phasecontract.Roots) deliverable.Result {
	path := filepath.Join(roots.Workspace, phase+"-report.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return res
	}
	res.ArtifactPath = path
	res.Content = string(data)
	return res
}

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

// noisyStdoutBridge exits cleanly with stdout full of the contract's echoed PASS and FAIL example sentinels.
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

func TestRun_Timeout_DeliverableSettlesOnRetry_ReconcilesToPass(t *testing.T) {
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictPASS}
	fb := &fakeBridge{err: artifactTimeoutErr(), writeArtifact: "# audit\n<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"PASS\"} -->\n"}
	// The first two verifies miss a file still being written; the third, inside the settle window, sees the PASS.
	calls := 0
	settling := func(phase string, roots phasecontract.Roots) (deliverable.Result, error) {
		calls++
		if calls < 3 {
			return verifiedFrom(deliverable.Result{OK: false}, phase, roots), nil
		}
		return verifiedFrom(deliverable.Result{OK: true}, phase, roots), nil
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

func TestRun_Timeout_DeliverableNeverSettles_StillFailsBounded(t *testing.T) {
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictPASS}
	fb := &fakeBridge{err: artifactTimeoutErr()}
	calls := 0
	neverOK := func(phase string, roots phasecontract.Roots) (deliverable.Result, error) {
		calls++
		return verifiedFrom(deliverable.Result{OK: false}, phase, roots), nil
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
