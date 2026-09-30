//go:build acs

package cycle389

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC389_001_LineCountReduced(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-tester.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read agents/evolve-tester.md: %v", err)
	}
	lineCount := strings.Count(string(raw), "\n")
	const maxLines = 185
	if lineCount > maxLines {
		t.Errorf("RED: agents/evolve-tester.md has %d lines — must be ≤%d.\n"+
			"Builder must collapse the triplicated worktree-resolution boilerplate\n"+
			"(lines 95-96/106-115), tighten the adversarial-mindset restatement\n"+
			"(lines 137-146), and condense low-density preamble (lines 19-22/127).\n"+
			"Baseline: 193 lines. Target: ≤185.",
			lineCount, maxLines)
	}
}

func TestC389_002_WordCountReduced(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-tester.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read agents/evolve-tester.md: %v", err)
	}
	wordCount := len(strings.Fields(string(raw)))
	const maxWords = 1280
	if wordCount > maxWords {
		t.Errorf("RED: agents/evolve-tester.md has %d words — must be ≤%d.\n"+
			"A reflow-only edit that preserves all words cannot satisfy this.\n"+
			"Baseline: 1320 words. Target: ≤1280.",
			wordCount, maxWords)
	}
}

// acs-predicate: config-check
func TestC389_003_AllSectionHeadersPresent(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-tester.md")
	required := []string{
		"## Inputs",
		"## What you produce",
		"## Banned patterns",
		"## How to translate an AC into a predicate",
		"## When verification is impossible",
		"## What you are NOT allowed to do",
		"## Adversarial mindset",
		"## Reference Index",
		"## Output Artifact",
		"## Reflection Authoring",
	}
	for _, header := range required {
		if !acsassert.FileContains(t, path, header) {
			t.Errorf("section %q was dropped — Builder must only remove redundant prose, not real sections",
				header)
		}
	}
}

// acs-predicate: config-check
func TestC389_004_BannedPatternsAndMetadataIntact(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-tester.md")
	bannedPatterns := []string{
		"presence ≠ execution",
		`echo "PASS"; exit 0`,
		"hermetic-determinism",
		"sleep",
		"acs-output/",
		"Lack required metadata",
	}
	for _, token := range bannedPatterns {
		if !acsassert.FileContains(t, path, token) {
			t.Errorf("banned-pattern item %q was removed — Builder must not touch the 6-item Banned patterns list",
				token)
		}
	}
	metadataFields := []string{
		"# AC-ID:",
		"# Acceptance-of:",
	}
	for _, field := range metadataFields {
		if !acsassert.FileContains(t, path, field) {
			t.Errorf("metadata header field %q was removed — Builder must preserve the predicate metadata-header spec",
				field)
		}
	}
}

func TestC389_005_OnlyTesterFileChanged(t *testing.T) {
	root := acsassert.RepoRoot(t)
	stdout, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "--name-only", "HEAD~1..HEAD")
	if err != nil || code != 0 {
		t.Fatalf("git diff HEAD~1..HEAD failed (exit=%d): %v", code, err)
	}
	changed := strings.Fields(strings.TrimSpace(stdout))
	const expected = "agents/evolve-tester.md"
	if len(changed) != 1 || changed[0] != expected {
		t.Errorf("RED: expected exactly 1 file changed (%s), got %v.\n"+
			"Before Builder's commit, the worktree HEAD shows cycle-388's change (agents/evolve-builder.md).\n"+
			"After Builder commits, this must show ONLY agents/evolve-tester.md — no control-plane edits.",
			expected, changed)
	}
}

// acs-predicate: config-check
func TestC389_006_FrontmatterAndReflectionTailPresent(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-tester.md")
	if !acsassert.FileContains(t, path, "name: evolve-tester") {
		t.Errorf("frontmatter field 'name: evolve-tester' was removed — Builder must not touch frontmatter")
	}
	if !acsassert.FileContains(t, path, "reflection-authoring-step.md") {
		t.Errorf("Reflection Authoring link to reflection-authoring-step.md was removed — Builder must preserve the tail")
	}
	_, _, gitCode, gitErr := acsassert.SubprocessOutput(
		"git", "-C", root, "ls-files", "--error-unmatch", filepath.Join("agents", "evolve-tester.md"))
	if gitErr != nil || gitCode != 0 {
		t.Errorf("agents/evolve-tester.md is not git-tracked (exit=%d: %v) — must not be moved or gitignored",
			gitCode, gitErr)
	}
}
