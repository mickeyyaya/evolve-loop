package prompts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var personaFiles = []string{"evolve-scout.md", "evolve-builder.md", "evolve-auditor.md"}

func countLines(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return strings.Count(string(data), "\n")
}

// Despite the name, this is the standing combined line budget of the three personas;
// core.personaBudgetFailures runs this package in-lane when a persona doc changes.
func TestPersonaStopCriterionDedupe_CombinedLineCountReduced(t *testing.T) {
	root := acsassert.RepoRoot(t)
	total := 0
	for _, f := range personaFiles {
		total += countLines(t, filepath.Join(root, "agents", f))
	}
	const preDedupeBaseline = 751
	if total >= preDedupeBaseline {
		t.Errorf("combined evolve-scout/builder/auditor.md line count = %d, want < %d (pre-dedupe baseline) — extract the shared STOP CRITERION structure into one reference doc per scout-report Task 3", total, preDedupeBaseline)
	}
}

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
		// scout gates
		"system-health-complete", "inbox-audit-complete", "backlog-complete",
		"build-plan-written", "research-cache-section", "evals-materialized",
		// builder gates
		"worktree-verified", "implementation-complete", "self-verify-passed",
		"report-written", "turn-budget-respected",
		// auditor gates
		"predicates-run", "verdict-decided",
		// banned post-report phrases, without the ellipsis so "…" and "..." both match
		"Let me also check", "Let me verify one more thing", "I should also check",
	}
	for _, want := range required {
		if !strings.Contains(all, want) {
			t.Errorf("dedupe must not lose gate-name/banned-pattern text %q from agents/evolve-{scout,builder,auditor}.md or their shared reference doc", want)
		}
	}
}
