//go:build acs

// Package cycle412 materializes the cycle-412 acceptance criteria for two prompt-optimization tasks:
//   - prune-dead-legacy-script-refs (agents/evolve-{scout,builder,auditor,tdd-engineer,orchestrator,triage}.md)
//   - dedupe-phase-prompt-reference-index (agents/evolve-{scout,builder,auditor}.md)
//
// Goal: optimize per-agent token usage by removing 18 dead legacy/scripts refs, 5 v12.0.0
// disclaimer blocks, and collapsing 30 repeated reference-file paths (9/12/9) to ≤2 each.
//
// AC map (1:1 with scout-report.md top_n; R9.3 floor-binding):
//
//	prune-dead-legacy-script-refs:
//	  AC1 zero legacy/scripts refs across 6 prompt files (18 currently)  → C412_001 (RED)
//	  AC2 combined word count < 14126 (currently exactly 14126)           → C412_002 (RED)
//	  AC3 gate anchors preserved (STOP CRITERION, challenge-token, etc)   → C412_003 (pre-existing GREEN, config-check)
//	  AC4 no file gutted (each prompt ≥ 100 lines)                        → C412_004 (pre-existing GREEN)
//	  AC5 v12.0.0 status disclaimers removed (5 currently)                → C412_005 (RED)
//
//	dedupe-phase-prompt-reference-index:
//	  AC1 reference path repeated ≤ 2× per file (scout 9, builder 12, auditor 9 currently) → C412_006 (RED)
//	  AC2 all 7 scout reference section names present after collapse                         → C412_007 (pre-existing GREEN, config-check)
//	  AC3 ≥1 pointer per file (not amputated wholesale)                                      → C412_008 (pre-existing GREEN)
//	  AC4 combined scout+builder+auditor word count < 6861 (currently exactly 6861)          → C412_009 (RED)
//
// Adversarial diversity (per SKILL §6):
//
//	Negative: legacy/scripts present → C412_001 (RED); v12.0.0 disclaimer present → C412_005 (RED);
//	          path repeat > 2 → C412_006 (RED).
//	Edge/OOD: file gutted (< 100 lines) → C412_004; pointer amputated entirely → C412_008.
//	Semantic:  word-count reduction (distinct from string-absence) → C412_002, C412_009.
//
// Deferred (zero predicates per R9.3): 27 carryover breadcrumbs (all infra/codex-tmux boot-wedge class).
package cycle412

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// countWords returns the number of whitespace-delimited tokens (matches wc -w).
func countWords(text string) int {
	return len(strings.Fields(text))
}

// countSubstring returns the number of non-overlapping occurrences of substr in s.
func countSubstring(s, substr string) int {
	return strings.Count(s, substr)
}

// countLines returns the number of newline-terminated or final lines in text.
func countLines(text string) int {
	lines := strings.Split(text, "\n")
	// Drop trailing empty string from a final newline.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		return len(lines) - 1
	}
	return len(lines)
}

// ── Task 1: prune-dead-legacy-script-refs ────────────────────────────────────

// TestC412_001_NoLegacyScriptRefsInAnyPrompt asserts that every occurrence of
// "legacy/scripts" has been removed from all six phase-prompt files.
//
// BEHAVIORAL: reads each file and counts "legacy/scripts" occurrences; the test
// fails while any exist. Adding or commenting text cannot satisfy it — only
// genuine removal of all 18 references passes.
//
// NEGATIVE (adversarial): an unmodified tree (18 total occurrences) fails here.
// This is the primary anti-no-op signal for prune-dead-legacy-script-refs.
//
// RED: currently 20 occurrences total (scout 2, builder 5, auditor 3, tdd 3,
// orchestrator 5, triage 2). The scout cited 18 counting lines via grep -c;
// builder:131 and tdd:83 each have 2 occurrences on a single line.
func TestC412_001_NoLegacyScriptRefsInAnyPrompt(t *testing.T) {
	root := acsassert.RepoRoot(t)
	files := []string{
		filepath.Join(root, "agents", "evolve-scout.md"),
		filepath.Join(root, "agents", "evolve-builder.md"),
		filepath.Join(root, "agents", "evolve-auditor.md"),
		filepath.Join(root, "agents", "evolve-tdd-engineer.md"),
		filepath.Join(root, "agents", "evolve-orchestrator.md"),
		filepath.Join(root, "agents", "evolve-triage.md"),
	}
	total := 0
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("cannot read %s: %v", f, err)
		}
		n := countSubstring(string(raw), "legacy/scripts")
		if n > 0 {
			t.Errorf("RED: %s still contains %d 'legacy/scripts' reference(s) — "+
				"all 18 dead refs must be removed and replaced with their in-process equivalents.\n"+
				"Each removed instruction must be replaced with the current Go orchestrator / "+
				"evolve CLI equivalent documented in its local v12.0.0 status block.",
				filepath.Base(f), n)
		}
		total += n
	}
	if total > 0 {
		t.Errorf("RED: total 'legacy/scripts' occurrences across all prompts: %d (must be 0)", total)
	}
}

