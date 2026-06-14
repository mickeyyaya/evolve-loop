package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

// ============================================================================
// Deprecated cost flags are accepted no-ops
// ============================================================================

// TestParseLoopArgs_BudgetAliasAccepted verifies --budget-usd and its --budget
// alias still parse without error (so existing scripts/CI don't break) but no
// longer drive any behavior — cost is display-only telemetry now, and the flag
// must NOT bump the cycle count (the former budget-mode 50-cycle default is gone).
func TestParseLoopArgs_BudgetAliasAccepted(t *testing.T) {
	t.Parallel()
	for _, flag := range []string{"--budget-usd", "--budget"} {
		flag := flag
		t.Run(flag, func(t *testing.T) {
			t.Parallel()
			var stderr bytes.Buffer
			cfg, rc := parseLoopArgs([]string{flag, "7.50", "test goal"}, &stderr)
			if rc != 0 {
				t.Fatalf("%s: rc=%d (stderr=%q)", flag, rc, stderr.String())
			}
			if cfg.MaxCycles != 1 {
				t.Fatalf("%s must not drive cycle count: MaxCycles=%d want 1", flag, cfg.MaxCycles)
			}
		})
	}
}

// ============================================================================
// Deprecation WARN
// ============================================================================

// TestParseLoopArgs_NegativeBudgetAccepted — --budget-usd is a deprecated no-op,
// so ANY value (incl. negative) is accepted and ignored. Previously a negative
// value was a hard rc=10 error; that validation was removed for consistency.
func TestParseLoopArgs_NegativeBudgetAccepted(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	cfg, rc := parseLoopArgs([]string{"--budget-usd", "-1", "goal"}, &stderr)
	if rc != 0 {
		t.Fatalf("rc=%d want 0 (--budget-usd is an accepted no-op); stderr=%q", rc, stderr.String())
	}
	if cfg.MaxCycles != 1 {
		t.Fatalf("MaxCycles=%d want 1 (budget flag must not affect cycles)", cfg.MaxCycles)
	}
}

func TestParseLoopArgs_LegacyPositionalIntegerWarn(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	cfg, rc := parseLoopArgs([]string{"3", "balanced", "goal"}, &stderr)
	if rc != 0 {
		t.Fatalf("rc=%d want 0", rc)
	}
	if cfg.MaxCycles != 3 {
		t.Fatalf("MaxCycles=%d want 3", cfg.MaxCycles)
	}
	if !strings.Contains(stderr.String(), "deprecated") {
		t.Fatalf("expected deprecation WARN; stderr=%q", stderr.String())
	}
}

func TestParseLoopArgs_ExplicitCyclesNoWarn(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	_, _ = parseLoopArgs([]string{"--cycles", "3", "goal"}, &stderr)
	if strings.Contains(stderr.String(), "deprecated") {
		t.Fatalf("--cycles flag should not trigger deprecation WARN; stderr=%q", stderr.String())
	}
}

// ============================================================================
// Gap #3: QUOTA-PAUSE detection
// ============================================================================

func TestDetectQuotaPause_HappyPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cs := map[string]any{
		"cycle_id": float64(7),
		"checkpoint": map[string]any{
			"enabled":               true,
			"reason":                "quota-likely",
			"quotaResetAt":          "2026-05-23T12:00:00Z",
			"quotaResetSource":      "api-429",
			"autoResumeAttempts":    float64(1),
			"autoResumeMaxAttempts": float64(3),
		},
	}
	b, _ := json.Marshal(cs)
	if err := os.WriteFile(filepath.Join(dir, "cycle-state.json"), b, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	qp, ok := detectQuotaPause(dir)
	if !ok {
		t.Fatalf("detectQuotaPause returned !ok")
	}
	if qp.Cycle != 7 || qp.WakeAt != "2026-05-23T12:00:00Z" || qp.Source != "api-429" ||
		qp.Attempts != 1 || qp.MaxAttempts != 3 {
		t.Fatalf("quotaPause mismatch: %+v", qp)
	}
}

