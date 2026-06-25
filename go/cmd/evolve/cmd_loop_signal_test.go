package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

// cmd_loop_signal_test.go — F4: a SIGINT/SIGTERM must stop the loop GRACEFULLY
// (checkpoint + resumable, rc=130), not on the OS default disposition (the
// silent kill that lost cycles 394/395). The loopSignalContext seam lets the
// test cancel the loop's context exactly as a signal would, without delivering
// a real process signal to the test runner.

// cancelOnRunOrch cancels the loop's (seam-injected) context on the first
// RunCycle — simulating a signal landing mid-cycle — then returns ctx.Err().
type cancelOnRunOrch struct{ cancel context.CancelFunc }

func (o *cancelOnRunOrch) RunCycle(ctx context.Context, _ core.CycleRequest) (core.CycleResult, error) {
	o.cancel()
	<-ctx.Done()
	return core.CycleResult{Cycle: 396}, ctx.Err()
}

func (o *cancelOnRunOrch) RunCycleFromPhase(ctx context.Context, req core.CycleRequest, _ *core.ResumePoint) (core.CycleResult, error) {
	return o.RunCycle(ctx, req)
}

func TestRunLoop_SignalGracefulStop(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(`{"dispatch":{"policy":"off"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "state.json"), []byte(`{"failedApproaches":[],"lastCycleNumber":0}`), 0o644); err != nil {
		t.Fatal(err)
	}

	storage := &fixtures.FakeStorage{}
	ledger := newFakeLedger()
	defer installStubDeps(t, storage, ledger)()

	// Seam: hand runLoop a context the fake can cancel — no real signal.
	ctx, cancel := context.WithCancel(context.Background())
	prevSig := loopSignalContext
	loopSignalContext = func(context.Context) (context.Context, context.CancelFunc) { return ctx, cancel }
	defer func() { loopSignalContext = prevSig }()

	prevOrch := loopOrchOverride
	loopOrchOverride = &cancelOnRunOrch{cancel: cancel}
	defer func() { loopOrchOverride = prevOrch }()

	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--goal-text", "signal goal",
		"--cycles", "5",
	}, nil, &stdout, &stderr)

	if rc != 130 {
		t.Fatalf("rc=%d, want 130 (graceful signal stop); stderr=%s", rc, stderr.String())
	}
	s := stderr.String()
	if !strings.Contains(s, "received interrupt") || !strings.Contains(s, "evolve loop --resume") {
		t.Errorf("expected a graceful, resumable signal message; got:\n%s", s)
	}
}
