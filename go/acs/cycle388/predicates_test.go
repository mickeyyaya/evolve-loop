//go:build acs

// Package cycle388 materializes the cycle-388 acceptance criteria for the
// committed top_n task:
//
//   - trim-builder-prompt-redundancy — remove ≥9 redundant lines (and ≥56 words,
//     strictly fewer bytes) from agents/evolve-builder.md by consolidating the
//     triply-restated turn-exit rule and removing a duplicate self-assess-PASS
//     anecdote, while preserving every real section, frontmatter field, and
//     behavioral keyword.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	trim-builder-prompt-redundancy:
//	  AC1  line count < 280 (baseline 288)                              → C388_001 (RED)
//	  AC2  word count < 2780 (baseline 2835)                           → C388_002 (RED)
//	  AC3  byte count < 21994 (baseline 21994)                         → C388_003 (RED)
//	  AC4  frontmatter intact (name: evolve-builder)                   → C388_004 (pre-existing GREEN)
//	  AC5  all 16 real ## section headers present                      → C388_005 (pre-existing GREEN)
//	  AC6  behavior keywords preserved                                  → C388_006 (pre-existing GREEN)
//	  AC7  file is git-tracked                                          → C388_007 (pre-existing GREEN)
//	  [adversarial] section-delete FAILs AC5; reflow-only FAILs AC2+AC3
//
// Floor binding (R9.3): predicates only for committed top_n task.
// Deferred task (auditor dedup) gets zero predicates.
package cycle388

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// TestC388_001_LineCountReduced verifies that agents/evolve-builder.md has been
// trimmed to < 280 lines (baseline: 288). This is the primary RED indicator:
// the file is currently 288 lines, which exceeds the threshold.
//
// BEHAVIORAL: reads the file and counts newlines — not a grep/text-presence check.
// Adding a magic string cannot satisfy this; only genuine line removal can.
//
// NEGATIVE (adversarial): if Builder deleted a section to hit the target, AC5
// would catch the missing header. The two predicates together enforce
// "fewer lines WITHOUT losing any real section."
//
// RED: currently 288 lines, which exceeds the < 280 threshold.
func TestC388_001_LineCountReduced(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-builder.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read agents/evolve-builder.md: %v", err)
	}
	lineCount := strings.Count(string(raw), "\n")
	const maxLines = 279 // strictly < 280
	if lineCount > maxLines {
		t.Errorf("RED: agents/evolve-builder.md has %d lines — must be < 280 (≤279).\n"+
			"Builder must consolidate the triply-restated turn-exit rule (lines 56/251/263/265)\n"+
			"and remove the repeated self-assess-PASS anecdote to achieve ≥9-line reduction.\n"+
			"Baseline: 288 lines. Target: < 280.",
			lineCount)
	}
}

// TestC388_002_WordCountReduced verifies that agents/evolve-builder.md word count
// drops below 2780 (baseline: 2835). This is the anti-reflow guard:
// if Builder merely reflowed paragraphs without removing content, word count
// stays near 2835 and this predicate still FAILS.
//
// BEHAVIORAL: reads the file and counts words via strings.Fields — equivalent to
// `wc -w`. A reflow-only edit that leaves all words intact cannot pass this.
//
// RED: currently 2835 words, which exceeds 2780.
func TestC388_002_WordCountReduced(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-builder.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read agents/evolve-builder.md: %v", err)
	}
	wordCount := len(strings.Fields(string(raw)))
	const maxWords = 2779 // strictly < 2780
	if wordCount > maxWords {
		t.Errorf("RED: agents/evolve-builder.md has %d words — must be < 2780 (≤2779).\n"+
			"A reflow-only edit that preserves all words cannot satisfy this.\n"+
			"Baseline: 2835 words. Target: < 2780.",
			wordCount)
	}
}

// TestC388_003_ByteCountReduced verifies that agents/evolve-builder.md byte count
// is strictly less than 21994 (baseline: 21994). The baseline IS the threshold —
// any non-empty removal must satisfy this, but a no-op edit fails.
//
// BEHAVIORAL: reads the file and checks raw byte length.
//
// RED: currently 21994 bytes, which is NOT < 21994 (strict decrease required).
func TestC388_003_ByteCountReduced(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-builder.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read agents/evolve-builder.md: %v", err)
	}
	byteCount := len(raw)
	const maxBytes = 21993 // strictly < 21994
	if byteCount > maxBytes {
		t.Errorf("RED: agents/evolve-builder.md is %d bytes — must be < 21994 (≤21993).\n"+
			"Any genuine content removal satisfies this; a no-op edit or whitespace-only\n"+
			"change that leaves the byte count at 21994 or above fails.\n"+
			"Baseline: 21994 bytes. Target: < 21994.",
			byteCount)
	}
}

// TestC388_004_FrontmatterIntact verifies that the YAML frontmatter field
// `name: evolve-builder` is still present after Builder's edit.
//
// acs-predicate: config-check
//
// Pre-existing GREEN: field exists in the current 288-line file.
// Must remain GREEN after Builder's edit (frontmatter is identity-critical).
func TestC388_004_FrontmatterIntact(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-builder.md")
	if !acsassert.FileContains(t, path, "name: evolve-builder") {
		t.Errorf("frontmatter field 'name: evolve-builder' was removed — Builder must not touch frontmatter")
	}
}

// TestC388_005_AllSectionHeadersPresent verifies that all 16 real ## section
// headers survive Builder's redundancy-removal edit.
//
// acs-predicate: config-check
//
// Anti-gaming counterpart to AC1: a Builder who deletes a whole section to hit
// the line-count target fails here. Only genuine de-duplication of redundant
// prose satisfies both AC1 (fewer lines) and AC5 (all sections intact).
//
// Pre-existing GREEN: all 16 headers present in the current 288-line file.
// Must remain GREEN after Builder's edit.
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

// TestC388_006_BehaviorKeywordsPreserved verifies that the load-bearing
// behavioral keywords and references survive the prose trim.
//
// acs-predicate: config-check
//
// Pre-existing GREEN: all keywords present in the current file.
// Must remain GREEN after Builder's edit (no behavioral rule may be dropped).
func TestC388_006_BehaviorKeywordsPreserved(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-builder.md")
	required := []string{
		"Completion Gates",          // STOP CRITERION table (must survive consolidation)
		"report-written",            // gate name in Completion Gates table
		"Builder MUST NOT write",    // predicate-authoring ban in EGPS section
		"AC-TABLE",                  // AC-TABLE region marker
		"POSTHOC",                   // POSTHOC enforcement directive
		"reflection-authoring-step", // reflection link in Reflection Authoring section
	}
	for _, kw := range required {
		if !acsassert.FileContains(t, path, kw) {
			t.Errorf("behavior keyword %q is missing after trim — Builder must preserve all behavioral rules and references", kw)
		}
	}
}

// TestC388_007_FileIsGitTracked verifies that agents/evolve-builder.md remains
// a tracked git file after Builder's markdown-only edit.
//
// BEHAVIORAL: runs `git ls-files --error-unmatch` as a subprocess. Disk presence
// alone is insufficient — a gitignored worktree file would be silently dropped at
// ship (cycle-93 lesson).
//
// Pre-existing GREEN: file is already tracked. Builder's markdown edit must not
// cause it to become untracked or gitignored.
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
