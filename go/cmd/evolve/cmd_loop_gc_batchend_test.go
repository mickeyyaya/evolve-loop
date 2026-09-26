package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

// gcHookCall is one observed gcHookFn invocation. markerPresent /
// cycleHadRun are sampled AT CALL TIME, which is what makes this an ordering
// assertion rather than a mere "was it called" check:
//   - markerPresent==false  ⇒ finalizeCompletedCycle already ran (it deletes
//     the on-disk cycle-state.json marker).
//   - cycleHadRun==true     ⇒ the batch's cycle(s) already executed, so this is
//     the batch-END call and not the batch-START one.
type gcHookCall struct {
	workspace     string
	markerPresent bool
	cycleHadRun   bool
}

// batchEndOrch is the scripted cycle runner for the clean-exit test: it flips
// the shared cycleHadRun flag the spy samples, then reports a clean PASS so the
// batch reaches its max-cycles exit.
type batchEndOrch struct {
	noSignals
	ran *bool
}

func (o *batchEndOrch) RunCycle(_ context.Context, _ core.CycleRequest) (core.CycleResult, error) {
	*o.ran = true
	return core.CycleResult{Cycle: 6, FinalVerdict: core.VerdictPASS}, nil
}

func (o *batchEndOrch) RunCycleFromPhase(ctx context.Context, req core.CycleRequest, _ *core.ResumePoint) (core.CycleResult, error) {
	return o.RunCycle(ctx, req)
}

// installGCHookSpy replaces the gcHookFn seam with a recorder. Returns the
// (growing) call log and the flag the scripted orchestrator flips.
func installGCHookSpy(t *testing.T, evolveDir string) (calls *[]gcHookCall, cycleRan *bool) {
	t.Helper()
	calls = &[]gcHookCall{}
	cycleRan = new(bool)
	prev := gcHookFn
	gcHookFn = func(_ loopConfig, workspace string, _ io.Writer) {
		_, statErr := os.Stat(filepath.Join(evolveDir, "cycle-state.json"))
		*calls = append(*calls, gcHookCall{
			workspace:     workspace,
			markerPresent: statErr == nil,
			cycleHadRun:   *cycleRan,
		})
	}
	t.Cleanup(func() { gcHookFn = prev })
	return calls, cycleRan
}

// TestRunLoopBatch_GCHookFiresAfterFinalizeAtBatchEnd is the crux: the sweep
// must run at batch end, after the final cycle and after finalize. The
// fixture writes a real cycle-state.json marker that is present at batch
// start, so a start-only hook records only markerPresent==true/
// cycleHadRun==false invocations and fails here.
func TestRunLoopBatch_GCHookFiresAfterFinalizeAtBatchEnd(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	writeLoopFinalizeFixture(t, evolveDir, 5, 5) // completed cycle, no live owner

	calls, cycleRan := installGCHookSpy(t, evolveDir)

	storage := &fixtures.FakeStorage{}
	defer installStubDeps(t, storage, newFakeLedger())()

	prevOrch := loopOrchOverride
	loopOrchOverride = &batchEndOrch{ran: cycleRan}
	defer func() { loopOrchOverride = prevOrch }()

	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--goal-text", "batch-end gc goal",
		"--cycles", "1",
	}, nil, &stdout, &stderr)

	if rc != 0 {
		t.Fatalf("rc=%d, want 0 (clean max_cycles completion); stderr=%s", rc, stderr.String())
	}
	if len(*calls) == 0 {
		t.Fatal("gcHookFn was never invoked — the batch-end workspace hygiene sweep is not wired (S5 wiring gap)")
	}
	last := (*calls)[len(*calls)-1]
	if !last.cycleHadRun {
		t.Errorf("the LAST gcHookFn invocation happened before any cycle ran (calls=%+v) — the hook is still batch-START only; S5 requires a batch-END sweep so the batch's own worktrees/run dirs are reapable", *calls)
	}
	if last.markerPresent {
		t.Errorf("the batch-end gcHookFn invocation still saw cycle-state.json on disk (calls=%+v) — the plan requires finalize FIRST, then the hook, so the sweep observes a finalized batch", *calls)
	}
	runsDir := filepath.Join(projectRoot, ".evolve", "runs")
	if last.workspace == "" || !strings.HasPrefix(filepath.Clean(last.workspace), filepath.Clean(runsDir)) {
		t.Errorf("batch-end gcHookFn workspace = %q, want a cycle workspace under %q (the manifests must land in this batch's run dir)", last.workspace, runsDir)
	}
}

// TestRunLoopBatch_SignalExitSkipsBatchEndGCHook is the NEGATIVE half: an
// interrupted batch is resumable, so no batch-end sweep may fire. A blanket
// "always sweep on the way out" implementation passes the test above and fails
// here — which is exactly the no-op-resistance this pair is for.
func TestRunLoopBatch_SignalExitSkipsBatchEndGCHook(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	writeLoopFinalizeFixture(t, evolveDir, 5, 5)

	calls, cycleRan := installGCHookSpy(t, evolveDir)

	storage := &fixtures.FakeStorage{}
	defer installStubDeps(t, storage, newFakeLedger())()

	ctx, cancel := context.WithCancel(context.Background())
	prevSig := loopSignalContext
	loopSignalContext = func(context.Context) (context.Context, context.CancelFunc) { return ctx, cancel }
	defer func() { loopSignalContext = prevSig }()

	prevOrch := loopOrchOverride
	loopOrchOverride = &signalMarkingOrch{cancel: cancel, ran: cycleRan}
	defer func() { loopOrchOverride = prevOrch }()

	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--goal-text", "batch-end gc goal",
		"--cycles", "5",
	}, nil, &stdout, &stderr)

	if rc != 130 {
		t.Fatalf("rc=%d, want 130 (graceful signal stop); stderr=%s", rc, stderr.String())
	}
	for i, c := range *calls {
		if c.cycleHadRun {
			t.Errorf("gcHookFn call #%d fired after the interrupted cycle (%+v) — a signal-interrupted batch stays resumable, so its worktrees/branches must NOT be swept", i, c)
		}
	}
}

// signalMarkingOrch is cancelOnRunOrch plus the cycleHadRun flag the spy
// samples: it marks that a cycle entered, cancels the loop context (simulating
// SIGINT landing mid-cycle), and returns ctx.Err().
type signalMarkingOrch struct {
	noSignals
	cancel context.CancelFunc
	ran    *bool
}

func (o *signalMarkingOrch) RunCycle(ctx context.Context, _ core.CycleRequest) (core.CycleResult, error) {
	*o.ran = true
	o.cancel()
	<-ctx.Done()
	return core.CycleResult{Cycle: 6}, ctx.Err()
}

func (o *signalMarkingOrch) RunCycleFromPhase(ctx context.Context, req core.CycleRequest, _ *core.ResumePoint) (core.CycleResult, error) {
	return o.RunCycle(ctx, req)
}