// TestC412_002_CombinedWordCountReduced asserts that the total word count
// across all six phase-prompt files is strictly less than 13900.
//
// BEHAVIORAL: reads and counts whitespace-delimited tokens (strings.Fields semantics).
// Removing 20 dead legacy/scripts instruction occurrences plus 5 v12.0.0 disclaimer
// blocks reduces the word count by ~300–500 words from the Go-measured baseline of
// 14124. The threshold 13900 sits between the baseline and the expected post-cleanup
// floor; Builder must perform the actual dead-ref removal to pass it.
//
// NEGATIVE: an unmodified tree totals 14124 words (> 13900), so this fails.
// Only genuine removal of dead instructions satisfies it.
//
// RED: combined Go-measured baseline = 14124 (scout 1915, builder 2683, auditor 2263,
// tdd 2801, orchestrator 2356, triage 2106). The scout report cited 14126 via wc -w;
// Go's strings.Fields measures 14124 for the same files.
func TestC412_002_CombinedWordCountReduced(t *testing.T) {
	root := acsassert.RepoRoot(t)
	files := []struct {
		rel      string
		baseline int
	}{
		{"agents/evolve-scout.md", 1915},
		{"agents/evolve-builder.md", 2683},
		{"agents/evolve-auditor.md", 2263},
		{"agents/evolve-tdd-engineer.md", 2801},
		{"agents/evolve-orchestrator.md", 2358},
		{"agents/evolve-triage.md", 2106},
	}
	total := 0
	for _, f := range files {
		raw, err := os.ReadFile(filepath.Join(root, f.rel))
		if err != nil {
			t.Fatalf("cannot read %s: %v", f.rel, err)
		}
		total += countWords(string(raw))
	}
	const maxWords = 13900 // baseline 14124; target ≤ 13900 after dead-ref cleanup
	if total > maxWords {
		t.Errorf("RED: combined word count across 6 phase prompts = %d (Go baseline 14124) — "+
			"must be ≤ 13900 after removing dead legacy/scripts instructions and v12.0.0 disclaimers.\n"+
			"Expected reduction: ~300-500 words from ~20 dead-instruction occurrences + 5 disclaimer blocks.\n"+
			"Current count %d > 13900 — dead-ref removal not yet applied.",
			total, total)
	}
}

// TestC412_003_GateAnchorsPreserved asserts that integrity-critical gate anchors
// remain in each phase prompt after the legacy/scripts cleanup.
//
// acs-predicate: config-check
//
// ANTI-GAMING: if Builder over-deletes (removes a STOP CRITERION block or
// challenge-token instruction while stripping legacy/scripts), this fails.
//
// Pre-existing GREEN: all anchors present before any edits.
func TestC412_003_GateAnchorsPreserved(t *testing.T) {
	root := acsassert.RepoRoot(t)

	checks := []struct {
		file    string
		anchors []string
	}{
		{
			"agents/evolve-scout.md",
			[]string{"STOP CRITERION", "Gates (all six required)", "challenge-token"},
		},
		{
			"agents/evolve-builder.md",
			[]string{"STOP CRITERION", "AC-TABLE-BEGIN"},
		},
		{
			"agents/evolve-auditor.md",
			[]string{"STOP CRITERION", "EGPS Verdict Computation", "challenge-token"},
		},
		{
			"agents/evolve-tdd-engineer.md",
			[]string{"challenge-token"},
		},
		{
			"agents/evolve-orchestrator.md",
			[]string{"STOP CRITERION", "evolve guard phase"},
		},
		{
			"agents/evolve-triage.md",
			[]string{"challenge-token"},
		},
	}

	for _, c := range checks {
		path := filepath.Join(root, c.file)
		for _, anchor := range c.anchors {
			if !acsassert.FileContains(t, path, anchor) {
				t.Errorf("gate anchor %q was removed from %s — "+
					"legacy/scripts cleanup must not delete gate-critical instructions",
					anchor, c.file)
			}
		}
	}
}

