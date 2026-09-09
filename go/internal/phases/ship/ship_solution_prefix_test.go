package ship

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestDefaultCommitMessage_SolutionPrefix — ADR-0099 slice 2: a document cycle
// lands under the `solution(<slug>)` prefix so the commit vocabulary names the
// deliverable it carries; code cycles keep the legacy message byte-identical.
func TestDefaultCommitMessage_SolutionPrefix(t *testing.T) {
	ws := t.TempDir()
	for name, body := range map[string]string{
		"triage-report.md":     "<!-- challenge-token: x -->\n# Triage\n\ncycle_size_estimate: medium\ndeliverable_kind: document\n\n## top_n\n- netflix-margin: x\n",
		"triage-decision.json": `{"top_n":[{"id":"netflix-margin"}]}`,
	} {
		if err := os.WriteFile(filepath.Join(ws, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if got := defaultCommitMessage(core.PhaseRequest{Cycle: 9, Workspace: ws}); got != "solution(netflix-margin): evolve-cycle 9" {
		t.Errorf("document cycle message = %q", got)
	}
	if got := defaultCommitMessage(core.PhaseRequest{Cycle: 9, GoalHash: "h", Workspace: t.TempDir()}); got != "evolve-cycle 9: goal=h" {
		t.Errorf("code cycle message must be unchanged: %q", got)
	}
}

// TestDefaultCommitMessage_SolutionPrefix_MultiSlug: the scope carries ONE
// slug (the prefix grammar admits [a-z0-9-] only); the rest ride in the subject.
func TestDefaultCommitMessage_SolutionPrefix_MultiSlug(t *testing.T) {
	ws := t.TempDir()
	for name, body := range map[string]string{
		"triage-report.md":     "<!-- challenge-token: x -->\n# Triage\n\ncycle_size_estimate: medium\ndeliverable_kind: document\n\n## top_n\n- a: x\n- b: y\n",
		"triage-decision.json": `{"top_n":[{"id":"netflix-margin"},{"id":"india-bundle"}]}`,
	} {
		if err := os.WriteFile(filepath.Join(ws, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if got := defaultCommitMessage(core.PhaseRequest{Cycle: 3, Workspace: ws}); got != "solution(netflix-margin): evolve-cycle 3 (also india-bundle)" {
		t.Errorf("multi-slug message = %q", got)
	}
}
