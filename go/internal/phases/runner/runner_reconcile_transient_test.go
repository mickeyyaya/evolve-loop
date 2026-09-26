package runner

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// transientBridgeErr wraps core.ErrTransientBridgeFailure as bridge.Engine.Launch does, so errors.Is sees the real shape.
func transientBridgeErr(code int) error {
	return fmt.Errorf("bridge: launch exit=%d: %w", code, core.ErrTransientBridgeFailure)
}

func TestRun_TransientError_WellFormedPASS_ReconcilesToPass(t *testing.T) {
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictPASS}
	fb := &fakeBridge{err: transientBridgeErr(85), writeArtifact: "# audit\n<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"PASS\"} -->\n"}
	r := New(Options{
		Hooks:    hooks,
		Bridge:   fb,
		Prompts:  fakePromptsFS("evolve-auditor", "x"),
		VerifyFn: verifyReturns(deliverable.Result{OK: true}, nil),
	})

	resp, err := r.Run(context.Background(), core.PhaseRequest{Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("a well-formed PASS deliverable on a transient (quota) teardown must reconcile to a NIL error; got %v", err)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("verdict=%q, want PASS (reconciled)", resp.Verdict)
	}
	if !resp.Reconciled {
		t.Error("resp.Reconciled must be true on a reconciled transient teardown")
	}
	if hooks.classifyCalls != 1 {
		t.Errorf("Classify must run on the reconcile fall-through; got %d calls", hooks.classifyCalls)
	}
	if !hasWarningDiag(resp.Diagnostics) {
		t.Errorf("expected a warning diagnostic recording the reconciliation, got %+v", resp.Diagnostics)
	}
}

func TestRun_TransientError_SentinelFAIL_StaysFail(t *testing.T) {
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictFAIL}
	fb := &fakeBridge{err: transientBridgeErr(85), writeArtifact: "# audit\nFAIL\n"}
	r := New(Options{
		Hooks:    hooks,
		Bridge:   fb,
		Prompts:  fakePromptsFS("evolve-auditor", "x"),
		VerifyFn: verifyReturns(deliverable.Result{OK: true}, nil),
	})

	resp, err := r.Run(context.Background(), core.PhaseRequest{Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("a reconciled FAIL is a COMPLETED phase — it must return nil error; got %v", err)
	}
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("verdict=%q, want FAIL (honored, not downgraded)", resp.Verdict)
	}
	if !resp.Reconciled {
		t.Error("resp.Reconciled must be true — reconcile engaged, then honored the agent's FAIL")
	}
}

func TestRun_TransientError_NotWellFormed_Mandatory_StaysFail(t *testing.T) {
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictPASS}
	fb := &fakeBridge{err: transientBridgeErr(85)} // no artifact written
	notOK := deliverable.Result{OK: false, Violations: []deliverable.Violation{{Code: deliverable.CodeMissingArtifact, Message: "deliverable not found"}}}
	r := New(Options{
		Hooks:    hooks,
		Bridge:   fb,
		Prompts:  fakePromptsFS("evolve-auditor", "x"),
		VerifyFn: verifyReturns(notOK, nil),
		SleepFn:  func(time.Duration) {}, // skip the settle-retry delay on the miss path
	})

	resp, err := r.Run(context.Background(), core.PhaseRequest{Workspace: t.TempDir()})
	if err == nil {
		t.Fatal("a malformed/absent deliverable on a transient teardown must hard-fail on a mandatory phase")
	}
	if !errors.Is(err, core.ErrTransientBridgeFailure) {
		t.Errorf("error should wrap ErrTransientBridgeFailure; got %v", err)
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

func TestRun_TransientError_NotWellFormed_Optional_DegradesToWarn(t *testing.T) {
	hooks := &fakeHooks{phase: "build-planner", agent: "evolve-build-planner", model: "opus", prompt: "x"}
	fb := &fakeBridge{err: transientBridgeErr(85)}
	notOK := deliverable.Result{OK: false, Violations: []deliverable.Violation{{Code: deliverable.CodeMissingArtifact, Message: "deliverable not found"}}}
	r := New(Options{
		Hooks:    hooks,
		Bridge:   fb,
		Prompts:  fakePromptsFS("evolve-build-planner", "x"),
		Optional: true,
		VerifyFn: verifyReturns(notOK, nil),
		SleepFn:  func(time.Duration) {},
	})

	resp, err := r.Run(context.Background(), core.PhaseRequest{Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("optional + transient + no-deliverable must degrade to WARN with a nil error; got %v", err)
	}
	if resp.Verdict != core.VerdictWARN {
		t.Errorf("verdict=%q, want WARN (optional degrade)", resp.Verdict)
	}
	if resp.Reconciled {
		t.Error("resp.Reconciled must be false — nothing was reconciled")
	}
}

func TestRun_TransientError_WellFormedPASS_Optional_ReconcilesToPass(t *testing.T) {
	hooks := &fakeHooks{phase: "build-planner", agent: "evolve-build-planner", model: "opus", prompt: "x", verdict: core.VerdictPASS}
	fb := &fakeBridge{err: transientBridgeErr(80), writeArtifact: "# plan\n"}
	r := New(Options{
		Hooks:    hooks,
		Bridge:   fb,
		Prompts:  fakePromptsFS("evolve-build-planner", "x"),
		Optional: true,
		VerifyFn: verifyReturns(deliverable.Result{OK: true}, nil),
	})

	resp, err := r.Run(context.Background(), core.PhaseRequest{Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("optional + well-formed-PASS on a transient teardown must reconcile to nil error; got %v", err)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("verdict=%q, want PASS (reconciled up from WARN)", resp.Verdict)
	}
	if !resp.Reconciled {
		t.Error("resp.Reconciled must be true")
	}
}

func TestRun_TransientError_DeliverableSettlesOnRetry_ReconcilesToPass(t *testing.T) {
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictPASS}
	fb := &fakeBridge{err: transientBridgeErr(86), writeArtifact: "# audit\n<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"PASS\"} -->\n"}
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
		SleepFn:  func(time.Duration) {},
	})

	resp, err := r.Run(context.Background(), core.PhaseRequest{Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("a deliverable that settles within the retry window must reconcile to a NIL error on a transient teardown; got %v", err)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("verdict=%q, want PASS (reconciled after settle-retry)", resp.Verdict)
	}
	if !resp.Reconciled {
		t.Error("resp.Reconciled must be true once the settling deliverable is caught")
	}
	if calls < 3 {
		t.Errorf("settle-retry must re-verify a settling deliverable on the transient path; got only %d verify calls", calls)
	}
}
