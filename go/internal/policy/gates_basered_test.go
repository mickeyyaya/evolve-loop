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

func TestPolicy_MainRedStopIsShadowUntilThePolicyNamesAStage(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		p    Policy
		want string
	}{
		"no gates block":       {Policy{}, "shadow"},
		"an empty gates block": {Policy{Gates: &GatesPolicy{}}, "shadow"},
		"the policy enforces":  {Policy{Gates: &GatesPolicy{MainRedStop: "enforce"}}, "enforce"},
	} {
		if got := tc.p.GatesConfig().MainRedStop; got != tc.want {
			t.Errorf("%s: MainRedStop = %q, want %q", name, got, tc.want)
		}
	}
}

func TestGatesConfig_MainRedStopEnforcedOnlyAtTheEnforceStage(t *testing.T) {
	t.Parallel()
	for stage, want := range map[string]bool{"enforce": true, "shadow": false, "": false, "off": false} {
		if got := (GatesConfig{MainRedStop: stage}).MainRedStopEnforced(); got != want {
			t.Errorf("stage %q: MainRedStopEnforced = %v, want %v", stage, got, want)
		}
	}
	if (Policy{}).GatesConfig().MainRedStopEnforced() {
		t.Error("the default stage enforces the stop, want shadow")
	}
}