func TestDetectQuotaPause_NotFlagged(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		cs   any
	}{
		{"missing cycle-state.json", nil},
		{"malformed json", "{not json"},
		{"no checkpoint block", map[string]any{"cycle_id": float64(1)}},
		{"checkpoint disabled", map[string]any{"checkpoint": map[string]any{"enabled": false, "reason": "quota-likely"}}},
		{"different reason", map[string]any{"checkpoint": map[string]any{"enabled": true, "reason": "operator-pause"}}},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			if tc.cs != nil {
				var raw []byte
				if s, ok := tc.cs.(string); ok {
					raw = []byte(s)
				} else {
					raw, _ = json.Marshal(tc.cs)
				}
				if err := os.WriteFile(filepath.Join(dir, "cycle-state.json"), raw, 0o644); err != nil {
					t.Fatalf("write: %v", err)
				}
			}
			if _, ok := detectQuotaPause(dir); ok {
				t.Fatalf("detectQuotaPause should not fire for %s", tc.name)
			}
		})
	}
}

func TestDetectQuotaPause_FallbackFields(t *testing.T) {
	t.Parallel()
	// Cycle from top-level "cycle" (not cycle_id), default Source
	// when quotaResetSource missing.
	dir := t.TempDir()
	cs := map[string]any{
		"cycle": float64(4),
		"checkpoint": map[string]any{
			"enabled": true,
			"reason":  "quota-likely",
		},
	}
	b, _ := json.Marshal(cs)
	_ = os.WriteFile(filepath.Join(dir, "cycle-state.json"), b, 0o644)
	qp, ok := detectQuotaPause(dir)
	if !ok {
		t.Fatalf("not flagged")
	}
	if qp.Cycle != 4 {
		t.Fatalf("Cycle=%d want 4 (top-level fallback)", qp.Cycle)
	}
	if qp.Source != "unknown" {
		t.Fatalf("Source=%q want unknown (default)", qp.Source)
	}
	if qp.MaxAttempts != 3 {
		t.Fatalf("MaxAttempts=%d want 3 (default)", qp.MaxAttempts)
	}
}

// TestRunLoop_QuotaPause_Rc5 drives runLoop end-to-end with a
// cycle-state.json checkpoint that triggers quota-pause → rc=5.
func TestRunLoop_QuotaPause_Rc5(t *testing.T) {
	t.Setenv("EVOLVE_DISPATCH_POLICY", "off")
	t.Setenv("EVOLVE_AUTO_PRUNE", "0")

	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	storage := &fixtures.FakeStorage{}
	ledger := newFakeLedger()
	defer installStubDeps(t, storage, ledger)()

	// Pre-seed the quota-likely cycle-state. detectQuotaPause runs
	// AFTER each cycle's RunCycle, so the cycle-state must be present
	// at that point. Since our stub orchestrator doesn't write
	// cycle-state.json, we seed it before runLoop starts.
	cs := map[string]any{
		"cycle_id": float64(1),
		"checkpoint": map[string]any{
			"enabled":      true,
			"reason":       "quota-likely",
			"quotaResetAt": "2026-05-23T20:00:00Z",
		},
	}
	b, _ := json.Marshal(cs)
	_ = os.WriteFile(filepath.Join(evolveDir, "cycle-state.json"), b, 0o644)

	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--goal-text", "x",
		"--cycles", "1",
	}, nil, &stdout, &stderr)
	if rc != 5 {
		t.Fatalf("rc=%d want 5 (quota-pause); stderr=%q", rc, stderr.String())
	}
	if !strings.Contains(stderr.String(), "QUOTA-PAUSE: cycle=1 wake-at=2026-05-23T20:00:00Z") {
		t.Fatalf("expected QUOTA-PAUSE marker; stderr=%q", stderr.String())
	}
	if !strings.Contains(stdout.String(), `"stop_reason": "quota-pause"`) {
		t.Fatalf("stop_reason should be quota-pause; stdout=%q", stdout.String())
	}
}

