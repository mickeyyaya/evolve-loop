package policy

import "testing"

func TestRetroAutofileDefaultWeight_DefaultAndOverride(t *testing.T) {
	var zero Policy
	if got := zero.RetroAutofileDefaultWeight(); got != 0.75 {
		t.Errorf("RetroAutofileDefaultWeight() with no policy block = %v, want 0.75 default", got)
	}

	p := Policy{RetroAutofile: &RetroAutofilePolicy{DefaultWeight: 0.6}}
	if got := p.RetroAutofileDefaultWeight(); got != 0.6 {
		t.Errorf("RetroAutofileDefaultWeight() with block = %v, want 0.6 override", got)
	}
}
