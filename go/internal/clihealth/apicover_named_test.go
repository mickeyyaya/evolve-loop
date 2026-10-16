package clihealth

import (
	"testing"
	"time"
)

func TestClearBootStrike_Named(t *testing.T) {
	t.Parallel()
	s := NewStore(t.TempDir(), nil)
	if err := s.ClearBootStrike("absent-driver"); err != nil {
		t.Errorf("ClearBootStrike on absent driver: %v", err)
	}
}

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
	if got := CooldownForStrikes(100); got != MaxCooldown {
		t.Errorf("CooldownForStrikes(100)=%v, want MaxCooldown=%v", got, MaxCooldown)
	}
}
