// cmd_loop_faillearn_test.go — failure-floor Phase 2 (inbox
// retro-always-invariant, gap 3): loop-level fatal exits must produce a
// batch-level failedApproaches entry (classification loop-fatal,
// stop_reason in the summary) plus a deterministic lesson artifact —
// today they only emit the JSON envelope and exit.
//
// Uses the m4 harness (installStubDeps / stuckStorage / noopRunner /
// newFakeLedger) defined in cmd_loop_m4_test.go.
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/test/fixtures"
)

// seedLoopStateFile writes a minimal on-disk state.json (the file
// failurelog.Record appends to — distinct from the FakeStorage state).
func seedLoopStateFile(t *testing.T, evolveDir string) string {
	t.Helper()
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir evolveDir: %v", err)
	}
	path := filepath.Join(evolveDir, "state.json")
	if err := os.WriteFile(path, []byte(`{"lastCycleNumber": 0}`), 0o644); err != nil {
		t.Fatalf("seed state.json: %v", err)
	}
	return path
}

func readFailedApproaches(t *testing.T, statePath string) []map[string]any {
	t.Helper()
	raw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state.json: %v", err)
	}
	var state map[string]any
	if err := json.Unmarshal(raw, &state); err != nil {
		t.Fatalf("parse state.json: %v", err)
	}
	rawList, _ := state["failedApproaches"].([]any)
	out := make([]map[string]any, 0, len(rawList))
	for _, e := range rawList {
		if m, ok := e.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

// TestLoopFatal_CircuitBreaker_AppendsBatchLevelFailedApproach forces the
// dispatcher-deadlock fatal (same harness as TestRunLoop_CircuitBreakerTrips)
// and asserts the batch LEARNS from it.
func TestLoopFatal_CircuitBreaker_AppendsBatchLevelFailedApproach(t *testing.T) {

	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	statePath := seedLoopStateFile(t, evolveDir)
	writeDispatchPolicyFull(t, evolveDir, "off", 3)
	defer installStubDeps(t, &stuckStorage{}, newFakeLedger())()
	if err := os.MkdirAll(cycleWorkspace(projectRoot, 1), 0o755); err != nil {
		t.Fatalf("mkdir ws: %v", err)
	}

	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--goal-text", "test goal",
		"--cycles", "5",
	}, nil, &stdout, &stderr)
	if rc != 1 {
		t.Fatalf("rc=%d want 1 (circuit_breaker); stderr=%q", rc, stderr.String())
	}

	fa := readFailedApproaches(t, statePath)
	if len(fa) != 1 {
		t.Fatalf("failedApproaches = %v, want exactly one loop-fatal entry", fa)
	}
	if got, _ := fa[0]["classification"].(string); got != "loop-fatal" {
		t.Errorf("classification = %q, want loop-fatal", got)
	}
	if got, _ := fa[0]["summary"].(string); !strings.Contains(got, "stop_reason=circuit_breaker") {
		t.Errorf("summary = %q, want stop_reason=circuit_breaker", got)
	}

	lessons, _ := filepath.Glob(filepath.Join(evolveDir, "instincts", "lessons", "*.yaml"))
	if len(lessons) != 1 {
		t.Errorf("want one batch-level lesson artifact, got %v", lessons)
	}
}

// TestLoopMaxCycles_NoFalseFailureLearning: clean exits (max_cycles) must
// NOT write failure learning — the floor fires only on abnormal exits.
func TestLoopMaxCycles_NoFalseFailureLearning(t *testing.T) {

	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	statePath := seedLoopStateFile(t, evolveDir)
	writeDispatchPolicy(t, evolveDir, "off")
	defer installStubDeps(t, &fixtures.FakeStorage{}, newFakeLedger())()

	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--goal-text", "test goal",
		"--cycles", "2",
	}, nil, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc=%d want 0; stderr=%q", rc, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"stop_reason": "max_cycles"`) {
		t.Fatalf("want max_cycles exit; stdout=%q", stdout.String())
	}

	if fa := readFailedApproaches(t, statePath); len(fa) != 0 {
		t.Errorf("clean exit appended failedApproaches: %v", fa)
	}
	if lessons, _ := filepath.Glob(filepath.Join(evolveDir, "instincts", "lessons", "*.yaml")); len(lessons) != 0 {
		t.Errorf("clean exit wrote lessons: %v", lessons)
	}
}

// TestLoopFatal_MissingStateJSON_DoesNotBlockExit: the floor is
// best-effort — recording into a missing state.json must not panic or
// change the exit code (the WARN is the only trace).
func TestLoopFatal_MissingStateJSON_DoesNotBlockExit(t *testing.T) {

	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	} // NOTE: no state.json seeded
	writeDispatchPolicyFull(t, evolveDir, "off", 3)
	defer installStubDeps(t, &stuckStorage{}, newFakeLedger())()
	if err := os.MkdirAll(cycleWorkspace(projectRoot, 1), 0o755); err != nil {
		t.Fatalf("mkdir ws: %v", err)
	}

	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--goal-text", "test goal",
		"--cycles", "5",
	}, nil, &stdout, &stderr)
	if rc != 1 {
		t.Fatalf("rc=%d want 1 (circuit_breaker unchanged by floor); stderr=%q", rc, stderr.String())
	}
	if !strings.Contains(stdout.String(), "circuit_breaker") {
		t.Fatalf("stop_reason must remain circuit_breaker; stdout=%q", stdout.String())
	}
}
