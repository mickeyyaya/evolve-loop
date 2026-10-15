package channel

import "testing"

func TestResolveStage(t *testing.T) {
	t.Parallel()
	cases := []struct {
		raw  string
		want string
	}{
		{"", "shadow"},
		{"shadow", "shadow"},
		{"off", "off"},
		{"0", "off"},
		{"enforce", "enforce"},
		{"SHADOW", "shadow"},
		{"OFF", "off"},
		{"ENFORCE", "enforce"},
		{"  enforce  ", "enforce"},
		{"typo", "off"},
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
