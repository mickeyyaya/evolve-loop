//go:build acs

// Package cycle391 materializes the cycle-391 acceptance criteria for the
// committed top_n task:
//
//   - intent-prompt-token-reduction — remove ≥15 inert lines (≥600 bytes) from
//     agents/evolve-intent.md by deleting the C69–C73 calibration table
//     (lines ~116-126), the v9.0.1 design-correction paragraph (lines ~100-101),
//     and the "### No web research deadline" subsection (lines ~112-114),
//     while preserving every behavioral instruction, required section header,
//     and anchor.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	intent-prompt-token-reduction:
//	  AC1  line count ≤219 (baseline 234)                              → C391_001 (RED)
//	  AC1b byte count < 12769 (baseline 12769)                         → C391_002 (RED)
//	  AC2  all required section headers + behavioral anchors present   → C391_003 (pre-existing GREEN, config-check)
//	  AC3  archaeology markers absent (C69 / cycle 11 / No web ...)    → C391_004 (RED — markers present before Builder)
//	  AC4  negative: only agents/evolve-intent.md changed              → C391_005 (RED — wrong file in diff before Builder)
//	  AC5  frontmatter + Reflection Authoring tail present             → C391_006 (pre-existing GREEN, config-check)
//	  [adversarial] section-delete FAILs AC2; reflow-only FAILs AC1+AC1b;
//	                adding archaeology text FAILs AC3
//
// Floor binding (R9.3): predicates only for committed top_n task.
// Deferred tasks (BA1 multi-file strip, BA2 docguard) get zero predicates.
package cycle391

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// TestC391_001_LineCountReduced verifies that agents/evolve-intent.md has been
// trimmed to ≤219 lines (baseline: 234). Removing the 3 inert blocks
// (C69–C73 table ≈11 lines, design-correction paragraph ≈2 lines,
// "No web research deadline" subsection ≈3 lines) yields ≈218 lines.
//
// BEHAVIORAL: reads the file and counts newlines — not a grep/text-presence check.
// Adding a magic string cannot satisfy this; only genuine line removal can.
//
// NEGATIVE (adversarial): if Builder deleted a behavioral section to hit the
// target, AC2 (TestC391_003) would catch the missing header. AC1 + AC2 together
// enforce "fewer lines WITHOUT losing any real section."
//
// RED: currently 234 lines, which exceeds the ≤219 threshold.
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

// TestC391_002_ByteCountReduced verifies that agents/evolve-intent.md byte count
// drops below 12769 (the baseline). This is the anti-reflow guard: if Builder
// merely reflowed paragraphs without removing content, the byte count stays near
// 12769 and this predicate still FAILS.
//
// BEHAVIORAL: reads the file and counts bytes via len(raw). A reflow-only edit
// that preserves all content cannot pass this.
//
// RED: currently 12769 bytes, which is NOT < 12769.
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

// TestC391_003_AllBehavioralAnchorsPresent verifies that all required section
// headers and behavioral anchors survive the Builder's redundancy-removal edit.
//
// acs-predicate: config-check
//
// Anti-gaming counterpart to AC1: a Builder who deletes a whole behavioral
// section to hit the line-count target fails here. Only surgical removal of
// the 3 identified inert blocks satisfies both AC1 (fewer lines) and AC2 (all
// anchors intact).
//
// Pre-existing GREEN: all anchors present in the current 234-line file.
// Must remain GREEN after Builder's edit.
func TestC391_003_AllBehavioralAnchorsPresent(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-intent.md")

	// Required section headers.
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

	// Required behavioral anchors (AwN classifier + key contracts).
	requiredAnchors := []string{
		"awn_class",               // frontmatter field + classifier
		"IMKI",                    // AwN class
		"IMR",                     // AwN class
		"IwE",                     // AwN class
		"IBTC",                    // AwN class
		"CLEAR",                   // AwN class
		"challenged_premises",     // mandatory rule
		"Emergency Exit",          // turn-budget safety net
		"HARD STOP",               // absolute turn limit
		"INTENT_MODE",             // output contract
		"intent-delta.md",         // delta-mode contract
		"gate_intent_to_research", // ≥1 challenged-premise enforcement
	}
	for _, anchor := range requiredAnchors {
		if !acsassert.FileContains(t, path, anchor) {
			t.Errorf("behavioral anchor %q was removed — Builder must preserve all behavioral contracts",
				anchor)
		}
	}
}

