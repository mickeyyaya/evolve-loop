package prompts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const recapRule = "Write the counts of an earlier review round in words, never as `N SEVERITY` pairs."

var recapRuleCarriers = []string{
	"agents/evolve-diff-code-reviewer.md",
	"agents/evolve-diff-go-reviewer.md",
	"agents/evolve-diff-architecture-reviewer.md",
	"agents/evolve-adversarial-review.md",
	"skills/docs-review/SKILL.md",
}

func TestDiffReviewerPersonas_HoldTheRecapRule(t *testing.T) {
	root := acsassert.RepoRoot(t)
	for _, rel := range recapRuleCarriers {
		raw, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(raw), recapRule) {
			t.Errorf("%s lacks the recap rule %q; a recap with N SEVERITY pairs then fails a clean review", rel, recapRule)
		}
	}
}

func TestDiffReviewerPersonas_TheDocsReviewPersonaPointsAtTheSkillThatHoldsTheRule(t *testing.T) {
	root := acsassert.RepoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-diff-docs-review.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "skills/docs-review/SKILL.md") {
		t.Error("the docs-review persona no longer names skills/docs-review/SKILL.md, the home of the recap rule")
	}
}
