package clihealth

import (
	"testing"
	"time"
)

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

func TestBootTimeoutBench_NoCLINameLiteral(t *testing.T) {
	t.Parallel()
	if !IsBootTimeoutExitCode(80) {
		t.Error("IsBootTimeoutExitCode(80) = false; exit-80 must be the boot-timeout class")
	}
	if IsBootTimeoutExitCode(81) {
		t.Error("IsBootTimeoutExitCode(81) = true; only exit-80 is the boot-timeout class")
	}

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

func TestBootTimeout_NonExitCodeDoesNotStrike(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStore(dir, func() time.Time { return time.Now() })
	const driver = "codex-tmux"

	for _, code := range []int{0, 85, 124, 127, 1} {
		if IsBootTimeoutExitCode(code) {
			if _, err := s.RecordBootStrike(driver); err != nil {
				t.Fatalf("RecordBootStrike(%d): %v", code, err)
			}
		}
	}
	if IsBootTimeoutExitCode(80) {
		if _, err := s.RecordBootStrike(driver); err != nil {
			t.Fatalf("RecordBootStrike(80): %v", err)
		}
	}

	if _, ok := s.Active()[driver]; ok {
		t.Errorf("driver must NOT be benched after 1 exit-80 mixed with non-80 exits (threshold=%d)", DefaultBootBenchThreshold)
	}
}
