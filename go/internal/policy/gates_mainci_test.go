package policy

import (
	"testing"
	"time"
)

func TestPolicy_MainCIPollMinutesDefaultsAndOverrides(t *testing.T) {
	t.Parallel()
	if got := (Policy{}).GatesConfig().MainCIPoll; got <= 0 || got > time.Hour {
		t.Errorf("default MainCIPoll = %v, want a positive interval of at most an hour", got)
	}
	for minutes, want := range map[int]time.Duration{7: 7 * time.Minute, 0: (Policy{}).GatesConfig().MainCIPoll, -3: (Policy{}).GatesConfig().MainCIPoll} {
		if got := (Policy{Gates: &GatesPolicy{MainCIPollMinutes: minutes}}).GatesConfig().MainCIPoll; got != want {
			t.Errorf("MainCIPollMinutes %d: MainCIPoll = %v, want %v", minutes, got, want)
		}
	}
}
