package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
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
