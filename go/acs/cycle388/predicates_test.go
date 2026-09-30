//go:build acs

package cycle388

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC388_001_LineCountReduced(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-builder.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read agents/evolve-builder.md: %v", err)
	}
	lineCount := strings.Count(string(raw), "\n")
	const maxLines = 279
	if lineCount > maxLines {
		t.Errorf("RED: agents/evolve-builder.md has %d lines — must be < 280 (≤279).\n"+
			"Builder must consolidate the triply-restated turn-exit rule (lines 56/251/263/265)\n"+
			"and remove the repeated self-assess-PASS anecdote to achieve ≥9-line reduction.\n"+
			"Baseline: 288 lines. Target: < 280.",
			lineCount)
	}
}

func TestC388_002_WordCountReduced(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-builder.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read agents/evolve-builder.md: %v", err)
	}
	wordCount := len(strings.Fields(string(raw)))
	const maxWords = 2779
	if wordCount > maxWords {
		t.Errorf("RED: agents/evolve-builder.md has %d words — must be < 2780 (≤2779).\n"+
			"A reflow-only edit that preserves all words cannot satisfy this.\n"+
			"Baseline: 2835 words. Target: < 2780.",
			wordCount)
	}
}

func TestC388_003_ByteCountReduced(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-builder.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read agents/evolve-builder.md: %v", err)
	}
	byteCount := len(raw)
	const maxBytes = 21993
	if byteCount > maxBytes {
		t.Errorf("RED: agents/evolve-builder.md is %d bytes — must be < 21994 (≤21993).\n"+
			"Any genuine content removal satisfies this; a no-op edit or whitespace-only\n"+
			"change that leaves the byte count at 21994 or above fails.\n"+
			"Baseline: 21994 bytes. Target: < 21994.",
			byteCount)
	}
}

// acs-predicate: config-check
func TestC388_004_FrontmatterIntact(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-builder.md")
	if !acsassert.FileContains(t, path, "name: evolve-builder") {
		t.Errorf("frontmatter field 'name: evolve-builder' was removed — Builder must not touch frontmatter")
	}
}

// acs-predicate: config-check
func TestC388_005_AllSectionHeadersPresent(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-builder.md")
	required := []string{
		"## Inputs",
		"## Strategy Handling",
		"## Core Principles",
		"## Worktree Isolation",
		"## Turn budget",
		"## Shared Constraints",
		"## Workflow",
		"## Reference Index",
		"## AC-TABLE Region",
		"## Pre-handoff Regression Slice",
		"## Pre-handoff Git Tracking Attestation",
		"## STOP CRITERION",
		"## EGPS Predicate Authoring",
		"## Output",
		"## POSTHOC enforcement",
		"## Reflection Authoring",
	}
	for _, header := range required {
		if !acsassert.FileContains(t, path, header) {
			t.Errorf("section %q was dropped — Builder must only remove redundant prose, not real sections", header)
		}
	}
}

// acs-predicate: config-check
func TestC388_006_BehaviorKeywordsPreserved(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-builder.md")
	required := []string{
		"Completion Gates",
		"report-written",
		"Builder MUST NOT write",
		"AC-TABLE",
		"POSTHOC",
		"reflection-authoring-step",
	}
	for _, kw := range required {
		if !acsassert.FileContains(t, path, kw) {
			t.Errorf("behavior keyword %q is missing after trim — Builder must preserve all behavioral rules and references", kw)
		}
	}
}

func TestC388_007_FileIsGitTracked(t *testing.T) {
	root := acsassert.RepoRoot(t)
	relPath := filepath.Join("agents", "evolve-builder.md")
	_, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", relPath)
	if err != nil || code != 0 {
		t.Errorf("RED: agents/evolve-builder.md is not tracked by git (exit=%d: %v).\n"+
			"Builder's edit must remain within the git-tracked file — do not move or gitignore it.",
			code, err)
	}
}
