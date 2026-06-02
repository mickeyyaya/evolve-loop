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
// Gap #1: --budget alias for --budget-usd
// ============================================================================

func TestParseLoopArgs_BudgetAlias(t *testing.T) {
	t.Parallel()
	tests := []struct {
		flag string
		val  string
	}{
		{"--budget-usd", "7.50"},
		{"--budget", "7.50"}, // bash alias
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.flag, func(t *testing.T) {
			t.Parallel()
			var stderr bytes.Buffer
			cfg, rc := parseLoopArgs([]string{tc.flag, tc.val, "test goal"}, &stderr)
			if rc != 0 {
				t.Fatalf("%s %s: rc=%d (stderr=%q)", tc.flag, tc.val, rc, stderr.String())
			}
			if cfg.BudgetUSD != 7.5 {
				t.Fatalf("BudgetUSD=%v want 7.5", cfg.BudgetUSD)
			}
			if !cfg.BudgetDriven {
				t.Fatalf("BudgetDriven=false want true")
			}
		})
	}
}

// ============================================================================
// Gap #4: budget-driven mode safety-cap default
// ============================================================================

func TestParseLoopArgs_BudgetModeDefaultCycles(t *testing.T) {
	// NOT t.Parallel — mutates EVOLVE_BUDGET_MAX_CYCLES.
	var stderr bytes.Buffer
	// Unset env → default 50 safety cap
	t.Setenv("EVOLVE_BUDGET_MAX_CYCLES", "")
	cfg, _ := parseLoopArgs([]string{"--budget-usd", "1.0", "goal"}, &stderr)
	if cfg.MaxCycles != 50 {
		t.Fatalf("default budget-mode MaxCycles=%d want 50", cfg.MaxCycles)
	}
	// Env override
	t.Setenv("EVOLVE_BUDGET_MAX_CYCLES", "12")
	cfg, _ = parseLoopArgs([]string{"--budget", "1.0", "goal"}, &stderr)
	if cfg.MaxCycles != 12 {
		t.Fatalf("override MaxCycles=%d want 12", cfg.MaxCycles)
	}
	// Invalid env → falls back to 50
	t.Setenv("EVOLVE_BUDGET_MAX_CYCLES", "abc")
	cfg, _ = parseLoopArgs([]string{"--budget", "1.0", "goal"}, &stderr)
	if cfg.MaxCycles != 50 {
		t.Fatalf("invalid env MaxCycles=%d want 50 fallback", cfg.MaxCycles)
	}
	// Explicit --cycles overrides budget-mode default
	t.Setenv("EVOLVE_BUDGET_MAX_CYCLES", "")
	cfg, _ = parseLoopArgs([]string{"--budget", "1.0", "--cycles", "3", "goal"}, &stderr)
	if cfg.MaxCycles != 3 {
		t.Fatalf("explicit --cycles=3 MaxCycles=%d want 3", cfg.MaxCycles)
	}
}

// TestParseIntEnv covers the parseIntEnv helper directly (also exercised
// indirectly via the budget-mode default-cycles tests).
func TestParseIntEnv(t *testing.T) {
	tests := []struct {
		val  string
		def  int
		want int
	}{
		{"", 50, 50},
		{"100", 50, 100},
		{"0", 50, 50},  // ≤0 → default
		{"-1", 50, 50}, // ≤0 → default
		{"abc", 50, 50},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.val, func(t *testing.T) {
			t.Setenv("TEST_PARSE_INT_ENV", tc.val)
			if got := parseIntEnv("TEST_PARSE_INT_ENV", tc.def); got != tc.want {
				t.Fatalf("parseIntEnv(%q,%d)=%d want %d", tc.val, tc.def, got, tc.want)
			}
		})
	}
}

// ============================================================================
// Gap #7: budget validation + deprecation WARN
// ============================================================================

