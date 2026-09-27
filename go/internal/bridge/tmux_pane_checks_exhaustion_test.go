package bridge

import (
	"regexp"
	"testing"
)

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

func TestPaneProfileFor_UnknownDriver_NoExhaustedRegex(t *testing.T) {
	p := paneProfileFor(tmuxLaunch{name: "itest-tmux", promptMarker: "> "})
	if p.ExhaustedRegex != "" {
		t.Errorf("unknown driver ExhaustedRegex=%q, want empty (fail-open)", p.ExhaustedRegex)
	}
}
