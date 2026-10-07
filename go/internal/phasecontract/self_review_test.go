package phasecontract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSelfReviewRecorded_NeedsAVisibleLevelTwoHeadingWithAScoresLine(t *testing.T) {
	const scores = "- Scores: composite 0.91, correctness 0.9, security 1.0, performance 0.9, maintainability 0.85\n"
	for _, tc := range []struct {
		name   string
		report string
		want   bool
	}{
		{"the heading and its scores", "# Build\n## Self-Review\n- Skill: code-review-simplify, tier lightweight\n" + scores, true},
		{"the lower-case heading", "# Build\n## Self-review\n" + scores, true},
		{"bold field keys", "# Build\n## Self-Review\n- **Scores:** composite 0.9\n", true},
		{"the template's empty heading", "## Self-Review\n<!-- Step 5.6 on a code cycle: the code-review-simplify Self-review block (tier, scores, applied, declined, Go tests, re-verified). Omit on a document cycle. -->\n\n## E2E Verification\n", false},
		{"a heading without a scores line", "# Build\n## Self-Review\n- Applied: none\n", false},
		{"an empty scores line", "# Build\n## Self-Review\n- Scores:\n", false},
		{"a prose mention", "## Changes\nI skipped the `## Self-Review` section.\n" + scores, false},
		{"a fenced example", "```markdown\n## Self-Review\n" + scores + "```\n", false},
		{"a level-three heading", "### Self-Review notes\n" + scores, false},
		{"scores under the next section", "## Self-Review\n- Applied: none\n## Changes\n" + scores, false},
		{"no report", "", false},
		{"the unfilled template's placeholder scores", "## Self-Review\n- Scores: composite <0.NN>, correctness <0.NN>, security <0.NN>, performance <0.NN>, maintainability <0.NN>\n", false},
		{"a whole-number score", "## Self-Review\n- Scores: composite 1.0\n", true},
		{"a re-run that repeats the scores line", "## Self-Review\n" + scores + scores, true},
		{"a re-run that repeats the section", "## Self-Review\n" + scores + "## Changes\n- a.go\n## Self-Review\n" + scores, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := SelfReviewRecorded(tc.report); got != tc.want {
				t.Errorf("SelfReviewRecorded(%q) = %v, want %v", tc.report, got, tc.want)
			}
		})
	}
}

func TestSelfReview_TheWriterPersonasAndTheSkillDeclareTheHeadingTheRunnerChecks(t *testing.T) {
	for path, body := range selfReviewTexts(t) {
		for _, want := range []string{SelfReview.Canonical, "- Scores:"} {
			if !strings.Contains(body, want) {
				t.Errorf("%s does not declare %q, which the runner's self-review check reads", path, want)
			}
		}
	}
}

func TestSelfReview_TheHookNeverEditsEarlierPhaseOwnedFiles(t *testing.T) {
	for path, body := range selfReviewTexts(t) {
		for _, want := range []string{"git status --porcelain", "Declined: earlier-phase-owned"} {
			if !strings.Contains(body, want) {
				t.Errorf("%s does not carry the earlier-phase-owned rule (%q): the self-review would let a writer edit files an earlier phase wrote", path, want)
			}
		}
	}
	for _, path := range []string{"skills/code-review-simplify/SKILL.md", "skills/code-review-simplify/COMPACT.md", "agents/evolve-builder.md"} {
		for _, example := range []string{"testFiles", "go/acs/", "bug-reproduction"} {
			if !strings.Contains(selfReviewTexts(t)[path], example) {
				t.Errorf("%s does not name %q among the earlier-phase-owned examples", path, example)
			}
		}
	}
}

func selfReviewTexts(t *testing.T) map[string]string {
	t.Helper()
	agents := agentsDir(t)
	return map[string]string{
		"skills/code-review-simplify/SKILL.md":   readSkillFile(t, agents, "SKILL.md"),
		"skills/code-review-simplify/COMPACT.md": readSkillFile(t, agents, "COMPACT.md"),
		"agents/evolve-builder.md":               producerUnion(t, agents, []string{"evolve-builder"}),
		"agents/evolve-tdd-engineer.md":          producerUnion(t, agents, []string{"evolve-tdd-engineer"}),
	}
}

func readSkillFile(t *testing.T, agentsDir, file string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(agentsDir, "..", "skills", "code-review-simplify", file))
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}
