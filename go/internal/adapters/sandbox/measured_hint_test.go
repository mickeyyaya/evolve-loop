package sandbox

import "testing"

func TestShouldWrap_MeasurementOverridesSessionHint(t *testing.T) {
	for _, goos := range []string{"darwin", "linux"} {
		t.Run(goos, func(t *testing.T) {
			probe := ProbeResult{OS: goos, Available: true, CapabilityChecked: true, Capable: true}
			if wrap, reason := ShouldWrap(true, probe); !wrap {
				t.Fatalf("measured working sandbox was rejected by a session hint: %s", reason)
			}
			probe.Capable = false
			if wrap, _ := ShouldWrap(true, probe); wrap {
				t.Fatal("measured failure must refuse wrapping")
			}
			probe.CapabilityChecked = false
			probe.Capable = true
			if wrap, _ := ShouldWrap(true, probe); wrap {
				t.Fatal("an unchecked capability must not override a nested hint")
			}
			probe.CapabilityChecked = true
			probe.Available = false
			if wrap, _ := ShouldWrap(true, probe); wrap {
				t.Fatal("measurement cannot supply a missing sandbox binary")
			}
			probe.Available = true
			probe.OS = "unsupported"
			if wrap, _ := ShouldWrap(true, probe); wrap {
				t.Fatal("measurement cannot supply an unsupported implementation")
			}
		})
	}
}
