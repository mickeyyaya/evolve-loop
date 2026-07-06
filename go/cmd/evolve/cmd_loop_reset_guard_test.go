package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

// cmd_loop_reset_guard_test.go — a fresh `evolve loop` run must not silently
// clobber an unfinished cycle (which would lose its history). The guard
// detects "cycle-state ahead of lastCycleNumber" and refuses with the
// resume|reset fork.

func TestUnfinishedCycle(t *testing.T) {
	cases := []struct {
		name      string
		cycleID   int
		lastCycle int
		want      bool
	}{
		{"stuck cycle ahead of last", 108, 107, true},
		{"completed cycle (caught up)", 108, 108, false},
		{"no cycle-state", 0, 107, false},
		{"next stuck cycle", 109, 108, true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			cs := core.CycleState{CycleID: tc.cycleID}
			if got := unfinishedCycle(cs, tc.lastCycle); got != tc.want {
				t.Fatalf("unfinishedCycle(%d, %d)=%v want %v", tc.cycleID, tc.lastCycle, got, tc.want)
			}
		})
	}
}

func TestRunLoop_UnfinishedCycleGuardRefuses(t *testing.T) {
	projectRoot, evolveDir := seedResetDir(t, 108, 107)
	// Real storage over the seeded dir; the guard reads it and refuses before
	// any orchestrator phase runs (so the stub orchestrator is never invoked).
	restore := installStubDeps(t, storage.New(evolveDir), ledger.New(evolveDir))
	defer restore()

	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{"--goal-text", "x", "--max-cycles", "1", "--project-root", projectRoot}, nil, &stdout, &stderr)
	if rc != 2 {
		t.Fatalf("rc=%d want 2; stdout=%q stderr=%q", rc, stdout.String(), stderr.String())
	}
	var lr map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &lr); err != nil {
		t.Fatalf("parse loop result: %v (stdout=%q)", err, stdout.String())
	}
	if lr["stop_reason"] != "unfinished_cycle" {
		t.Errorf("stop_reason=%v want unfinished_cycle", lr["stop_reason"])
	}
	// Guidance must name both forks.
	g := stderr.String()
	if !strings.Contains(g, "--resume") || !strings.Contains(g, "cycle reset") {
		t.Errorf("guidance must mention --resume and cycle reset; got %q", g)
	}
}

func TestRunLoop_CorruptedCycleStateRefuses(t *testing.T) {
	projectRoot, evolveDir := seedResetDir(t, 108, 107)
	// Simulate a SIGKILL mid-write: truncated/garbage cycle-state.json.
	if err := os.WriteFile(filepath.Join(evolveDir, "cycle-state.json"), []byte(`{"cycle_id": 108, "phas`), 0o644); err != nil {
		t.Fatalf("corrupt cycle-state: %v", err)
	}
	restore := installStubDeps(t, storage.New(evolveDir), ledger.New(evolveDir))
	defer restore()

	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{"--goal-text", "x", "--max-cycles", "1", "--project-root", projectRoot}, nil, &stdout, &stderr)
	if rc != 2 {
		t.Fatalf("rc=%d want 2 (unreadable cycle-state must block); stderr=%q", rc, stderr.String())
	}
	var lr map[string]any
	_ = json.Unmarshal(stdout.Bytes(), &lr)
	if lr["stop_reason"] != "unfinished_cycle" {
		t.Errorf("stop_reason=%v want unfinished_cycle", lr["stop_reason"])
	}
	if !strings.Contains(stderr.String(), "unreadable") {
		t.Errorf("guidance should flag the unreadable state; got %q", stderr.String())
	}
}

// TestUnfinishedCycleGuard_DeadOwnerFreshLease_NotReportedAsLive — cycle-554
// workspace-hygiene-s1 sibling: the loop's F1-sibling guard (cmd_loop.go:317)
// reads a lease as "owned by a LIVE run" using freshness alone. A crashed
// owner (dead pid) with a still-fresh heartbeat must fall through to the
// normal unfinished_cycle (resume|reset) guidance instead — steering an
// operator at `owned_by_live_run` never to reset would wedge them forever
// against a run that will never come back. PID 999999 is a real, guaranteed-
// dead pid (same convention as TestDefaultBootRecovery_AutosealsDeadOwnerMarker)
// so the production pidAlive probe drives the decision, no injection needed.
// bootRecoverFn is stubbed to a no-op (the established spy-seam idiom, see
// TestRunLoop_InvokesBootRecoveryBeforeGate) so this test isolates the guard's
// OWN liveness check as defense-in-depth, independent of whether boot-time
// AutosealStaleMarker also would have healed the same marker first.
func TestUnfinishedCycleGuard_DeadOwnerFreshLease_NotReportedAsLive(t *testing.T) {
	projectRoot, evolveDir := seedResetDir(t, 108, 107)
	ws := filepath.Join(evolveDir, "runs", "cycle-108")
	if err := runlease.Write(ws, runlease.Lease{RunID: "01RUN", OwnerPID: 999999}, time.Now()); err != nil {
		t.Fatalf("seed lease: %v", err)
	}
	restore := installStubDeps(t, storage.New(evolveDir), ledger.New(evolveDir))
	defer restore()
	prevBR := bootRecoverFn
	defer func() { bootRecoverFn = prevBR }()
	bootRecoverFn = func(context.Context, loopConfig, core.Ledger, io.Writer) bootRecoveryResult {
		return bootRecoveryResult{}
	}

	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{"--goal-text", "x", "--max-cycles", "1", "--project-root", projectRoot}, nil, &stdout, &stderr)
	if rc != 2 {
		t.Fatalf("rc=%d want 2; stdout=%q stderr=%q", rc, stdout.String(), stderr.String())
	}
	var lr map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &lr); err != nil {
		t.Fatalf("parse loop result: %v (stdout=%q)", err, stdout.String())
	}
	if lr["stop_reason"] == "owned_by_live_run" {
		t.Errorf("a dead-owner fresh-lease marker must NOT be reported as owned_by_live_run; stderr=%q", stderr.String())
	}
	if lr["stop_reason"] != "unfinished_cycle" {
		t.Errorf("stop_reason=%v want unfinished_cycle (dead owner falls through to the resume|reset guidance)", lr["stop_reason"])
	}
}

func TestRunLoop_ForceFreshBypassesGuard(t *testing.T) {
	projectRoot, evolveDir := seedResetDir(t, 108, 107)
	restore := installStubDeps(t, storage.New(evolveDir), ledger.New(evolveDir))
	defer restore()

	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{"--goal-text", "x", "--max-cycles", "1", "--project-root", projectRoot, "--force-fresh"}, nil, &stdout, &stderr)
	// With the override the guard does not fire — the loop proceeds (stub
	// runners), so the stop_reason is anything BUT unfinished_cycle.
	var lr map[string]any
	_ = json.Unmarshal(stdout.Bytes(), &lr)
	if lr["stop_reason"] == "unfinished_cycle" {
		t.Fatalf("--force-fresh must bypass the guard; rc=%d stderr=%q", rc, stderr.String())
	}
}
