package channel

import "testing"

// TestResolveStage covers all branches of ResolveStage (ADR-0045 I6).
func TestResolveStage(t *testing.T) {
	t.Parallel()
	cases := []struct {
		raw  string
		want string
	}{
		{"", "shadow"},         // unset → shadow default
		{"shadow", "shadow"},   // canonical
		{"off", "off"},         // canonical
		{"enforce", "enforce"}, // canonical
		{"SHADOW", "shadow"},   // case-insensitive
		{"OFF", "off"},
		{"ENFORCE", "enforce"},
		{"  enforce  ", "enforce"}, // trim whitespace
		{"typo", "off"},            // unknown → off (fail-safe)
		{"yes", "off"},
		{"1", "off"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.raw+"->"+tc.want, func(t *testing.T) {
			if got := ResolveStage(tc.raw); got != tc.want {
				t.Errorf("ResolveStage(%q)=%q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

// TestEnabled_FoldsFlagIntoStage — ADR-0045 I6: the deprecated EVOLVE_CHANNEL
// flag is honored one release (with a deprecation signal); when unset the
// channel is implied by the EVOLVE_PHASE_RECOVERY stage, and shadow/off stay
// byte-identical (channel off).
func TestEnabled_FoldsFlagIntoStage(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name          string
		stage         string
		explicit      string
		wantOn        bool
		wantDeprecate bool
	}{
		{"unset_shadow_off_byteidentical", "shadow", "", false, false},
		{"unset_off_off", "off", "", false, false},
		{"unset_enforce_implies_on", "enforce", "", true, false},
		{"explicit_1_honored_with_warn", "shadow", "1", true, true},
		{"explicit_0_honored_with_warn", "enforce", "0", false, true},
		{"explicit_garbage_off_with_warn", "enforce", "yes", false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			on, dep := Enabled(c.stage, c.explicit)
			if on != c.wantOn || dep != c.wantDeprecate {
				t.Errorf("Enabled(%q,%q) = (on=%v dep=%v), want (on=%v dep=%v)",
					c.stage, c.explicit, on, dep, c.wantOn, c.wantDeprecate)
			}
		})
	}
}
