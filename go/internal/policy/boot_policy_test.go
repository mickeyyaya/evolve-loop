package policy

// boot_policy_test.go — the boot.binary_refresh knob (binary-lag self-heal,
// docs/chronicle/2026-08-binary-lag.md). Closed vocabulary {auto, off};
// absent block, empty word, and unknown words all resolve to "auto": the
// self-heal is integrity posture, so a typo must not silently disable it
// (the fleet-landing unknown-word precedent, inverted for a default-on dial).

import "testing"

func TestBootBinaryRefresh_Vocabulary(t *testing.T) {
	cases := []struct {
		name string
		p    Policy
		want string
	}{
		{"absent block defaults auto", Policy{}, "auto"},
		{"empty word defaults auto", Policy{Boot: &BootPolicy{}}, "auto"},
		{"off honored", Policy{Boot: &BootPolicy{BinaryRefresh: "off"}}, "off"},
		{"auto explicit", Policy{Boot: &BootPolicy{BinaryRefresh: "auto"}}, "auto"},
		{"unknown word fails safe to auto", Policy{Boot: &BootPolicy{BinaryRefresh: "shadwo"}}, "auto"},
	}
	for _, c := range cases {
		if got := c.p.BootBinaryRefresh(); got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}
