package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func seedStateJSON(t *testing.T, evolveDir string, body string) {
	t.Helper()
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "state.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write state.json: %v", err)
	}
}

// writeStdoutLog seeds both the legacy <phase>-stdout.log and the unified
// <phase>-events.ndjson that cyclecost reads.
// See ADR-0020.
func writeStdoutLog(t *testing.T, workspace, phase string, costUSD float64) {
	t.Helper()
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	raw, _ := json.Marshal(map[string]any{
		"type":           "result",
		"total_cost_usd": costUSD,
		"usage": map[string]any{
			"input_tokens": 100, "output_tokens": 50,
			"cache_read_input_tokens": 0, "cache_creation_input_tokens": 0,
		},
	})
	if err := os.WriteFile(filepath.Join(workspace, phase+"-stdout.log"), raw, 0o644); err != nil {
		t.Fatalf("write stdout log: %v", err)
	}
	env, _ := json.Marshal(map[string]any{
		"schema_version": "2.0", "seq": 1, "kind": "result", "severity": "INFO",
		"data": map[string]any{
			"cost_usd": costUSD,
			"tokens":   map[string]any{"in": 100, "out": 50, "cache_r": 0, "cache_c": 0},
		},
	})
	if err := os.WriteFile(filepath.Join(workspace, phase+"-events.ndjson"), env, 0o644); err != nil {
		t.Fatalf("write events log: %v", err)
	}
}

func TestRunLoop_M5_RecordsRecoverable(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	seedStateJSON(t, evolveDir, `{"lastCycleNumber": 0}`)
	writeDispatchPolicy(t, evolveDir, "verify")

	storage := &fixtures.FakeStorage{}
	ledger := newFakeLedger() // empty → verify fails
	defer installStubDeps(t, storage, ledger)()

	workspace := cycleWorkspace(projectRoot, 1)
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("mkdir ws: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "orchestrator-report.md"),
		[]byte("Build status: FAIL — tests RED\n"), 0o644); err != nil {
		t.Fatalf("write report: %v", err)
	}

	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--goal-text", "test",
		"--cycles", "1",
	}, nil, &stdout, &stderr)

	if rc != 3 {
		t.Fatalf("rc=%d want 3 (recoverable); stderr=%q", rc, stderr.String())
	}
	stateBytes, err := os.ReadFile(filepath.Join(evolveDir, "state.json"))
	if err != nil {
		t.Fatalf("read state.json: %v", err)
	}
	var state map[string]any
	if err := json.Unmarshal(stateBytes, &state); err != nil {
		t.Fatalf("parse state: %v", err)
	}
	entries, ok := state["failedApproaches"].([]any)
	if !ok || len(entries) != 1 {
		t.Fatalf("failedApproaches len=%d want 1; state=%v", len(entries), state)
	}
	entry := entries[0].(map[string]any)
	// Either is acceptable: which of Verdict/build-fail wins depends on ordering.
	class := entry["classification"].(string)
	if class != "code-audit-fail" && class != "code-build-fail" {
		t.Fatalf("classification=%q want code-{audit,build}-fail", class)
	}
}

func TestRunLoop_M5_AutoPruneAtStart(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"),
		[]byte(`{"dispatch":{"policy":"off"},"workflow":{"auto_prune":true}}`), 0o644); err != nil {
		t.Fatalf("write policy: %v", err)
	}
	state := map[string]any{
		"failedApproaches": []any{
			map[string]any{"cycle": float64(1), "expiresAt": "2020-01-01T00:00:00Z"},
			map[string]any{"cycle": float64(2), "expiresAt": "2099-01-01T00:00:00Z"},
		},
	}
	b, _ := json.Marshal(state)
	if err := os.WriteFile(filepath.Join(evolveDir, "state.json"), b, 0o644); err != nil {
		t.Fatalf("write state: %v", err)
	}

	storage := &fixtures.FakeStorage{}
	ledger := newFakeLedger()
	defer installStubDeps(t, storage, ledger)()

	var stdout, stderr bytes.Buffer
	_ = runLoop([]string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--goal-text", "test",
		"--cycles", "1",
	}, nil, &stdout, &stderr)

	stateBytes, err := os.ReadFile(filepath.Join(evolveDir, "state.json"))
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	var after map[string]any
	if err := json.Unmarshal(stateBytes, &after); err != nil {
		t.Fatalf("parse: %v", err)
	}
	entries := after["failedApproaches"].([]any)
	if len(entries) != 1 {
		t.Fatalf("after prune: len=%d want 1; entries=%v", len(entries), entries)
	}
	if !strings.Contains(stderr.String(), "auto-prune: removed 1 expired") {
		t.Fatalf("stderr should report auto-prune: %q", stderr.String())
	}
}

func TestRunLoop_M5_CostAccumulationLogged(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeDispatchPolicy(t, evolveDir, "off")

	storage := &fixtures.FakeStorage{}
	ledger := newFakeLedger()
	defer installStubDeps(t, storage, ledger)()

	workspace := cycleWorkspace(projectRoot, 1)
	writeStdoutLog(t, workspace, "scout", 0.15)
	writeStdoutLog(t, workspace, "builder", 0.30)

	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--goal-text", "test",
		"--cycles", "1",
		"--batch-cap-usd", "1.0",
	}, nil, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc=%d want 0; stderr=%q", rc, stderr.String())
	}
	if !strings.Contains(stderr.String(), "cycle 1 cost: $0.4500") {
		t.Fatalf("expected cost log line; stderr=%q", stderr.String())
	}
	var out map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("parse stdout: %v", err)
	}
	if got, ok := out["total_cost_usd"].(float64); !ok || got <= 0 {
		t.Fatalf("total_cost_usd=%v want >0", out["total_cost_usd"])
	}
}
