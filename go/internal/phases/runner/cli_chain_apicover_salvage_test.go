package runner

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
