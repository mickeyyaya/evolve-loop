//go:build acs

// Package cycle389 materializes the cycle-389 acceptance criteria for the
// committed top_n task:
//
//   - trim-evolve-tester-prompt — remove ≥8 redundant lines (and ≥40 words)
//     from agents/evolve-tester.md by collapsing the triplicated worktree-resolution
//     boilerplate, tightening the adversarial-mindset restatement, and condensing
//     low-density preamble prose, while preserving every real section, frontmatter
//     field, and behavioral contract.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	trim-evolve-tester-prompt:
//	  AC1  line count ≤185 (baseline 193)                              → C389_001 (RED)
//	  AC1b word count ≤1280 (baseline 1320)                           → C389_002 (RED)
//	  AC2  all 10 ## section headers present                          → C389_003 (pre-existing GREEN)
//	  AC3  banned-patterns list + metadata contract intact            → C389_004 (pre-existing GREEN)
//	  AC4  negative: only agents/evolve-tester.md changed             → C389_005 (RED — wrong file in diff before Builder)
//	  AC5  frontmatter + Reflection Authoring tail present            → C389_006 (pre-existing GREEN)
//	  [adversarial] section-delete FAILs AC2; reflow-only FAILs AC1+AC1b
//
// Floor binding (R9.3): predicates only for committed top_n task.
// Deferred tasks (B1 DRY pass, B2 linter) get zero predicates.
package cycle389

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// TestC389_001_LineCountReduced verifies that agents/evolve-tester.md has been
// trimmed to ≤185 lines (baseline: 193). This is the primary RED indicator:
// the file is currently 193 lines, which exceeds the threshold.
//
// BEHAVIORAL: reads the file and counts newlines — not a grep/text-presence check.
// Adding a magic string cannot satisfy this; only genuine line removal can.
//
// NEGATIVE (adversarial): if Builder deleted a section to hit the target, AC2
// would catch the missing header. AC1 + AC2 together enforce "fewer lines
// WITHOUT losing any real section."
//
// RED: currently 193 lines, which exceeds the ≤185 threshold.
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

// TestC389_002_WordCountReduced verifies that agents/evolve-tester.md word count
// drops to ≤1280 (baseline: 1320). This is the anti-reflow guard:
// if Builder merely reflowed paragraphs without removing content, the word count
// stays near 1320 and this predicate still FAILS.
//
// BEHAVIORAL: reads the file and counts words via strings.Fields — equivalent to
// `wc -w`. A reflow-only edit that leaves all words intact cannot pass this.
//
// RED: currently 1320 words, which exceeds 1280.
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

// TestC389_003_AllSectionHeadersPresent verifies that every one of the ten
// required ## section headers survives the Builder's redundancy-removal edit.
//
// acs-predicate: config-check
//
// Anti-gaming counterpart to AC1: a Builder who deletes a whole section to hit
// the line-count target fails here. Only genuine de-duplication of redundant
// prose satisfies both AC1 (fewer lines) and AC2 (all sections intact).
//
// Pre-existing GREEN: all 10 headers are present in the current 193-line file.
// Must remain GREEN after Builder's edit.
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

// TestC389_004_BannedPatternsAndMetadataIntact verifies that the 6-item
// banned-patterns list and the required predicate metadata-header fields are
// still present after the prose trim.
//
// acs-predicate: config-check
//
// Pre-existing GREEN: all items and fields present in the current 193-line file.
// Must remain GREEN after Builder's edit (the Tester behavioral contract must
// not be weakened by a prose reduction).
func TestC389_004_BannedPatternsAndMetadataIntact(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-tester.md")
	// The 6 banned-pattern items — distinctive tokens from each item.
	bannedPatterns := []string{
		"presence ≠ execution",   // item 1: grep-only ban
		`echo "PASS"; exit 0`,    // item 2: tautology ban
		"hermetic-determinism",   // item 3: curl/wget ban rationale
		"sleep",                  // item 4: sleep-duration ban
		"acs-output/",            // item 5: write-scope restriction
		"Lack required metadata", // item 6: metadata-header requirement
	}
	for _, token := range bannedPatterns {
		if !acsassert.FileContains(t, path, token) {
			t.Errorf("banned-pattern item %q was removed — Builder must not touch the 6-item Banned patterns list",
				token)
		}
	}
	// Metadata-header field names from the predicate template.
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

// TestC389_005_OnlyTesterFileChanged verifies that the cycle-389 commit changed
// exactly one file: agents/evolve-tester.md.
//
// BEHAVIORAL: runs `git diff HEAD~1..HEAD --name-only` in the worktree to list
// files changed in the most recent commit.
//
// NEGATIVE (adversarial): if Builder accidentally touched a control-plane file,
// a Go source file, or any file other than agents/evolve-tester.md, this
// predicate fails. The goal's HARD constraints prohibit all such edits.
//
// RED: before Builder's commit, HEAD in the worktree is the cycle-388 commit
// (d84e1fa7). The diff HEAD~1..HEAD shows agents/evolve-builder.md (the
// cycle-388 edit), NOT agents/evolve-tester.md, so the check fails.
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

// TestC389_006_FrontmatterAndReflectionTailPresent verifies that the YAML
// frontmatter and the Reflection Authoring tail are both intact after the trim.
//
// acs-predicate: config-check
//
// AC5 (edge): guards against truncation. A Builder who hits the line target
// by truncating the file's end loses the Reflection Authoring section and fails
// here. A Builder who strips the frontmatter loses the agent's identity and
// also fails here.
//
// Pre-existing GREEN: both are present in the current 193-line file.
// Must remain GREEN after Builder's edit.
func TestC389_006_FrontmatterAndReflectionTailPresent(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-tester.md")
	// Frontmatter identity field.
	if !acsassert.FileContains(t, path, "name: evolve-tester") {
		t.Errorf("frontmatter field 'name: evolve-tester' was removed — Builder must not touch frontmatter")
	}
	// Reflection Authoring tail — the link to the step file must survive any trim.
	if !acsassert.FileContains(t, path, "reflection-authoring-step.md") {
		t.Errorf("Reflection Authoring link to reflection-authoring-step.md was removed — Builder must preserve the tail")
	}
	// Git-tracking check (cycle-93 lesson: disk presence alone is insufficient).
	_, _, gitCode, gitErr := acsassert.SubprocessOutput(
		"git", "-C", root, "ls-files", "--error-unmatch", filepath.Join("agents", "evolve-tester.md"))
	if gitErr != nil || gitCode != 0 {
		t.Errorf("agents/evolve-tester.md is not git-tracked (exit=%d: %v) — must not be moved or gitignored",
			gitCode, gitErr)
	}
}
