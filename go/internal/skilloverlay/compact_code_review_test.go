package skilloverlay

import (
	"strings"
	"testing"
)

func TestMaterialize_TheCodeReviewSimplifyOverlayIsItsCompactProjection(t *testing.T) {
	prefix, missing := Materialize("../../../skills", []string{"code-review-simplify"})
	if len(missing) != 0 {
		t.Fatalf("missing = %v", missing)
	}
	if !strings.Contains(prefix, "COMPACT projection") || strings.Contains(prefix, "## Output Schema") {
		t.Errorf("the preloaded code-review-simplify body must be COMPACT.md, not the full SKILL.md; got %d bytes", len(prefix))
	}
	if len(prefix) > 4096 {
		t.Errorf("the preloaded code-review-simplify block is %d bytes; the compact projection stays near 2.5 KB", len(prefix))
	}
}
