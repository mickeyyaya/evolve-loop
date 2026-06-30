package clihealth

// clihealth_boottimeout_reset_test.go — RED contract for the reset-boot-strike-
// on-success feature (cycle-426 T2): ClearBootStrike removes the
// BootTimeoutPattern bench entry for a given driver so consecutive-strike
// counting restarts after a successful boot (any exit code != ExitREPLBootTimeout).
//
// All tests in this file are RED: ClearBootStrike does not yet exist on *Store.
// They become GREEN when the Builder adds:
//   func (s *Store) ClearBootStrike(driver string) error — removes the entry at
//   key=driver only when the stored Reason == BootTimeoutPattern; no-op on absent
//   driver or on entries with a different Reason.

import (
	"testing"
	"time"
)

// TestBootStrikeResetOnSuccess: strike→ClearBootStrike→strike must NOT bench
// the driver. The successful boot resets the consecutive counter to zero, so
// the subsequent failure is treated as the first strike in a new sequence.
func TestBootStrikeResetOnSuccess(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStore(dir, func() time.Time { return time.Now() })
	const driver = "codex-tmux"

	// First strike (below threshold — not yet benched).
	if _, err := s.RecordBootStrike(driver); err != nil {
		t.Fatalf("RecordBootStrike (strike 1): %v", err)
	}

	// Successful boot: clear the consecutive counter.
	if err := s.ClearBootStrike(driver); err != nil {
		t.Fatalf("ClearBootStrike: %v", err)
	}

	// Second strike after clear: treated as strike 1 of a new sequence.
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

// TestClearBootStrikeAbsentIsNoOp: calling ClearBootStrike on a driver that
// has no bench entry must be a no-op (no error, store unchanged).
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

// TestClearBootStrikeReasonGuard: ClearBootStrike must NOT remove entries
// whose Reason is not BootTimeoutPattern. A rate_limit entry stored under the
// same key must survive a ClearBootStrike call (the rate-limit bench is not
// reset by a successful REPL boot — it is a different classifier pattern).
func TestClearBootStrikeReasonGuard(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewStore(dir, func() time.Time { return time.Now() })

	now := time.Now()
	// Bench a driver with rate_limit — NOT BootTimeoutPattern.
	if err := s.Bench(Entry{
		Family:       "codex-tmux",
		Reason:       "rate_limit",
		BenchedAt:    now,
		BenchedUntil: now.Add(time.Hour),
		Strikes:      1,
	}); err != nil {
		t.Fatalf("Bench: %v", err)
	}

	// ClearBootStrike must leave the rate_limit entry intact.
	if err := s.ClearBootStrike("codex-tmux"); err != nil {
		t.Fatalf("ClearBootStrike: %v", err)
	}

	active := s.Active()
	if _, ok := active["codex-tmux"]; !ok {
		t.Errorf("ClearBootStrike removed a rate_limit bench; " +
			"Reason-guard must protect non-BootTimeoutPattern entries")
	}
}

// TestConsecutiveStrikesBenchWithoutReset: two consecutive RecordBootStrike
// calls WITHOUT a ClearBootStrike between them must still bench the driver
// (threshold behavior unchanged). Guards against an over-eager ClearBootStrike
// that auto-resets after each record.
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
