package policy

import (
	"testing"
	"time"
)

func TestPolicy_BaseRedRerunDefault(t *testing.T) {
	t.Parallel()
	if got := (Policy{}).GatesConfig().BaseRedRerun; got != 300*time.Second {
		t.Errorf("default BaseRedRerun = %v, want 5m0s", got)
	}
	if got := (Policy{Gates: &GatesPolicy{}}).GatesConfig().BaseRedRerun; got != 300*time.Second {
		t.Errorf("BaseRedRerun with an empty gates block = %v, want 5m0s", got)
	}
}

func TestPolicy_BaseRedRerunSecondsOverrideAndIgnoreANonPositiveValue(t *testing.T) {
	t.Parallel()
	for secs, want := range map[int]time.Duration{90: 90 * time.Second, 0: 300 * time.Second, -5: 300 * time.Second} {
		if got := (Policy{Gates: &GatesPolicy{BaseRedRerunSeconds: secs}}).GatesConfig().BaseRedRerun; got != want {
			t.Errorf("BaseRedRerunSeconds %d: BaseRedRerun = %v, want %v", secs, got, want)
		}
	}
}
