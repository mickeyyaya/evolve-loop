package carryover

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/textcap"
)

// ADR-0103 unit 04: the lifecycle's two rune caps are projections of textcap
// — the ONE home the advisor prompt also reads — so neither consumer can
// silently re-implement the rule. Unit 03's own rule tests stay beside them.
func TestTruncateRunes_ProjectsTextcap(t *testing.T) {
	for _, s := range []string{"", "   ", "short", "  padded  ", strings.Repeat("é", 500), "  " + strings.Repeat("é", 501) + "  "} {
		if got, want := TruncateRunes(s, 500), textcap.TruncateRunes(s, 500); got != want {
			t.Errorf("TruncateRunes(%q) = %q, want textcap's %q", s, got, want)
		}
	}
}

func TestCapRunes_ProjectsTextcap(t *testing.T) {
	for _, s := range []string{"", "  padded  ", strings.Repeat("é", 500), strings.Repeat("é", 501)} {
		if got, want := CapRunes(s, 500), textcap.CapRunes(s, 500); got != want {
			t.Errorf("CapRunes(%q) = %q, want textcap's %q", s, got, want)
		}
	}
}
