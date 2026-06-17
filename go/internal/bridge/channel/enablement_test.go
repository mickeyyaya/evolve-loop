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

// TestEnabled_UsesStageOnly — ADR-0045 I6 (EVOLVE_CHANNEL removed in v19.x):
// the channel is implied solely by the EVOLVE_PHASE_RECOVERY stage.
// shadow/off → byte-identical (channel off); enforce → on.
func TestEnabled_UsesStageOnly(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		stage  string
		wantOn bool
	}{
		{"shadow_off_byteidentical", "shadow", false},
		{"off_off", "off", false},
		{"enforce_implies_on", "enforce", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Enabled(c.stage); got != c.wantOn {
				t.Errorf("Enabled(%q) = %v, want %v", c.stage, got, c.wantOn)
			}
		})
	}
}
