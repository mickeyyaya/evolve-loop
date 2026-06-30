package clihealth

import (
	"testing"
	"time"
)

// TestBootTimeout_RepeatedExitBenchesDriver (eval AC1): threshold consecutive
// RecordBootStrike calls for the same driver must bench it (benched=true returned
// on the Nth call) and Active() must contain the driver.
func TestBootTimeout_RepeatedExitBenchesDriver(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStore(dir, func() time.Time { return time.Now() })
	const driver = "codex-tmux"

	for i := 1; i < DefaultBootBenchThreshold; i++ {
		benched, err := s.RecordBootStrike(driver)
		if err != nil {
			t.Fatalf("RecordBootStrike call %d: %v", i, err)
		}
		if benched {
			t.Errorf("RecordBootStrike call %d/%d: benched=true before threshold", i, DefaultBootBenchThreshold)
		}
	}

	// The Nth call must cross the threshold.
	benched, err := s.RecordBootStrike(driver)
	if err != nil {
		t.Fatalf("RecordBootStrike threshold call: %v", err)
	}
	if !benched {
		t.Errorf("RecordBootStrike at threshold (%d): benched=false, want true", DefaultBootBenchThreshold)
	}

	active := s.Active()
	if _, ok := active[driver]; !ok {
		t.Errorf("after threshold strikes, Active() does not contain %q", driver)
	}
}

// TestBootTimeout_SingleStrikeNoBench (eval AC2): one RecordBootStrike must NOT
// bench the driver; transient retry must be preserved.
func TestBootTimeout_SingleStrikeNoBench(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStore(dir, func() time.Time { return time.Now() })

	benched, err := s.RecordBootStrike("claude-tmux")
	if err != nil {
		t.Fatalf("RecordBootStrike: %v", err)
	}
	if benched {
		t.Errorf("single strike: benched=true, want false (threshold=%d)", DefaultBootBenchThreshold)
	}
	if len(s.Active()) != 0 {
		t.Errorf("Active() non-empty after single strike, want empty")
	}
}

// TestBootTimeoutBench_NoCLINameLiteral (eval AC3): the bench-trigger path
// must be driver-agnostic. Verified by benching a completely synthetic driver
// name (if the path hardcoded CLI names, the unknown driver would never bench).
// Also verifies IsBootTimeoutExitCode is exit-code-keyed with no CLI name param.
func TestBootTimeoutBench_NoCLINameLiteral(t *testing.T) {
	t.Parallel()
	// IsBootTimeoutExitCode is a pure function keyed on exit code only (no driver/CLI name).
	if !IsBootTimeoutExitCode(80) {
		t.Error("IsBootTimeoutExitCode(80) = false; exit-80 must be the boot-timeout class")
	}
	if IsBootTimeoutExitCode(81) {
		t.Error("IsBootTimeoutExitCode(81) = true; only exit-80 is the boot-timeout class")
	}

	// Bench an unknown/synthetic driver — if CLI name were hardcoded, this would never bench.
	const synthetic = "future-unknown-driver-tmux"
	dir := t.TempDir()
	s := NewStore(dir, func() time.Time { return time.Now() })
	for i := 0; i < DefaultBootBenchThreshold; i++ {
		if _, err := s.RecordBootStrike(synthetic); err != nil {
			t.Fatalf("RecordBootStrike(%q) call %d: %v", synthetic, i+1, err)
		}
	}
	if _, ok := s.Active()[synthetic]; !ok {
		t.Errorf("bench-trigger must be driver-agnostic: %q not in Active() after threshold strikes; CLI-name hardcoding suspected", synthetic)
	}
}

// TestBootTimeout_NonExitCodeDoesNotStrike (eval AC4): only exit 80 increments
// the strike counter; other exit codes interleaved with <threshold exit-80s
// must leave the driver un-benched.
func TestBootTimeout_NonExitCodeDoesNotStrike(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStore(dir, func() time.Time { return time.Now() })
	const driver = "codex-tmux"

	// Simulate: mixed sequence where only exit-80s call RecordBootStrike.
	for _, code := range []int{0, 85, 124, 127, 1} {
		if IsBootTimeoutExitCode(code) {
			if _, err := s.RecordBootStrike(driver); err != nil {
				t.Fatalf("RecordBootStrike(%d): %v", code, err)
			}
		}
	}
	// One exit-80 (below threshold=2).
	if IsBootTimeoutExitCode(80) {
		if _, err := s.RecordBootStrike(driver); err != nil {
			t.Fatalf("RecordBootStrike(80): %v", err)
		}
	}

	// After 1 exit-80 + several non-80s: still under threshold, must not bench.
	if _, ok := s.Active()[driver]; ok {
		t.Errorf("driver must NOT be benched after 1 exit-80 mixed with non-80 exits (threshold=%d)", DefaultBootBenchThreshold)
	}
}
