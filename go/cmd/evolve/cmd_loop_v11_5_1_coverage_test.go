package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunLoop_ResetErrorLogged covers the PruneByClassification error
// branch in the --reset wiring. Achieved by setting state.json to a
// directory (so atomic write fails on the rename step).
func TestRunLoop_ResetErrorLogged(t *testing.T) {
	t.Setenv("EVOLVE_DISPATCH_POLICY", "off")
	t.Setenv("EVOLVE_AUTO_PRUNE", "0")

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

	storage := &fakeStorage{}
	ledger := &fakeLedger{}
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

// TestRunLoop_BudgetDrivenCapExceeded covers the
// `cfg.BudgetDriven && totalCost > BatchCapUSD` branch — budget-driven
// mode hits the OUTER cap (rare; budget < cap normally but with a
// huge cost spike this can fire).
func TestRunLoop_BudgetDrivenCapExceeded(t *testing.T) {
	t.Setenv("EVOLVE_DISPATCH_POLICY", "off")
	t.Setenv("EVOLVE_AUTO_PRUNE", "0")

	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	_ = os.MkdirAll(evolveDir, 0o755)
	storage := &fakeStorage{}
	ledger := &fakeLedger{}
	defer installStubDeps(t, storage, ledger)()

	// Cycle cost $1.50 — exceeds both budget ($0.50) AND batch cap
	// ($1.00). Budget-driven mode treats this as success rc=0 with
	// stop_reason=budget via the cap-overrun branch.
	writeStdoutLog(t, cycleWorkspace(projectRoot, 1), "scout", 1.50)

	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--goal-text", "x",
		"--budget-usd", "0.50",
		"--batch-cap-usd", "1.00",
	}, nil, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc=%d want 0 (budget-driven cap-overrun = success); stderr=%q", rc, stderr.String())
	}
	if !strings.Contains(stderr.String(), "BUDGET-EXHAUSTED") {
		t.Fatalf("expected BUDGET-EXHAUSTED log; stderr=%q", stderr.String())
	}
}
