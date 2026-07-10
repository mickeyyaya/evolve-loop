package prompts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// persona_stopcriterion_dedupe_test.go — cycle-646 Task 3
// (persona-stop-criterion-dedupe): agents/evolve-{scout,builder,auditor}.md
// each carry a structurally-identical "## STOP CRITERION" block (named
// completion gates + banned-post-report patterns) with zero shared wording —
// a token-size refactor, not a behavior change. Scope: extract the shared
// STRUCTURE into one reference doc; each persona file keeps only its
// phase-specific gate list + a pointer. Every existing gate name and banned
// pattern must survive verbatim somewhere under agents/ (the persona file
// itself or the new shared reference doc — the second test searches the
// whole agents/evolve-*.md corpus so it is agnostic to which file the text
// ends up in).
//
// RED today: nothing has been extracted — combined line count is the
// pre-dedupe baseline (751, measured this cycle: 202+275+274).

var personaFiles = []string{"evolve-scout.md", "evolve-builder.md", "evolve-auditor.md"}

func countLines(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return strings.Count(string(data), "\n")
}

// TestPersonaStopCriterionDedupe_CombinedLineCountReduced is the primary
// signal: a no-op (nothing extracted) must fail this test, since it measures
// an actual byte-count reduction rather than the presence of any particular
// string.
func TestPersonaStopCriterionDedupe_CombinedLineCountReduced(t *testing.T) {
	root := acsassert.RepoRoot(t)
	total := 0
	for _, f := range personaFiles {
		total += countLines(t, filepath.Join(root, "agents", f))
	}
	const preDedupeBaseline = 751 // cycle-646 measured: evolve-scout.md(202) + evolve-builder.md(275) + evolve-auditor.md(274)
	if total >= preDedupeBaseline {
		t.Errorf("combined evolve-scout/builder/auditor.md line count = %d, want < %d (pre-dedupe baseline) — extract the shared STOP CRITERION structure into one reference doc per scout-report Task 3", total, preDedupeBaseline)
	}
}

// TestPersonaStopCriterionDedupe_NoGateOrBannedPatternTextLost is the
// negative/scope-boundary guard: a dedupe that drops a gate name or banned
// pattern while shrinking line count must still fail. Searches every
// agents/evolve-*.md file (not just the three personas) so it is agnostic to
// whether the shared reference doc is new or folds into an existing file.
func TestPersonaStopCriterionDedupe_NoGateOrBannedPatternTextLost(t *testing.T) {
	root := acsassert.RepoRoot(t)
	agentsDir := filepath.Join(root, "agents")
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		t.Fatalf("read agents dir: %v", err)
	}
	var combined strings.Builder
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "evolve-") || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(agentsDir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		combined.Write(data)
		combined.WriteString("\n")
	}
	all := combined.String()

	required := []string{
		// scout's 6 named gates (evolve-scout.md:178-183)
		"system-health-complete", "inbox-audit-complete", "backlog-complete",
		"build-plan-written", "research-cache-section", "evals-materialized",
		// builder's 5 named gates (evolve-builder.md:245-250)
		"worktree-verified", "implementation-complete", "self-verify-passed",
		"report-written", "turn-budget-respected",
		// auditor's 3 named gates (evolve-auditor.md:190-193)
		"predicates-run", "verdict-decided",
		// banned-post-report phrase anchors, one per phase (verbatim substrings,
		// ellipsis omitted so the match survives either "…" or "..." spelling)
		"Let me also check", "Let me verify one more thing", "I should also check",
	}
	for _, want := range required {
		if !strings.Contains(all, want) {
			t.Errorf("dedupe must not lose gate-name/banned-pattern text %q from agents/evolve-{scout,builder,auditor}.md or their shared reference doc", want)
		}
	}
}
