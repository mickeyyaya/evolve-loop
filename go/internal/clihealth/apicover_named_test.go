package clihealth

import (
	"testing"
	"time"
)

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
