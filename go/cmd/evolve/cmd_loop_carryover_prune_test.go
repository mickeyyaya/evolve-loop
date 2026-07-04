package main

// cmd_loop_carryover_prune_test.go — RED test (cycle 507, task
// prune-stale-carryover-todos) for the WIRING of the carryoverTodos TTL prune
// into runLoop's startup. Behavioral, not a source grep: it seeds an EXPIRED
// carryover todo in state.json, runs runLoop far enough to execute the
// AutoPrune block (which sits before the readiness gate), and asserts the
// expired entry is actually gone from state.json on disk.
//
// This closes the "prune wired into cmd_loop.go startup, not dead code" AC the
// same way Task 1 closes its wiring AC — by observing the runtime side effect,
// so a prune function that runLoop never calls fails HERE (the cycle-506
// dead-code trap). The prune must run under the existing wc.AutoPrune flag,
// beside the failedApproaches PruneExpired call.
//
// RED now: runLoop does not yet prune carryoverTodos, so the expired entry
// survives. Do NOT modify this file — wire PruneExpiredCarryoverTodos into the
// AutoPrune block.

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/looppreflight"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func TestRunLoop_AutoPrunesExpiredCarryoverTodos(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	past := time.Now().UTC().Add(-48 * time.Hour).Format(time.RFC3339)
	future := time.Now().UTC().Add(48 * time.Hour).Format(time.RFC3339)
	state := map[string]any{
		"lastCycleNumber": 506,
		"carryoverTodos": []map[string]any{
			{"id": "cycle-400-expired", "expiresAt": past},
			{"id": "cycle-505-fresh", "expiresAt": future},
			{"id": "cycle-366-legacy-no-ttl"},
		},
	}
	raw, _ := json.MarshalIndent(state, "", "  ")
	if err := os.WriteFile(filepath.Join(evolveDir, "state.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}

	prevDeps := wireOrchestratorDepsFn
	defer func() { wireOrchestratorDepsFn = prevDeps }()
	wireOrchestratorDepsFn = func(string, string) orchDeps {
		return orchDeps{Storage: &fixtures.FakeStorage{}, Ledger: newFakeLedger()}
	}
	prevPf := runLoopPreflightFn
	defer func() { runLoopPreflightFn = prevPf }()
	runLoopPreflightFn = func(loopConfig, io.Writer) looppreflight.Result { return forcedHalt() }

	var stdout, stderr bytes.Buffer
	runLoop([]string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--goal-text", "anything",
		"--cycles", "1",
		"--force-fresh",
	}, nil, &stdout, &stderr)

	out, err := os.ReadFile(filepath.Join(evolveDir, "state.json"))
	if err != nil {
		t.Fatalf("read state.json: %v", err)
	}
	got := string(out)
	if strings.Contains(got, "cycle-400-expired") {
		t.Errorf("runLoop startup must prune the EXPIRED carryover todo (dead-code check: PruneExpiredCarryoverTodos is not wired into the AutoPrune block)\n%s", got)
	}
	if !strings.Contains(got, "cycle-505-fresh") {
		t.Errorf("a still-fresh carryover todo must survive startup prune\n%s", got)
	}
	if !strings.Contains(got, "cycle-366-legacy-no-ttl") {
		t.Errorf("an untimestamped legacy carryover todo must never be auto-deleted\n%s", got)
	}
}
