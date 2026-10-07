package guards

import "testing"

func TestProtectedSurface_TheQualityIndexAndTheCodeReviewPhaseAreControlPlane(t *testing.T) {
	for _, p := range []string{
		"skills/quality-index/SKILL.md",
		"skills/quality-index/COMPACT.md",
		"skills/architecture-review/SKILL.md",
		"go/internal/qualityindex/grammar.go",
		"go/internal/codereview/record.go",
		"agents/evolve-code-reviewer.md",
		".evolve/phases/code-review/phase.json",
		".evolve/profiles/code-reviewer.json",
		"go/internal/router/code_review_pin_test.go",
	} {
		if !IsProtectedSurface(p) {
			t.Errorf("%s must be protected: a lane that rewrites it rewrites what \"qualified\" means for every cycle", p)
		}
	}
	for _, p := range []string{"go/internal/router/router.go", "docs/architecture/phase-registry.json", "agents/evolve-code-reviewer-notes.md"} {
		if IsProtectedSurface(p) {
			t.Errorf("%s must stay ordinary cycle territory: the rows are scoped to the review's own files", p)
		}
	}
}
