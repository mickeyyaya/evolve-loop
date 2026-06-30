package clihealth

// clihealth_amplify_cycle426_test.go — adversarial amplification for cycle-426
// ClearBootStrike lifecycle (T2).  Tests target lifecycle corners the Build
// tests do not probe:
//
//  1. Double-clear idempotency — clearing an already-cleared entry.
//  2. Full re-accumulation after clear — bench→clear→re-bench lifecycle.
//  3. Multi-driver isolation — ClearBootStrike(A) must not affect driver B.

import (
	"testing"
	"time"
)

// TestAmplify_C426_DoubleClearIsIdempotent: after setting a boot-bench and
// clearing it, a second ClearBootStrike on the now-absent entry must be a
// no-op (no error, Active() stays empty).
//
// Distinct from TestClearBootStrikeAbsentIsNoOp which tests a key that was
// NEVER set; here the key was SET then CLEARED, and the second call must not
// fail or corrupt the store.
func TestAmplify_C426_DoubleClearIsIdempotent(t *testing.T) {
	t.Parallel()
	s := NewStore(t.TempDir(), nil)
	const driver = "codex-tmux"

	for i := 0; i < DefaultBootBenchThreshold; i++ {
		if _, err := s.RecordBootStrike(driver); err != nil {
			t.Fatalf("RecordBootStrike call %d: %v", i+1, err)
		}
	}
	if _, ok := s.Active()[driver]; !ok {
		t.Fatal("precondition: driver not active after threshold strikes")
	}

	if err := s.ClearBootStrike(driver); err != nil {
		t.Fatalf("ClearBootStrike (first): %v", err)
	}
	if active := s.Active(); len(active) != 0 {
		t.Fatalf("Active() = %v after first clear; want empty", active)
	}

	// Second clear on the now-absent entry — must be a no-op, not an error.
	if err := s.ClearBootStrike(driver); err != nil {
		t.Errorf("ClearBootStrike (second, already absent): unexpected error: %v", err)
	}
	if active := s.Active(); len(active) != 0 {
		t.Errorf("Active() = %v after second clear on absent key; want empty", active)
	}
}

// TestAmplify_C426_FullReaccumulationAfterClear: reach threshold (benched) →
// clear (reset) → re-accumulate threshold strikes → benched again.
//
// Guards against an implementation where ClearBootStrike zeroes an in-memory
// counter but leaves the on-disk entry, causing the second round to count from
// a residual state and bench on fewer than threshold consecutive failures.
func TestAmplify_C426_FullReaccumulationAfterClear(t *testing.T) {
	t.Parallel()
	s := NewStore(t.TempDir(), nil)
	const driver = "claude-tmux"

	// Round 1: accumulate to threshold.
	for i := 0; i < DefaultBootBenchThreshold; i++ {
		if _, err := s.RecordBootStrike(driver); err != nil {
			t.Fatalf("round1 RecordBootStrike call %d: %v", i+1, err)
		}
	}
	if _, ok := s.Active()[driver]; !ok {
		t.Fatal("precondition round1: driver not active after threshold strikes")
	}

	if err := s.ClearBootStrike(driver); err != nil {
		t.Fatalf("ClearBootStrike: %v", err)
	}
	if _, ok := s.Active()[driver]; ok {
		t.Fatal("driver still active after ClearBootStrike; round2 precondition broken")
	}

	// Round 2: strikes below threshold must NOT bench.
	for i := 0; i < DefaultBootBenchThreshold-1; i++ {
		benched, err := s.RecordBootStrike(driver)
		if err != nil {
			t.Fatalf("round2 RecordBootStrike call %d: %v", i+1, err)
		}
		if benched {
			t.Errorf("round2 call %d/%d: benched=true before threshold; clear must have reset strike count to zero", i+1, DefaultBootBenchThreshold)
		}
	}
	if _, ok := s.Active()[driver]; ok {
		t.Error("Active() contains driver before round2 threshold is reached")
	}

	// Final strike reaches threshold again → must bench.
	benched, err := s.RecordBootStrike(driver)
	if err != nil {
		t.Fatalf("round2 threshold strike: %v", err)
	}
	if !benched {
		t.Errorf("round2 threshold strike: benched=false; re-accumulation must bench after %d consecutive strikes from a clean slate", DefaultBootBenchThreshold)
	}
	if _, ok := s.Active()[driver]; !ok {
		t.Error("Active() does not contain driver after round2 threshold; re-accumulation must produce an active bench")
	}
}

// TestAmplify_C426_ClearOnlyTargetDriver: with two drivers both having active
// boot benches, ClearBootStrike(A) must NOT affect driver B's bench entry.
//
// Guards against an implementation that clears all BootTimeoutPattern entries
// or iterates the full map instead of keying on the requested driver only.
func TestAmplify_C426_ClearOnlyTargetDriver(t *testing.T) {
	t.Parallel()
	s := NewStore(t.TempDir(), nil)
	const driverA = "codex-tmux"
	const driverB = "agy-tmux"

	now := time.Now()
	for _, drv := range []string{driverA, driverB} {
		if err := s.Bench(Entry{
			Family:       drv,
			Reason:       BootTimeoutPattern,
			BenchedAt:    now,
			BenchedUntil: now.Add(time.Hour),
			Strikes:      DefaultBootBenchThreshold,
		}); err != nil {
			t.Fatalf("Bench(%s): %v", drv, err)
		}
	}
	active := s.Active()
	if _, ok := active[driverA]; !ok {
		t.Fatal("precondition: driverA not active")
	}
	if _, ok := active[driverB]; !ok {
		t.Fatal("precondition: driverB not active")
	}

	if err := s.ClearBootStrike(driverA); err != nil {
		t.Fatalf("ClearBootStrike(%s): %v", driverA, err)
	}

	active = s.Active()
	if _, ok := active[driverA]; ok {
		t.Errorf("Active() still contains %s after ClearBootStrike; should be removed", driverA)
	}
	if _, ok := active[driverB]; !ok {
		t.Errorf("Active() lost %s after ClearBootStrike(%s); multi-driver isolation violated", driverB, driverA)
	}
}
