package runner

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/deliverable"
)

// runner_teardown_test.go — the file-authoritative read must apply UNCONDITIONALLY at
// teardown (ADR-0072). #336 removed the pane as a verdict-content source, but the file
// read was still gated behind a clean agent-completion signal: on a stall→context-cancel
// teardown whose bridge error is NOT an infra-teardown sentinel, the runner hard-failed
// "without consulting the deliverable" — flipping a green, verified, ship-eligible PASS to
// FAIL (cycle-603/921/931 false-FAIL, verified live on v22.4.1). The runner must consult
// the on-disk deliverable on ANY bridge teardown; the lifecycle anomaly is a WARN, never a
// verdict flip. deliverable.Verify + the ship-guard still gate it (a malformed/absent
// deliverable still hard-fails).

// TestRun_StallTeardown_NonInfraBridgeError_ReconcilesFromFile is the cycle-931 repro: a
// contracted phase whose agent wrote a valid PASS deliverable then stalled, so the session
// was torn down with a NON-infra-teardown error. RED before the fix (else-branch hard-FAIL
// without consulting the deliverable); GREEN after (reconcile → record the file's PASS).
func TestRun_StallTeardown_NonInfraBridgeError_ReconcilesFromFile(t *testing.T) {
	genuinePass := "# audit\n<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"PASS\"} -->\n"
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictPASS}
	// A stall→context-cancel teardown: a bridge error that wraps context.Canceled but is NOT
	// an infra-teardown sentinel (ErrArtifactTimeout / ErrTransientBridgeFailure), plus a valid
	// PASS deliverable already flushed to disk before the stall. Pre-fix this hit the substantive-
	// error branch and hard-failed without consulting the deliverable.
	stallErr := fmt.Errorf("audit: bridge: session stalled after deliverable write: %w", context.Canceled)
	fb := &fakeBridge{err: stallErr, writeArtifact: genuinePass}
	r := New(Options{
		Hooks:    hooks,
		Bridge:   fb,
		Prompts:  fakePromptsFS("evolve-auditor", "x"),
		VerifyFn: verifyReturns(deliverable.Result{OK: true}, nil),
		SleepFn:  func(time.Duration) {},
	})

	resp, err := r.Run(context.Background(), core.PhaseRequest{Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("a reconciled stall-teardown must return nil error (the phase completed via the file); got %v", err)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("verdict=%q, want PASS — a valid on-disk deliverable must be recorded at teardown, not flipped to FAIL by a stall (cycle-931/603 false-FAIL)", resp.Verdict)
	}
	if !resp.Reconciled {
		t.Errorf("resp.Reconciled=false — a teardown that recovered the file verdict must mark Reconciled for the audit ledger trail")
	}
}

// TestRun_LiveCancelledCtx_PlainBridgeError_ReconcilesFromFile proves the runner's
// `ctx.Err() != nil` teardown disjunct INDEPENDENTLY of the engine-level fix (which wraps
// cancelled exits as ErrTransientBridgeFailure): a genuinely-cancelled Run ctx paired with
// a PLAIN, unwrapped bridge error — the shape a non-Engine Bridge could surface — must still
// route to reconcile, not the substantive-error hard-fail. This is the one disjunct not
// covered by the engine's sentinel wrapping.
func TestRun_LiveCancelledCtx_PlainBridgeError_ReconcilesFromFile(t *testing.T) {
	genuinePass := "# audit\n<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"PASS\"} -->\n"
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictPASS}
	plainErr := fmt.Errorf("bridge: driver exited rc=143") // NOT context.Canceled, NOT an infra sentinel
	fb := &fakeBridge{err: plainErr, writeArtifact: genuinePass}
	r := New(Options{
		Hooks:    hooks,
		Bridge:   fb,
		Prompts:  fakePromptsFS("evolve-auditor", "x"),
		VerifyFn: verifyReturns(deliverable.Result{OK: true}, nil),
		SleepFn:  func(time.Duration) {},
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // OUR context is cancelled — the teardown signal, independent of the bridgeErr shape
	resp, err := r.Run(ctx, core.PhaseRequest{Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("a reconciled teardown must return nil error; got %v", err)
	}
	if resp.Verdict != core.VerdictPASS || !resp.Reconciled {
		t.Errorf("a live-cancelled ctx with a PLAIN bridgeErr + valid deliverable must reconcile to PASS via the ctx.Err() disjunct; got verdict=%q reconciled=%v", resp.Verdict, resp.Reconciled)
	}
}

// TestRun_StallTeardown_NoValidDeliverable_StillFails is the anti-gaming complement: a
// bridge teardown with NO trustworthy deliverable (Verify fails) must still hard-fail — the
// unconditional-consult fix may only UPGRADE toward a verified on-disk verdict, never
// launder a FAIL into a PASS or invent a verdict from a crashed session's absent output.
func TestRun_StallTeardown_NoValidDeliverable_StillFails(t *testing.T) {
	hooks := &fakeHooks{phase: "audit", agent: "evolve-auditor", model: "opus", prompt: "x", verdict: core.VerdictPASS}
	stallErr := fmt.Errorf("audit: bridge: session stalled: %w", context.Canceled)
	// No deliverable written; Verify reports the contracted file missing/malformed.
	fb := &fakeBridge{err: stallErr}
	r := New(Options{
		Hooks:    hooks,
		Bridge:   fb,
		Prompts:  fakePromptsFS("evolve-auditor", "x"),
		VerifyFn: verifyReturns(deliverable.Result{OK: false, Violations: []deliverable.Violation{{Code: "MISSING_ARTIFACT", Message: "deliverable not found"}}}, nil),
		SleepFn:  func(time.Duration) {},
	})

	resp, err := r.Run(context.Background(), core.PhaseRequest{Workspace: t.TempDir()})
	if resp.Verdict != core.VerdictFAIL {
		t.Errorf("verdict=%q, want FAIL — a teardown with no trustworthy deliverable must hard-fail (mandatory phase); the fix must never invent a verdict", resp.Verdict)
	}
	if err == nil {
		t.Errorf("a mandatory-phase teardown with no trustworthy deliverable must return a non-nil bridge error")
	}
}
