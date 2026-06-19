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

// TestRunLoop_ResetErrorLogged covers the PruneByClassification error
// branch in the --reset wiring. Achieved by setting state.json to a
// directory (so atomic write fails on the rename step).
func TestRunLoop_ResetErrorLogged(t *testing.T) {

	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	// Seed state.json as a regular file with classifications that
	// would normally be pruned by --reset.
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	state := map[string]any{
		"failedApproaches": []any{
			map[string]any{"cycle": float64(1), "classification": "infrastructure-systemic"},
		},
	}
	b, _ := json.Marshal(state)
	if err := os.WriteFile(filepath.Join(evolveDir, "state.json"), b, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	writeDispatchPolicy(t, evolveDir, "off")
	// Make evolveDir read-only so the tmp+mv atomic write fails.
	// We need state.json itself to be readable but the directory to
	// reject new files — chmod 555 on the dir.
	if err := os.Chmod(evolveDir, 0o555); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	defer os.Chmod(evolveDir, 0o755) // restore for cleanup

	if os.Geteuid() == 0 {
		t.Skip("running as root — chmod 555 doesn't restrict writes")
	}

	storage := &fixtures.FakeStorage{}
	ledger := newFakeLedger()
	defer installStubDeps(t, storage, ledger)()

	var stdout, stderr bytes.Buffer
	_ = runLoop([]string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--goal-text", "x",
		"--cycles", "1",
		"--reset",
	}, nil, &stdout, &stderr)
	if !strings.Contains(stderr.String(), "[loop] --reset:") {
		t.Fatalf("expected --reset error in stderr; got %q", stderr.String())
	}
}

// TestRunLoop_BudgetFlagsDoNotStop verifies the deprecated cost flags no longer
// stop the loop: even when the cycle cost exceeds both --budget-usd and
// --batch-cap-usd, the run completes its cycles normally (rc=0, no
// BUDGET-EXHAUSTED / batch_cap), with cost reported as telemetry only.
func TestRunLoop_BudgetFlagsDoNotStop(t *testing.T) {

	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	_ = os.MkdirAll(evolveDir, 0o755)
	writeDispatchPolicy(t, evolveDir, "off")
	storage := &fixtures.FakeStorage{}
	ledger := newFakeLedger()
	defer installStubDeps(t, storage, ledger)()

	// Cycle cost $1.50 exceeds both the (deprecated) budget ($0.50) and batch
	// cap ($1.00). Neither gates anymore — the loop runs its one cycle and exits 0.
	writeStdoutLog(t, cycleWorkspace(projectRoot, 1), "scout", 1.50)

	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--goal-text", "x",
		"--cycles", "1",
		"--budget-usd", "0.50",
		"--batch-cap-usd", "1.00",
	}, nil, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc=%d want 0 (cost flags are no-ops); stderr=%q", rc, stderr.String())
	}
	if strings.Contains(stderr.String(), "BUDGET-EXHAUSTED") || strings.Contains(stderr.String(), "BATCH-BUDGET") {
		t.Fatalf("cost flags must not emit budget stops; stderr=%q", stderr.String())
	}
	if !strings.Contains(stdout.String(), `"stop_reason": "max_cycles"`) {
		t.Fatalf("stop_reason should be max_cycles; stdout=%q", stdout.String())
	}
}