// TestC412_004_NoFileGutted asserts that none of the six phase-prompt files
// was emptied or severely truncated during the legacy/scripts cleanup.
//
// EDGE (anti-gaming): a Builder that deletes an entire section to remove a
// legacy/scripts reference would drive a file below 100 lines. Each file
// is currently 219–405 lines — well above the 100-line floor.
//
// Pre-existing GREEN: all files are ≥219 lines currently.
func TestC412_004_NoFileGutted(t *testing.T) {
	root := acsassert.RepoRoot(t)
	files := []string{
		"agents/evolve-scout.md",
		"agents/evolve-builder.md",
		"agents/evolve-auditor.md",
		"agents/evolve-tdd-engineer.md",
		"agents/evolve-orchestrator.md",
		"agents/evolve-triage.md",
	}
	for _, rel := range files {
		raw, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("cannot read %s: %v", rel, err)
		}
		n := countLines(string(raw))
		const minLines = 100
		if n < minLines {
			t.Errorf("%s has only %d lines — floor is %d. "+
				"Dead-ref removal must not gut entire sections.",
				rel, n, minLines)
		}
	}
}

// TestC412_005_NoV12StatusDisclaimers asserts that all five
// "> **v12.0.0 status:**" disclaimer blocks have been removed from the six
// phase-prompt files.
//
// BEHAVIORAL: counts "v12.0.0 status" occurrences per file. The disclaimers
// exist solely to neutralize the dead legacy/scripts references — once those
// references are removed (AC1), the disclaimers are pure workaround weight
// and must be deleted too (CLAUDE rule no_workaround_root_cause_redesign).
//
// NEGATIVE (adversarial): the unmodified tree has 5 disclaimers (scout 1,
// builder 1, auditor 1, tdd 1, triage 1; orchestrator 0) — all must go.
//
// RED: currently 5 occurrences total.
func TestC412_005_NoV12StatusDisclaimers(t *testing.T) {
	root := acsassert.RepoRoot(t)
	files := []string{
		filepath.Join(root, "agents", "evolve-scout.md"),
		filepath.Join(root, "agents", "evolve-builder.md"),
		filepath.Join(root, "agents", "evolve-auditor.md"),
		filepath.Join(root, "agents", "evolve-tdd-engineer.md"),
		filepath.Join(root, "agents", "evolve-orchestrator.md"),
		filepath.Join(root, "agents", "evolve-triage.md"),
	}
	total := 0
	for _, f := range files {
		if !acsassert.FileNotContains(t, f, "v12.0.0 status") {
			n := countSubstring(func() string {
				raw, _ := os.ReadFile(f)
				return string(raw)
			}(), "v12.0.0 status")
			t.Errorf("RED: %s still contains %d 'v12.0.0 status' disclaimer(s) — "+
				"these cover-disclaimers must be deleted once their legacy/scripts triggers are removed.",
				filepath.Base(f), n)
			total += n
		}
	}
	if total > 0 {
		t.Errorf("RED: total 'v12.0.0 status' occurrences: %d (must be 0 after cleanup)", total)
	}
}

// ── Task 2: dedupe-phase-prompt-reference-index ───────────────────────────────

// countLinesContaining returns the number of lines in text that contain substr.
// Matches grep -c semantics: lines counted, not occurrences per line.
func countLinesContaining(text, substr string) int {
	n := 0
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, substr) {
			n++
		}
	}
	return n
}

// TestC412_006_ReferencePathLineCountAtMostTwo asserts that the reference-file
// path appears on at most 2 lines per file in the three target prompts
// (grep -c semantics: lines, not occurrence count).
//
// BEHAVIORAL: counts lines containing each path (grep -c equivalent).
// The Reference Index currently repeats the full path on every row AND many
// body-text inline refs: scout 9 lines, builder 12 lines, auditor 9 lines.
// After collapse, the path should appear on ≤2 lines total (one base-pointer
// declaration; all other mentions use just the section name).
//
// NEGATIVE (adversarial): unmodified files have 9/12/9 lines — all above
// the ≤2 limit. Primary anti-no-op signal for dedupe-phase-prompt-reference-index.
//
// RED: scout=9, builder=12, auditor=9 lines containing the reference path currently.
func TestC412_006_ReferencePathLineCountAtMostTwo(t *testing.T) {
	root := acsassert.RepoRoot(t)
	checks := []struct {
		file    string
		refPath string
		current int
	}{
		{"agents/evolve-scout.md", "agents/evolve-scout-reference.md", 9},
		{"agents/evolve-builder.md", "agents/evolve-builder-reference.md", 12},
		{"agents/evolve-auditor.md", "agents/evolve-auditor-reference.md", 9},
	}
	for _, c := range checks {
		raw, err := os.ReadFile(filepath.Join(root, c.file))
		if err != nil {
			t.Fatalf("cannot read %s: %v", c.file, err)
		}
		n := countLinesContaining(string(raw), c.refPath)
		if n > 2 {
			t.Errorf("RED: %s has %q on %d lines (currently %d; must be ≤2 lines).\n"+
				"Collapse: declare the base path once as a section header; replace all other "+
				"inline occurrences with just the section name.",
				c.file, c.refPath, n, c.current)
		}
	}
}

