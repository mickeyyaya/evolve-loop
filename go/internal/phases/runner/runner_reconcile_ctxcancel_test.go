package runner

// runner_reconcile_ctxcancel_test.go — verifyreconcile-ctx-cancel-unconditional-
// sleep. The bounded settle-WAIT existed to catch a late deliverable flush, but
// it ignored the phase context: a cancelled cycle (deadline exceeded, operator
// abort) still paid the whole ladder — up to reconcileSettleRetries ×
// reconcileSettleInterval of sleeping PLUS a verify probe per rung, each of which
// now nests deliverable's 500ms write-in-flight grace window. Waiting for a
// deliverable whose phase is already dead buys nothing, so the wait observes
// cancellation. The live-ctx ceiling is unchanged (see the NeverSettles tests in
// runner_reconcile_test.go / runner_cleanexit_test.go, which still burn the full
// bounded window).

import (
	"context"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// TestRun_AlreadyCancelledCtx_SettleWaitDoesNotBurnTheWindow — the phase ctx is
// cancelled before the verdict-source probe. The first verify still runs (it is
// free and can only UPGRADE toward the agent's real on-disk verdict), but the
// loop must not sleep or re-probe even once.
func TestRun_AlreadyCancelledCtx_SettleWaitDoesNotBurnTheWindow(t *testing.T) {
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictFAIL}
	nb := &noisyStdoutBridge{fileContent: "", stdout: "raw scrollback\n"}
	verifyCalls, sleeps := 0, 0
	neverSettles := func(phase string, roots phasecontract.Roots) (deliverable.Result, error) {
		verifyCalls++
		return verifiedFrom(deliverable.Result{OK: false}, phase, roots), nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := New(Options{
		Hooks:    hooks,
		Bridge:   nb,
		Prompts:  fakePromptsFS("evolve-auditor", "x"),
		VerifyFn: neverSettles,
		SleepFn:  func(time.Duration) { sleeps++ },
	})

	if _, err := r.Run(ctx, core.PhaseRequest{Workspace: t.TempDir()}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if sleeps != 0 {
		t.Errorf("a cancelled phase must not sleep out the settle ladder; got %d sleep(s) of %s each", sleeps, reconcileSettleInterval)
	}
	if verifyCalls != 1 {
		t.Errorf("got %d verify call(s), want exactly 1 — the first probe always runs, then a cancelled ctx bails with that result (want ≤1, never the full %d)", verifyCalls, reconcileSettleRetries+1)
	}
}

// TestRun_CancelledCtx_TeardownReconcileStillSettles is the SCOPE guard on the
// cancellation bail, and the reason the teardown call site passes WithoutCancel. On
// the reconcile-on-teardown path a cancelled ctx is frequently the CAUSE of the
// teardown: the tmux driver, on ctx.Err(), takes one final completion poll and
// otherwise exits ExitArtifactTimeout — "laundering a finished session into a
// timeout … the runner's settle-retry was the only thing standing between that
// mislabel and a false FAIL" (driver_tmux_repl.go). So a deliverable that settles on
// a later rung must STILL be caught with a dead ctx, or the cycles-824/825 false-FAIL
// class re-opens: a genuinely-PASS audited cycle discarded and requeued from scratch.
func TestRun_CancelledCtx_TeardownReconcileStillSettles(t *testing.T) {
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictPASS}
	fb := &fakeBridge{err: artifactTimeoutErr(), writeArtifact: verifiedPASS}
	calls := 0
	settling := func(phase string, roots phasecontract.Roots) (deliverable.Result, error) {
		calls++
		if calls < 3 {
			return verifiedFrom(deliverable.Result{OK: false}, phase, roots), nil
		}
		return verifiedFrom(deliverable.Result{OK: true}, phase, roots), nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // the cancel that produced this very teardown
	r := New(Options{
		Hooks:    hooks,
		Bridge:   fb,
		Prompts:  fakePromptsFS("evolve-auditor", "x"),
		VerifyFn: settling,
		SleepFn:  func(time.Duration) {},
	})

	resp, err := r.Run(ctx, core.PhaseRequest{Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("a deliverable settling within the window must reconcile even under a cancelled ctx (the cancel IS the teardown); got %v", err)
	}
	if resp.Verdict != core.VerdictPASS || !resp.Reconciled {
		t.Errorf("verdict=%q reconciled=%v, want PASS+reconciled — the teardown ladder must be cancellation-immune", resp.Verdict, resp.Reconciled)
	}
	if calls < 3 {
		t.Errorf("the teardown settle-retry must keep re-probing under a cancelled ctx; got only %d verify call(s)", calls)
	}
}

// TestRun_CtxCancelledDuringSettleWait_StopsReprobing — cancellation lands while
// the wait is in flight (the common shape: the orchestrator cancels the phase ctx
// as the deadline passes). The post-wait check must bail with the last result
// instead of paying another probe + another interval, bounding the whole
// post-cancel cost at ONE interval.
func TestRun_CtxCancelledDuringSettleWait_StopsReprobing(t *testing.T) {
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictFAIL}
	nb := &noisyStdoutBridge{fileContent: "", stdout: "raw scrollback\n"}
	ctx, cancel := context.WithCancel(context.Background())
	verifyCalls, sleeps := 0, 0
	neverSettles := func(phase string, roots phasecontract.Roots) (deliverable.Result, error) {
		verifyCalls++
		return verifiedFrom(deliverable.Result{OK: false}, phase, roots), nil
	}
	r := New(Options{
		Hooks:    hooks,
		Bridge:   nb,
		Prompts:  fakePromptsFS("evolve-auditor", "x"),
		VerifyFn: neverSettles,
		SleepFn: func(time.Duration) {
			sleeps++
			cancel() // the cycle is torn down mid-wait
		},
	})

	if _, err := r.Run(ctx, core.PhaseRequest{Workspace: t.TempDir()}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if sleeps != 1 {
		t.Errorf("got %d sleep(s), want exactly 1 — cancellation during the wait must cost at most one interval", sleeps)
	}
	if verifyCalls != 1 {
		t.Errorf("got %d verify call(s), want 1 — a ctx cancelled during the wait must not trigger another probe (each nests deliverable's grace window)", verifyCalls)
	}
}
