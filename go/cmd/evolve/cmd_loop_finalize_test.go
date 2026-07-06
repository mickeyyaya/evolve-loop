package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

// cmd_loop_finalize_test.go — RED tests for S2 (workspace-hygiene-2026-07
// plan): a clean `max_cycles` batch exit must clear the completed cycle's
// on-disk cycle-state.json marker (core.ClearCompletedCycleMarker, see
// internal/core/cycle_finalize_test.go), so the operator no longer has to run
// `evolve cycle reset --force` before every relaunch. A signal-interrupted
// exit must NOT clear it — the run may be resumed.
//
// These tests write a REAL cycle-state.json / state.json to evolveDir: the
// finalize call site reads the marker straight off disk
// (core.ResolveCycleStatePath), independent of the in-memory FakeStorage the
// stubbed orchestrator uses for its own cycle bookkeeping — installStubDeps
// (cmd_loop_m4_test.go) does not touch this file, so it is a clean channel to
// assert the new wiring in isolation.

func writeLoopFinalizeFixture(t *testing.T, evolveDir string, cycleID, lastCycleNumber int) {
	t.Helper()
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(`{"dispatch":{"policy":"off"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	stateJSON := `{"failedApproaches":[],"lastCycleNumber":` + strconv.Itoa(lastCycleNumber) + `}`
	if err := os.WriteFile(filepath.Join(evolveDir, "state.json"), []byte(stateJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	csJSON := `{"cycle_id":` + strconv.Itoa(cycleID) + `,"phase":"ship","workspace_path":"` + filepath.ToSlash(filepath.Join(evolveDir, "runs", "cycle-finalize-fixture")) + `"}`
	if err := os.WriteFile(filepath.Join(evolveDir, "cycle-state.json"), []byte(csJSON), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestLoop_MaxCyclesExit_ClearsCompletedMarker: a normal (non-signal,
// non-FAIL) batch exit hitting its --cycles budget must clear a completed
// (cycle_id<=lastCycleNumber, no live owner) on-disk cycle-state.json —
// eliminating the forced `evolve cycle reset --force` before the next launch.
func TestLoop_MaxCyclesExit_ClearsCompletedMarker(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	writeLoopFinalizeFixture(t, evolveDir, 5, 5) // completed, no live owner

	storage := &fixtures.FakeStorage{}
	ledger := newFakeLedger()
	defer installStubDeps(t, storage, ledger)()

	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--goal-text", "finalize goal",
		"--cycles", "1",
	}, nil, &stdout, &stderr)

	if rc != 0 {
		t.Fatalf("rc=%d, want 0 (clean max_cycles completion); stderr=%s", rc, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(evolveDir, "cycle-state.json")); !os.IsNotExist(err) {
		t.Errorf("cycle-state.json still present after a clean max_cycles exit (stat err=%v) — ClearCompletedCycleMarker was not wired at the batch-end call site", err)
	}
}

// TestLoop_SignalExit_PreservesMarker: a SIGINT/SIGTERM-interrupted batch must
// NOT clear the marker — the interrupted cycle (or a still-unfinished prior
// one) must remain resumable via `evolve loop --resume`.
func TestLoop_SignalExit_PreservesMarker(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	writeLoopFinalizeFixture(t, evolveDir, 5, 5) // would clear if (wrongly) finalized

	storage := &fixtures.FakeStorage{}
	ledger := newFakeLedger()
	defer installStubDeps(t, storage, ledger)()

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
		"--goal-text", "finalize goal",
		"--cycles", "5",
	}, nil, &stdout, &stderr)

	if rc != 130 {
		t.Fatalf("rc=%d, want 130 (graceful signal stop); stderr=%s", rc, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(evolveDir, "cycle-state.json")); err != nil {
		t.Errorf("cycle-state.json removed on a signal-interrupted exit (stat err=%v) — must be preserved for --resume", err)
	}
}
