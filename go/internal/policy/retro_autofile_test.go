package policy

import "testing"

// TestRetroAutofileDefaultWeight_DefaultAndOverride encodes AC3's "weight
// defaults from policy.json" half for the retro→inbox autofiler
// (retro-preventive-actions-autofile-inbox, cycle 657): the default weight for
// auto-filed preventive-action items is sourced from policy, not a Go literal
// (feedback_phase_settings_from_config_not_code), with a safe built-in fallback
// when the policy block is absent.
//
// RED until the Builder adds a RetroAutofile policy block + the
// RetroAutofileDefaultWeight() accessor. DO NOT modify this test — implement
// production code to green it.
func TestRetroAutofileDefaultWeight_DefaultAndOverride(t *testing.T) {
	// Absent block ⇒ the compiled-in safe default (0.75, per the item).
	var zero Policy
	if got := zero.RetroAutofileDefaultWeight(); got != 0.75 {
		t.Errorf("RetroAutofileDefaultWeight() with no policy block = %v, want 0.75 default", got)
	}

	// A present block overrides the default.
	p := Policy{RetroAutofile: &RetroAutofilePolicy{DefaultWeight: 0.6}}
	if got := p.RetroAutofileDefaultWeight(); got != 0.6 {
		t.Errorf("RetroAutofileDefaultWeight() with block = %v, want 0.6 override", got)
	}
}
