//go:build acs

// Package cycle387 materializes the cycle-387 acceptance criteria for the
// committed top_n task:
//
//   - trim-tdd-engineer-prompt-redundancy — remove ≥27 redundant lines from
//     agents/evolve-tdd-engineer.md (retired-bash repetitions and fallback shell
//     example) while preserving every real section, frontmatter field, and
//     behavioral keyword.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	trim-tdd-engineer-prompt-redundancy:
//	  AC1  line count ≤405 (≥27-line reduction from baseline 432)   → C387_001 (RED)
//	  AC2  all 9 ## section headers present                          → C387_002 (pre-existing GREEN)
//	  AC3  frontmatter + behavioral keywords preserved               → C387_003 (pre-existing GREEN)
//	  AC4  file is git-tracked (markdown-only edit, tracked)         → C387_004 (pre-existing GREEN)
//	  [adversarial] sections not dropped to hit line count           → C387_002 (anti-gaming via AC1+AC2 combo)
//
// Floor binding (R9.3): predicates only for committed top_n task.
// Deferred tasks (BA1, BA2, carryover infra todos) get zero predicates.
package cycle387

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// TestC387_001_LineCountReduced verifies that agents/evolve-tdd-engineer.md has
// been trimmed to ≤405 lines (baseline: 432). This is the primary RED indicator:
// the file is still 432 lines until Builder removes the redundant content.
//
// BEHAVIORAL: reads the file and counts newlines — not a grep/text-presence check.
// A magic-string addition cannot satisfy this; only genuine line removal can.
//
// NEGATIVE: the anti-gaming signal. A predicate that accepts the current 432-line
// file would pass without any implementation change — this one doesn't.
//
// RED: file is currently 432 lines, which exceeds the ≤405 threshold.
func TestC387_001_LineCountReduced(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-tdd-engineer.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read agents/evolve-tdd-engineer.md: %v", err)
	}
	lineCount := strings.Count(string(raw), "\n")
	const maxLines = 405
	if lineCount > maxLines {
		t.Errorf("RED: agents/evolve-tdd-engineer.md has %d lines — must be ≤%d.\n"+
			"Builder must remove the retired-bash repetitions and the fallback shell-test\n"+
			"example (lines 103–128 per scout-report.md) to achieve the ≥27-line reduction.\n"+
			"Baseline: 432 lines. Target: ≤405.",
			lineCount, maxLines)
	}
}

// TestC387_002_AllSectionHeadersPresent verifies that every one of the nine
// required ## section headers survives the Builder's line-reduction edit.
//
// acs-predicate: config-check
//
// Anti-gaming counterpart to AC1: a Builder who deletes an entire section to
// hit the line-count target fails here. Only genuine de-duplication of redundant
// prose satisfies both AC1 (fewer lines) and AC2 (all sections intact).
//
// Pre-existing GREEN: all 9 headers are present in the current 432-line file.
// Must remain GREEN after Builder's edit.
func TestC387_002_AllSectionHeadersPresent(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-tdd-engineer.md")
	required := []string{
		"## Inputs",
		"## Pipeline Position",
		"## Workflow",
		"## Operating Principles",
		"## AC-Materialization Contract",
		"## Predicate Quality Requirements",
		"## Failure Modes",
		"## Output",
		"## Reflection Authoring",
	}
	for _, header := range required {
		if !acsassert.FileContains(t, path, header) {
			t.Errorf("section %q was dropped — Builder must only remove redundant prose, not real sections",
				header)
		}
	}
}

// TestC387_003_FrontmatterAndKeywordsPreserved verifies that both frontmatter
// fields and all load-bearing behavioral keywords are still present after the
// prose trim.
//
// acs-predicate: config-check
//
// Pre-existing GREEN: all fields and keywords are present in the current file.
// Must remain GREEN after Builder's edit (no behavioral rule may be dropped).
func TestC387_003_FrontmatterAndKeywordsPreserved(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-tdd-engineer.md")
	required := []string{
		"perspective:",
		"output-format:",
		"//go:build acs",
		"acsassert",
		"FORBIDDEN",
		"RED",
		".evolve/evals/",
	}
	for _, kw := range required {
		if !acsassert.FileContains(t, path, kw) {
			t.Errorf("keyword/field %q is missing after trim — Builder must preserve all behavioral keywords and frontmatter fields",
				kw)
		}
	}
}

// TestC387_004_FileIsGitTracked verifies that agents/evolve-tdd-engineer.md
// remains a tracked git file after Builder's markdown-only edit.
//
// BEHAVIORAL: runs `git ls-files --error-unmatch` as a subprocess. Disk presence
// alone is insufficient — a gitignored worktree file would be silently dropped at
// ship (cycle-93 lesson).
//
// Pre-existing GREEN: file is already tracked. Builder's markdown edit must not
// cause it to become untracked or gitignored.
func TestC387_004_FileIsGitTracked(t *testing.T) {
	root := acsassert.RepoRoot(t)
	relPath := filepath.Join("agents", "evolve-tdd-engineer.md")
	_, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", relPath)
	if err != nil || code != 0 {
		t.Errorf("RED: agents/evolve-tdd-engineer.md is not tracked by git (exit=%d: %v).\n"+
			"Builder's edit must remain within the git-tracked file — do not move or gitignore it.",
			code, err)
	}
}
