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

// Reconcile-on-transient (cycle-835): a TRANSIENT bridge failure (exit 80/85/86 —
// quota exhaustion, liveness-exhaustion) is an infra teardown, NOT a verdict, in
// exactly the same way an artifact-wait timeout (exit 81) is. The agent may have
// written its contracted deliverable before the infra tore the session down. The
// classic case: agy+codex are quota-walled, so the deep-tier Opus auditor runs on
// an overloaded claude and hits its OWN exit=85 at the tail — AFTER writing a
// complete PASS audit report. Before this fix that report was discarded (the
// non-timeout `else` branch hard-failed without consulting disk) and a genuinely
// PASS-audited cycle was recorded FAIL. The reconcile trigger must cover both
// infra-teardown shapes so a well-formed deliverable is trusted regardless of
// WHICH infra event ended the session.

// transientBridgeErr mimics exactly what bridge.Engine.Launch returns on a
// transient exit code — a wrapped core.ErrTransientBridgeFailure — so the
// runner's errors.Is match is exercised against the real wire shape.
func transientBridgeErr(code int) error {
	return fmt.Errorf("bridge: launch exit=%d: %w", code, core.ErrTransientBridgeFailure)
}

// TestRun_TransientError_WellFormedPASS_ReconcilesToPass — the core cycle-835
// fix: a transient (exit 85 quota) teardown + a well-formed PASS deliverable →
// reconcile to PASS with a nil error, Reconciled=true, and Classify actually ran
// (proves the fall-through, not a bare sentinel read). RED before the fix: the
// non-timeout `else` branch hard-fails, so err != nil and Classify never runs.
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

// TestRun_TransientError_SentinelFAIL_StaysFail — reconciliation only UPGRADES
// toward the agent's real verdict; it never invents a PASS. A well-formed
// deliverable whose Classify verdict is FAIL stays FAIL, and because the
// deliverable is COMPLETE the phase is a normal completed FAIL (nil error → routes
// as a real audit-fail, not an infra-transient retry).
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

// TestRun_TransientError_NotWellFormed_Mandatory_StaysFail — a transient teardown
// where the agent left NO trustworthy deliverable (hung / partial / malformed) on
// a MANDATORY phase → hard-FAIL, Classify NOT reached, error wraps the transient
// sentinel. This is the guard that reconciliation can't ship a hung agent just
// because the teardown was infra-shaped.
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

// TestRun_TransientError_NotWellFormed_Optional_DegradesToWarn — an OPTIONAL
// phase hitting a transient teardown with no trustworthy deliverable degrades to
// WARN+advance (its successor is verdict-unconditional), mirroring the timeout
// optional-degrade path. NOTE: this is the INFRA-shaped branch; a NON-infra
// (launch/safety) error on an optional phase still stays cycle-fatal — that
// invariant lives in TestRun_NonTimeoutError_StaysFail_Unchanged /
// TestRun_OptionalPhase_OtherBridgeError_StillFails and is unaffected here.
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

// TestRun_TransientError_WellFormedPASS_Optional_ReconcilesToPass — an optional
// phase reconciles UP past the WARN degrade when the deliverable is clean.
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

// TestRun_TransientError_DeliverableSettlesOnRetry_ReconcilesToPass — the bounded
// settle-retry applies to transient teardowns too: a deliverable still settling to
// disk (first verifies miss, a later one within the window catches the PASS) must
// reconcile, exactly as on the timeout path (cycles 824/825 settle-race).
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