// TestC391_004_ArchaeologyMarkersAbsent verifies that the three inert historical
// blocks have been removed from agents/evolve-intent.md.
//
// BEHAVIORAL: reads the file and checks that the archaeology markers are ABSENT.
// This is a NEGATIVE test — currently RED because all three markers are present.
// A Builder who adds the tokens back (or fails to remove them) fails here.
//
// RED: all three markers are currently present in the 234-line baseline.
// GREEN: only after all three inert blocks are removed by Builder.
//
// Adversarial note: this is the strongest anti-no-op signal for this task —
// a no-op implementation (unchanged file) would leave all markers present and
// fail this predicate.
func TestC391_004_ArchaeologyMarkersAbsent(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-intent.md")

	// Each marker uniquely identifies one of the three inert blocks to remove.
	// FileNotContains returns true and logs nothing when the substring is absent;
	// fails + logs when the substring is still present (cycle-352 lesson).
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

// TestC391_005_OnlyIntentFileChanged verifies that the cycle-391 commit changed
// exactly one file: agents/evolve-intent.md.
//
// BEHAVIORAL: runs `git diff HEAD~1..HEAD --name-only` in the worktree to list
// files changed in the most recent commit.
//
// NEGATIVE (adversarial): if Builder accidentally touched a control-plane file,
// a Go source file, or any file other than agents/evolve-intent.md, this
// predicate fails. The goal's HARD constraints prohibit all such edits.
//
// RED: before Builder's commit, HEAD in the worktree is the cycle-389 commit
// (c50c281f). The diff HEAD~1..HEAD shows agents/evolve-tester.md (cycle-389's
// change), NOT agents/evolve-intent.md, so the check fails.
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

// TestC391_006_FrontmatterAndReflectionTailPresent verifies that the YAML
// frontmatter identity fields and the Reflection Authoring tail are both intact
// after the trim.
//
// acs-predicate: config-check
//
// AC5 (edge/OOD): guards against truncation. A Builder who hits the line target
// by truncating the file's end loses the Reflection Authoring section and fails
// here. A Builder who strips the frontmatter loses the agent's identity.
//
// Also verifies git-tracking (cycle-93 lesson): disk presence alone is
// insufficient — a gitignored file would be silently dropped at ship.
//
// Pre-existing GREEN: both are present in the current 234-line file.
// Must remain GREEN after Builder's edit.
func TestC391_006_FrontmatterAndReflectionTailPresent(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-intent.md")

	// Frontmatter identity field.
	if !acsassert.FileContains(t, path, "name: evolve-intent") {
		t.Errorf("frontmatter field 'name: evolve-intent' was removed — Builder must not touch frontmatter")
	}
	// Reflection Authoring tail — the link must survive any trim.
	if !acsassert.FileContains(t, path, "reflection-authoring-step.md") {
		t.Errorf("Reflection Authoring link to reflection-authoring-step.md was removed — Builder must preserve the tail")
	}

	// Git-tracking check (cycle-93 lesson: disk presence alone is insufficient).
	_, _, gitCode, gitErr := acsassert.SubprocessOutput(
		"git", "-C", root, "ls-files", "--error-unmatch", filepath.Join("agents", "evolve-intent.md"))
	if gitErr != nil || gitCode != 0 {
		t.Errorf("agents/evolve-intent.md is not git-tracked (exit=%d: %v) — must not be moved or gitignored",
			gitCode, gitErr)
	}
}
