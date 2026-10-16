package clihealth

import (
	"testing"
	"time"
)

func TestBootStrikeResetOnSuccess(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStore(dir, func() time.Time { return time.Now() })
	const driver = "codex-tmux"

	if _, err := s.RecordBootStrike(driver); err != nil {
		t.Fatalf("RecordBootStrike (strike 1): %v", err)
	}

	if err := s.ClearBootStrike(driver); err != nil {
		t.Fatalf("ClearBootStrike: %v", err)
	}

	benched, err := s.RecordBootStrike(driver)
	if err != nil {
		t.Fatalf("RecordBootStrike (after clear): %v", err)
	}
	if benched {
		t.Errorf("strike→ClearBootStrike→strike: benched=true, want false "+
			"(threshold=%d) — ClearBootStrike must reset the consecutive strike "+
			"counter; the post-clear strike must be treated as strike 1", DefaultBootBenchThreshold)
	}
	if _, ok := s.Active()[driver]; ok {
		t.Errorf("Active() still contains %q after clear→re-strike below threshold; "+
			"driver must not be benched from a non-adjacent failure", driver)
	}
}

func TestClearBootStrikeAbsentIsNoOp(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStore(dir, nil)

	if err := s.ClearBootStrike("nonexistent-driver"); err != nil {
		t.Errorf("ClearBootStrike on absent driver: unexpected error: %v", err)
	}
	if active := s.Active(); len(active) != 0 {
		t.Errorf("Active() = %v after ClearBootStrike on absent driver; want empty", active)
	}
}

func TestClearBootStrikeReasonGuard(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStore(dir, func() time.Time { return time.Now() })

	now := time.Now()
	if err := s.Bench(Entry{
		Family:       "codex-tmux",
		Reason:       "rate_limit",
		BenchedAt:    now,
		BenchedUntil: now.Add(time.Hour),
		Strikes:      1,
	}); err != nil {
		t.Fatalf("Bench: %v", err)
	}

	if err := s.ClearBootStrike("codex-tmux"); err != nil {
		t.Fatalf("ClearBootStrike: %v", err)
	}

	active := s.Active()
	if _, ok := active["codex-tmux"]; !ok {
		t.Errorf("ClearBootStrike removed a rate_limit bench; " +
			"Reason-guard must protect non-BootTimeoutPattern entries")
	}
}

func TestConsecutiveStrikesBenchWithoutReset(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStore(dir, func() time.Time { return time.Now() })
	const driver = "agy-tmux"

	for i := 1; i <= DefaultBootBenchThreshold; i++ {
		benched, err := s.RecordBootStrike(driver)
		if err != nil {
			t.Fatalf("RecordBootStrike call %d: %v", i, err)
		}
		if i < DefaultBootBenchThreshold && benched {
			t.Errorf("RecordBootStrike call %d/%d: benched=true before threshold", i, DefaultBootBenchThreshold)
		}
		if i == DefaultBootBenchThreshold && !benched {
			t.Errorf("RecordBootStrike call %d (threshold): benched=false, want true", i)
		}
	}

	if _, ok := s.Active()[driver]; !ok {
		t.Errorf("Active() does not contain %q after %d consecutive strikes; "+
			"threshold bench must not require ClearBootStrike", driver, DefaultBootBenchThreshold)
	}
}
