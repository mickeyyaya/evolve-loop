package clihealth

import (
	"testing"
	"time"
)

// TestClearBootStrike_Named names the exported ClearBootStrike method so
// apicover -enforce confirms its identifier is tracked. Verifies it is callable
// and returns nil on an absent driver (the no-op path).
func TestClearBootStrike_Named(t *testing.T) {
	t.Parallel()
	s := NewStore(t.TempDir(), nil)
	if err := s.ClearBootStrike("absent-driver"); err != nil {
		t.Errorf("ClearBootStrike on absent driver: %v", err)
	}
}

// TestCooldownBounds_DefaultAndMax pins DefaultCooldown and MaxCooldown to their
// real consumer, CooldownForStrikes: the first strike yields exactly
// DefaultCooldown, and a high strike count saturates at exactly MaxCooldown. If
// either const drifted, the bench-window math the runner/loop share would change.
func TestCooldownBounds_DefaultAndMax(t *testing.T) {
	t.Parallel()
	if DefaultCooldown != 30*time.Minute {
		t.Errorf("DefaultCooldown=%v, want 30m", DefaultCooldown)
	}
	if MaxCooldown != 4*time.Hour {
		t.Errorf("MaxCooldown=%v, want 4h", MaxCooldown)
	}
	if got := CooldownForStrikes(1); got != DefaultCooldown {
		t.Errorf("CooldownForStrikes(1)=%v, want DefaultCooldown=%v", got, DefaultCooldown)
	}
	// Strikes far beyond the doubling range must saturate at the cap, not exceed it.
	if got := CooldownForStrikes(100); got != MaxCooldown {
		t.Errorf("CooldownForStrikes(100)=%v, want MaxCooldown=%v", got, MaxCooldown)
	}
}