// ============================================================================
// --reset Go-side pruning
// ============================================================================

func TestRunLoop_ResetPrunesAtStart(t *testing.T) {
	t.Setenv("EVOLVE_DISPATCH_POLICY", "off")
	t.Setenv("EVOLVE_AUTO_PRUNE", "0")

	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Seed state.json with one entry per class — 3 should be pruned by
	// --reset, 1 (code-audit-fail) should remain.
	state := map[string]any{
		"failedApproaches": []any{
			map[string]any{"cycle": float64(1), "classification": "infrastructure-systemic"},
			map[string]any{"cycle": float64(2), "classification": "infrastructure-transient"},
			map[string]any{"cycle": float64(3), "classification": "ship-gate-config"},
			map[string]any{"cycle": float64(4), "classification": "code-audit-fail"},
		},
	}
	b, _ := json.Marshal(state)
	_ = os.WriteFile(filepath.Join(evolveDir, "state.json"), b, 0o644)

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
	if !strings.Contains(stderr.String(), "--reset: pruned 3") {
		t.Fatalf("expected '--reset: pruned 3' in stderr; got %q", stderr.String())
	}
	raw, _ := os.ReadFile(filepath.Join(evolveDir, "state.json"))
	var after map[string]any
	_ = json.Unmarshal(raw, &after)
	left := after["failedApproaches"].([]any)
	if len(left) != 1 {
		t.Fatalf("expected 1 entry after prune, got %d: %v", len(left), left)
	}
	if left[0].(map[string]any)["classification"].(string) != "code-audit-fail" {
		t.Fatalf("kept entry should be code-audit-fail; got %v", left[0])
	}
}

// ============================================================================
// Deprecated cost env-vars are inert
// ============================================================================

// TestRunLoop_DeprecatedCostEnvVarsInert verifies the former budget env-vars
// (EVOLVE_CHECKPOINT_DISABLE / EVOLVE_BATCH_BUDGET_DISABLE) no longer change
// behavior: cost is always summarized as display-only telemetry, no BATCH-BUDGET
// output is ever emitted, and the loop exits 0 regardless of cost.
func TestRunLoop_DeprecatedCostEnvVarsInert(t *testing.T) {
	t.Setenv("EVOLVE_DISPATCH_POLICY", "off")
	t.Setenv("EVOLVE_AUTO_PRUNE", "0")
	t.Setenv("EVOLVE_CHECKPOINT_DISABLE", "1")
	t.Setenv("EVOLVE_BATCH_BUDGET_DISABLE", "1")

	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	_ = os.MkdirAll(evolveDir, 0o755)
	storage := &fixtures.FakeStorage{}
	ledger := newFakeLedger()
	defer installStubDeps(t, storage, ledger)()
	// $2.50 of the former $1.00 cap — would once have tripped rc=4; now inert.
	writeStdoutLog(t, cycleWorkspace(projectRoot, 1), "scout", 2.50)

	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--goal-text", "x",
		"--cycles", "1",
		"--batch-cap-usd", "1.0",
	}, nil, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc=%d want 0 (cost env-vars are inert); stderr=%q", rc, stderr.String())
	}
	if strings.Contains(stderr.String(), "BATCH-BUDGET") {
		t.Fatalf("no BATCH-BUDGET output expected; got %q", stderr.String())
	}
	// Cost telemetry is always summarized now (the env-var no longer skips it).
	if !strings.Contains(stderr.String(), "cycle 1 cost: $2.5000") {
		t.Fatalf("expected cycle cost still logged as telemetry; got %q", stderr.String())
	}
}

// ============================================================================
// Helper: writeStdoutLog from cmd_loop_m5_test.go is shared — declared there
// ============================================================================
// (compile-time guard so this file's helpers don't clash)
var _ = fmt.Sprintf("placeholder")