// TestC412_007_AllScoutReferenceSectionsPresent asserts that all seven
// section names from the scout Reference Index survive after path deduplication.
//
// acs-predicate: config-check
//
// ANTI-GAMING: if Builder collapses the table by deleting rows (removing section
// names instead of just shortening the repeated path), this fails. The seven
// section names must remain discoverable in the file.
//
// Pre-existing GREEN: all 7 sections present in current scout file.
func TestC412_007_AllScoutReferenceSectionsPresent(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "agents", "evolve-scout.md")
	sections := []string{
		"turn-budget-rationale",
		"mode-discovery-detail",
		"eval-integrity-rules",
		"eval-format-template",
		"output-template",
		"task-selection-tables",
		"project-digest-template",
	}
	for _, section := range sections {
		if !acsassert.FileContains(t, path, section) {
			t.Errorf("scout reference section %q was removed from agents/evolve-scout.md — "+
				"path deduplication must preserve every section name; only the repeated full-path "+
				"prefix on each row should be collapsed",
				section)
		}
	}
}

// TestC412_008_ReferencePointerNotAmputated asserts that each target file
// retains at least one pointer to its reference file after deduplication.
//
// EDGE (anti-amputate): a deduplication that deletes the entire Reference Index
// section would satisfy C412_006 (≤2 repeats) but violate the "on-demand
// reference mechanism stays" integrity constraint. At least one discoverable
// pointer must survive.
//
// Pre-existing GREEN: all three files currently have 9/12/9 pointers.
func TestC412_008_ReferencePointerNotAmputated(t *testing.T) {
	root := acsassert.RepoRoot(t)
	checks := []struct {
		file    string
		refPath string
	}{
		{"agents/evolve-scout.md", "agents/evolve-scout-reference.md"},
		{"agents/evolve-builder.md", "agents/evolve-builder-reference.md"},
		{"agents/evolve-auditor.md", "agents/evolve-auditor-reference.md"},
	}
	for _, c := range checks {
		raw, err := os.ReadFile(filepath.Join(root, c.file))
		if err != nil {
			t.Fatalf("cannot read %s: %v", c.file, err)
		}
		n := countLinesContaining(string(raw), c.refPath)
		if n < 1 {
			t.Errorf("%s has 0 lines with %q — the reference file pointer was amputated.\n"+
				"Deduplication must keep ≥1 discoverable pointer (the base-path declaration row).",
				c.file, c.refPath)
		}
	}
}

// TestC412_009_ScoutBuilderAuditorWordCountReduced asserts that the combined
// word count of the three Reference-Index-bearing prompts is strictly less than
// 6861 after deduplication.
//
// BEHAVIORAL: reads and word-counts the three target files (scout+builder+auditor).
// The current combined baseline is exactly 6861 — collapsing 30 redundant
// path strings must reduce this below the baseline.
//
// NEGATIVE: an unmodified tree totals exactly 6861 — not strictly less, so
// this fails. Only genuine Reference Index collapse satisfies it.
//
// RED: scout 1915 + builder 2683 + auditor 2263 = 6861 currently.
func TestC412_009_ScoutBuilderAuditorWordCountReduced(t *testing.T) {
	root := acsassert.RepoRoot(t)
	files := []struct {
		rel      string
		baseline int
	}{
		{"agents/evolve-scout.md", 1915},
		{"agents/evolve-builder.md", 2683},
		{"agents/evolve-auditor.md", 2263},
	}
	total := 0
	for _, f := range files {
		raw, err := os.ReadFile(filepath.Join(root, f.rel))
		if err != nil {
			t.Fatalf("cannot read %s: %v", f.rel, err)
		}
		total += countWords(string(raw))
	}
	const maxWords = 6860 // strictly < 6861
	if total > maxWords {
		t.Errorf("RED: combined word count for scout+builder+auditor = %d (baseline 6861) — "+
			"must be < 6861 after collapsing Reference Index path repetition.\n"+
			"Current count %d == baseline — no Reference Index deduplication has been applied yet.",
			total, total)
	}
}
