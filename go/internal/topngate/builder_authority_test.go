package topngate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// repoRoot resolves the repository root from this test's own location:
// <root>/go/internal/topngate. Mirrors internal/triagecap/floors_test.go's
// repoRoot helper.
func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(wd)))
}

// TestBuilderPromptNamesTopNAsSoleTaskAuthority is the AC-2 regression test:
// the Builder's own instructions must name triage-report.md's ## top_n as its
// sole task authority, demoting scout-report.md to background context only.
//
// Before this fix, agents/evolve-builder.md instructed the opposite: "Read
// task from workspace/scout-report.md" and "the `## Task: <slug>` line ...
// MUST be ... copied verbatim from the scout-report's `## Selected Tasks`" —
// the root cause of the cycle-640 wrong-task build
// (builder-task-binding-topn-gate, 7th recurrence: cycles 282, 310, 522, 575,
// 577, 599, 640).
func TestBuilderPromptNamesTopNAsSoleTaskAuthority(t *testing.T) {
	path := filepath.Join(repoRoot(t), "agents", "evolve-builder.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	body := string(data)
	lower := strings.ToLower(body)

	if !strings.Contains(body, "triage-report.md") {
		t.Error("evolve-builder.md must reference triage-report.md as the task source")
	}
	if !strings.Contains(body, "top_n") {
		t.Error("evolve-builder.md must reference the ## top_n section")
	}
	if !strings.Contains(lower, "sole") && !strings.Contains(lower, "authoritative") && !strings.Contains(lower, "exclusively") {
		t.Error("evolve-builder.md must state that triage top_n is the SOLE/authoritative/exclusive task source, not merely mentioned alongside scout-report.md")
	}
	if !strings.Contains(lower, "background") && !strings.Contains(lower, "context only") && !strings.Contains(lower, "never task selection") {
		t.Error("evolve-builder.md must explicitly demote scout-report.md to background/context-only status, never task selection")
	}
}
