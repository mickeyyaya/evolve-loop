package runner

// cli_chain_apicover_salvage_test.go — apicover Phase-5 naming coverage for the
// salvaged cycle-943 export (false-RED salvage, post-v22.4.2): NAMES + EXERCISES
// FormatSkillOverlayLog. Behavioral (Rule 9): pins the exact observability line
// shape operators/graders grep for, including the empty-set rendering that
// distinguishes "no overlay resolved" from "the line never ran".
import (
	"testing"
)

func TestFormatSkillOverlayLog_LineShapeAndEmptySet(t *testing.T) {
	t.Parallel()
	got := FormatSkillOverlayLog("audit", []string{"fable"}, "deep")
	if got != "[runner] phase=audit skill-overlays=[fable] (tier=deep)" {
		t.Errorf("overlay log line drifted: %q", got)
	}
	if got := FormatSkillOverlayLog("scout", nil, "balanced"); got != "[runner] phase=scout skill-overlays=[] (tier=balanced)" {
		t.Errorf("empty set must render explicitly as []: %q", got)
	}
}
