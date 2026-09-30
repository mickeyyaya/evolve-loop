//go:build acs

package cycle391

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC391_001_LineCountReduced(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-intent.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read agents/evolve-intent.md: %v", err)
	}
	lineCount := strings.Count(string(raw), "\n")
	const maxLines = 219
	if lineCount > maxLines {
		t.Errorf("RED: agents/evolve-intent.md has %d lines — must be ≤%d.\n"+
			"Builder must delete the C69–C73 calibration table (~lines 116-126),\n"+
			"the v9.0.1 design-correction paragraph (~lines 100-101), and the\n"+
			"'No web research deadline' subsection (~lines 112-114).\n"+
			"Baseline: 234 lines. Target: ≤219.",
			lineCount, maxLines)
	}
}

func TestC391_002_ByteCountReduced(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-intent.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read agents/evolve-intent.md: %v", err)
	}
	byteCount := len(raw)
	const maxBytes = 12768
	if byteCount > maxBytes {
		t.Errorf("RED: agents/evolve-intent.md has %d bytes — must be <%d (baseline 12769).\n"+
			"A reflow-only edit that preserves all content cannot satisfy this.\n"+
			"Only genuine removal of the 3 inert blocks can reduce byte count below baseline.",
			byteCount, maxBytes+1)
	}
}

// acs-predicate: config-check
func TestC391_003_AllBehavioralAnchorsPresent(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-intent.md")

	requiredHeaders := []string{
		"## Inputs",
		"## Your single output",
		"## The Ask-when-Needed (AwN) classifier",
		"## Turn budget",
		"## STOP CRITERION",
		"## The mandatory ≥1 challenged_premise rule",
		"## What you MUST NOT do",
		"## Length budget",
		"## Re-run behavior",
		"## Output contract (INTENT_MODE)",
		"## Composition",
		"## Reference",
		"## Reflection Authoring",
	}
	for _, header := range requiredHeaders {
		if !acsassert.FileContains(t, path, header) {
			t.Errorf("section %q was dropped — Builder must only remove inert justification prose, not real sections",
				header)
		}
	}

	requiredAnchors := []string{
		"awn_class",
		"IMKI",
		"IMR",
		"IwE",
		"IBTC",
		"CLEAR",
		"challenged_premises",
		"Emergency Exit",
		"HARD STOP",
		"INTENT_MODE",
		"intent-delta.md",
		"gate_intent_to_research",
	}
	for _, anchor := range requiredAnchors {
		if !acsassert.FileContains(t, path, anchor) {
			t.Errorf("behavioral anchor %q was removed — Builder must preserve all behavioral contracts",
				anchor)
		}
	}
}

func TestC391_004_ArchaeologyMarkersAbsent(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-intent.md")

	archaeologyMarkers := []struct {
		token string
		block string
	}{
		{"C69", "Calibration basis (C69–C73 measurement) table — lines ~116-126"},
		{"cycle 11 measured", "v9.0.1 design-correction paragraph — lines ~100-101"},
		{"No web research deadline", "### No web research deadline subsection — lines ~112-114"},
	}
	for _, m := range archaeologyMarkers {
		if !acsassert.FileNotContains(t, path, m.token) {
			t.Errorf("RED: archaeology marker %q still present in agents/evolve-intent.md — "+
				"Builder must delete the %s", m.token, m.block)
		}
	}
}

func TestC391_005_OnlyIntentFileChanged(t *testing.T) {
	root := acsassert.RepoRoot(t)
	stdout, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "--name-only", "HEAD~1..HEAD")
	if err != nil || code != 0 {
		t.Fatalf("git diff HEAD~1..HEAD failed (exit=%d): %v", code, err)
	}
	changed := strings.Fields(strings.TrimSpace(stdout))
	const expected = "agents/evolve-intent.md"
	if len(changed) != 1 || changed[0] != expected {
		t.Errorf("RED: expected exactly 1 file changed (%s), got %v.\n"+
			"Before Builder's commit, the worktree HEAD shows cycle-389's change (agents/evolve-tester.md).\n"+
			"After Builder commits, this must show ONLY agents/evolve-intent.md — no control-plane edits.",
			expected, changed)
	}
}

// acs-predicate: config-check
func TestC391_006_FrontmatterAndReflectionTailPresent(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-intent.md")

	if !acsassert.FileContains(t, path, "name: evolve-intent") {
		t.Errorf("frontmatter field 'name: evolve-intent' was removed — Builder must not touch frontmatter")
	}
	if !acsassert.FileContains(t, path, "reflection-authoring-step.md") {
		t.Errorf("Reflection Authoring link to reflection-authoring-step.md was removed — Builder must preserve the tail")
	}

	_, _, gitCode, gitErr := acsassert.SubprocessOutput(
		"git", "-C", root, "ls-files", "--error-unmatch", filepath.Join("agents", "evolve-intent.md"))
	if gitErr != nil || gitCode != 0 {
		t.Errorf("agents/evolve-intent.md is not git-tracked (exit=%d: %v) — must not be moved or gitignored",
			gitCode, gitErr)
	}
}
