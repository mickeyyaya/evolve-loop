package sandbox

import "testing"

func TestShouldWrap_AnUnsupportedOSGivesTheOneReason(t *testing.T) {
	t.Parallel()
	wrap, reason := ShouldWrap(false, ProbeResult{OS: "plan9"})
	if wrap || reason != UnsupportedOSReason("plan9") {
		t.Fatalf("ShouldWrap(plan9) = (%v, %q), want the shared reason", wrap, reason)
	}
}