func TestParseLoopArgs_NegativeBudgetRejected(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	_, rc := parseLoopArgs([]string{"--budget-usd", "-1", "goal"}, &stderr)
	if rc != 10 {
		t.Fatalf("rc=%d want 10 for negative budget", rc)
	}
	if !strings.Contains(stderr.String(), "must be a positive number") {
		t.Fatalf("stderr missing positive-number diagnostic: %q", stderr.String())
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
	if !strings.Contains(stderr.String(), "deprecated since v8.60.0") {
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
// Gap #2: rc=4 batch_cap overrun + Gap #4 budget-mode success
// ============================================================================

func TestRunLoop_BatchCapOverrun_Rc4(t *testing.T) {
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

	// Cycle 1 cost = $2.50, cap = $1.00 → overrun → rc=4
	writeStdoutLog(t, cycleWorkspace(projectRoot, 1), "scout", 2.50)

	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--goal-text", "x",
		"--cycles", "1",
		"--batch-cap-usd", "1.0",
	}, nil, &stdout, &stderr)
	if rc != 4 {
		t.Fatalf("rc=%d want 4 (batch_cap); stderr=%q", rc, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"stop_reason": "batch_cap"`) {
		t.Fatalf("stop_reason should be batch_cap; stdout=%q", stdout.String())
	}
}

func TestRunLoop_BudgetMode_Rc0(t *testing.T) {
	t.Setenv("EVOLVE_DISPATCH_POLICY", "off")
	t.Setenv("EVOLVE_AUTO_PRUNE", "0")
	t.Setenv("EVOLVE_BUDGET_MAX_CYCLES", "5")

	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	storage := &fixtures.FakeStorage{}
	ledger := newFakeLedger()
	defer installStubDeps(t, storage, ledger)()

	// Cycle 1 cost = $0.60, budget = $0.50 → BUDGET-DRIVEN COMPLETE rc=0
	writeStdoutLog(t, cycleWorkspace(projectRoot, 1), "scout", 0.60)

	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--goal-text", "x",
		"--budget-usd", "0.50",
		"--batch-cap-usd", "20.0",
	}, nil, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc=%d want 0 (budget complete); stderr=%q", rc, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"stop_reason": "budget"`) {
		t.Fatalf("stop_reason should be budget; stdout=%q", stdout.String())
	}
}

// ============================================================================
// Gap #5: --reset Go-side pruning
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
// Gap #7: env opt-outs
// ============================================================================

func TestRunLoop_CheckpointDisabledSkipsWarn(t *testing.T) {
	t.Setenv("EVOLVE_DISPATCH_POLICY", "off")
	t.Setenv("EVOLVE_AUTO_PRUNE", "0")
	t.Setenv("EVOLVE_CHECKPOINT_DISABLE", "1")

	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	_ = os.MkdirAll(evolveDir, 0o755)
	storage := &fixtures.FakeStorage{}
	ledger := newFakeLedger()
	defer installStubDeps(t, storage, ledger)()
	// $0.85 of $1.00 → WARN would normally fire; checkpoint-disabled silences it
	writeStdoutLog(t, cycleWorkspace(projectRoot, 1), "scout", 0.85)

	var stdout, stderr bytes.Buffer
	_ = runLoop([]string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--goal-text", "x",
		"--cycles", "1",
		"--batch-cap-usd", "1.0",
	}, nil, &stdout, &stderr)
	if strings.Contains(stderr.String(), "BATCH-BUDGET WARN") {
		t.Fatalf("BATCH-BUDGET WARN should be suppressed under EVOLVE_CHECKPOINT_DISABLE=1; got %q", stderr.String())
	}
	// Cost tracking still happens.
	if !strings.Contains(stderr.String(), "cost: $0.8500") {
		t.Fatalf("expected cycle cost still logged; got %q", stderr.String())
	}
}

func TestRunLoop_BatchBudgetDisabledSkipsAccounting(t *testing.T) {
	t.Setenv("EVOLVE_DISPATCH_POLICY", "off")
	t.Setenv("EVOLVE_AUTO_PRUNE", "0")
	t.Setenv("EVOLVE_BATCH_BUDGET_DISABLE", "1")

	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	_ = os.MkdirAll(evolveDir, 0o755)
	storage := &fixtures.FakeStorage{}
	ledger := newFakeLedger()
	defer installStubDeps(t, storage, ledger)()
	// $2.50 of $1.00 — would normally rc=4; disabled bypasses that
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
		t.Fatalf("rc=%d want 0 (budget tracking disabled)", rc)
	}
	// No cost line in stderr (accounting fully skipped)
	if strings.Contains(stderr.String(), "cycle 1 cost") {
		t.Fatalf("cost line should be suppressed under BATCH_BUDGET_DISABLE=1; got %q", stderr.String())
	}
	// Total cost in output should remain 0 because we skipped summing.
	if !strings.Contains(stdout.String(), `"total_cost_usd": 0`) {
		t.Fatalf("expected total_cost_usd 0 under BATCH_BUDGET_DISABLE; got %q", stdout.String())
	}
}

// ============================================================================
// Helper: writeStdoutLog from cmd_loop_m5_test.go is shared — declared there
// ============================================================================
// (compile-time guard so this file's helpers don't clash)
var _ = fmt.Sprintf("placeholder")
