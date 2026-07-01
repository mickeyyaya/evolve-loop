package bridge

import (
	"regexp"
	"testing"
)

// paneProfileFor must project the per-CLI manifest's quota/rate-limit pattern
// into PaneProfile.ExhaustedRegex (single-source), so the SignalCenter's
// ExhaustionProbe detects a mid-phase wall. Acceptance: the projected pattern
// must match the REAL Gemini incident wording — "Individual quota reached" —
// the exact message that hung the agy router phase (usage-probe pattern already
// matches it; the bug was it was never wired into phase execution).
func TestPaneProfileFor_ProjectsExhaustedRegex(t *testing.T) {
	p := paneProfileFor(tmuxLaunch{name: "agy-tmux", promptMarker: "? for shortcuts"})
	if p.ExhaustedRegex == "" {
		t.Fatal("agy-tmux PaneProfile.ExhaustedRegex is empty — manifest pattern not projected")
	}
	re, err := regexp.Compile(p.ExhaustedRegex)
	if err != nil {
		t.Fatalf("projected ExhaustedRegex %q does not compile: %v", p.ExhaustedRegex, err)
	}
	real := "⚠ Individual quota reached. Please upgrade your subscription to increase your limits. Resets in 52h49m12s."
	if !re.MatchString(real) {
		t.Errorf("projected ExhaustedRegex %q does not match the real Gemini quota message %q", p.ExhaustedRegex, real)
	}
}

// A CLI whose manifest defines no exhaustion pattern (or an unknown driver)
// leaves ExhaustedRegex empty — exhaustion detection off, fail-open (never
// invents a wall for a driver that cannot report one).
func TestPaneProfileFor_UnknownDriver_NoExhaustedRegex(t *testing.T) {
	p := paneProfileFor(tmuxLaunch{name: "itest-tmux", promptMarker: "> "})
	if p.ExhaustedRegex != "" {
		t.Errorf("unknown driver ExhaustedRegex=%q, want empty (fail-open)", p.ExhaustedRegex)
	}
}
