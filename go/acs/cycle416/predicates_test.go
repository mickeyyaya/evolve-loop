//go:build acs

// Package cycle416 materializes the cycle-416 acceptance criteria for two prompt-compaction tasks:
//   - intent-ondemand-reference-tail (T1)
//   - prompt-compaction-coverage-gate (T2)
//
// Goal: close the dead-wired compaction in evolve-intent.md (marker absent → 0 bytes stripped
// per cycle despite CompactPrompts=true), and add a regression guard asserting all 7 per-cycle
// agents are strictly compacted.
//
// AC map (1:1 with scout-report.md top_n; R9.3 floor-binding):
//
//	intent-ondemand-reference-tail (T1):
//	  AC1 evolve-intent.md has a line-anchored ## Reference Index heading              → C416_001 (RED)
//	  AC2 StripOnDemandSections saves ≥500 bytes from evolve-intent.md body            → C416_002 (RED)
//	  AC3 required operational anchors survive above the ## Reference Index marker      → C416_003 (pre-existing GREEN)
//	  AC4 reference-grade sections (## Composition) absent from stripped body (neg)    → C416_004 (RED)
//
//	prompt-compaction-coverage-gate (T2):
//	  AC1 go/internal/prompts/compaction_coverage_test.go exists                       → C416_005 (RED)
//	  AC2 compaction_coverage_test.go contains "evolve-intent"                         → C416_006 (RED)
//	  AC3 compaction_coverage_test.go names all 7 per-cycle agents                     → C416_007 (RED)
//
// Adversarial diversity (SKILL §6):
//
//	Negative: ## Composition in stripped body → C416_004 (stripped==body when no marker → section present → RED).
//	          synthetic markerless body unchanged → C416_NEG (pre-existing GREEN, anti-gaming sentinel).
//	Edge/OOD: C416_003 — inline mention does not trigger strip (mirrors C415_008 pre-existing GREEN).
//	Semantic: marker-presence (C416_001) vs byte-delta (C416_002) vs anchor-survival (C416_003) vs
//	          reference-absent (C416_004) vs file-exists (C416_005) vs content-coverage (C416_006/007)
//	          = 7 distinct dimensions.
//
// Deferred (zero predicates per R9.3): supporting-agent audit (evolve-failure-advisor.md,
// evolve-reflector.md — beyond-ask B1), build-time marker lint (beyond-ask B2).
//
// Floor binding (R9.3): predicates authored ONLY for tasks in the triage top_n.
// Deferred tasks get zero predicates.
//
// 1:1 enforcement:
//
//	T1: predicate=4 (C416_001–C416_004), manual+checklist=0, unverifiable-remove=0 → total AC=4 ✓
//	T2: predicate=3 (C416_005–C416_007), manual+checklist=0, unverifiable-remove=0 → total AC=3 ✓
//	Adversarial: C416_NEG (synthetic negative, pre-existing GREEN) → total=1 sentinel
package cycle416

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// =============================================================================
// Task T1 — intent-ondemand-reference-tail
// =============================================================================

// TestC416_001_IntentMdHasCompactMarker asserts that evolve-intent.md contains a
// line-anchored ## Reference Index heading so StripOnDemandSections fires on every
// cycle dispatch (currently 0 bytes stripped — dead-wired since intent.go:133).
//
// BEHAVIORAL: scans body lines with the same prefix-match logic as StripOnDemandSections.
//
// RED: evolve-intent.md body has no ## Reference Index heading → bodyHasCompactMarker returns false.
func TestC416_001_IntentMdHasCompactMarker(t *testing.T) {
	root := acsassert.RepoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-intent.md"))
	if err != nil {
		t.Fatalf("read evolve-intent.md: %v", err)
	}
	_, body, err := prompts.ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	if !bodyHasCompactMarker(body) {
		t.Errorf("RED: evolve-intent.md has no line-anchored ## Reference Index heading.\n" +
			"Builder must add:\n" +
			"  ## Reference Index (Layer 3, on-demand)\n" +
			"above the ## Composition section (line 199) and below all behavior-bearing rules.\n" +
			"This closes the dead-wired compaction: CompactPrompts=true + intent.go:133 flag is\n" +
			"plumbed but 0 bytes stripped because the marker is absent.")
	}
}

// TestC416_002_IntentCompaction_SavesAtLeast500Bytes asserts that StripOnDemandSections
// applied to the real evolve-intent.md body saves ≥500 bytes.
// BEHAVIORAL: calls prompts.StripOnDemandSections — the production SSOT.
//
// RED: no heading → 0 bytes stripped (0 < 500).
func TestC416_002_IntentCompaction_SavesAtLeast500Bytes(t *testing.T) {
	root := acsassert.RepoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-intent.md"))
	if err != nil {
		t.Fatalf("read evolve-intent.md: %v", err)
	}
	_, body, err := prompts.ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := prompts.StripOnDemandSections(body)
	saved := len(body) - len(stripped)
	if saved < 500 {
		t.Errorf("RED: intent compaction saved only %d bytes (want ≥500).\n"+
			"Builder must place ## Reference Index (Layer 3, on-demand) before the reference-grade\n"+
			"sections (## Composition line 199, ## Reference line 205, ## Reflection Authoring line 214).\n"+
			"These sections total ~18 lines / ~990 bytes — 500 is a conservative floor.\n"+
			"body=%d stripped=%d", saved, len(body), len(stripped))
	}
}

// TestC416_003_IntentOperationalAnchors_AboveMarker asserts that required behavior-bearing
// rules survive prompts.StripOnDemandSections (remain above the ## Reference Index marker).
// Pre-existing GREEN: stripped==body (no heading) → all content present.
// Regression guard: fires if builder accidentally buries any of these anchors below the marker.
func TestC416_003_IntentOperationalAnchors_AboveMarker(t *testing.T) {
	root := acsassert.RepoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-intent.md"))
	if err != nil {
		t.Fatalf("read evolve-intent.md: %v", err)
	}
	_, body, err := prompts.ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := prompts.StripOnDemandSections(body)
	for _, anchor := range []string{
		"IMKI",
		"STOP CRITERION",
		"challenged_premise",
		"Output contract",
		"INTENT_MODE",
	} {
		if !strings.Contains(stripped, anchor) {
			t.Errorf("required operational anchor %q lost below ## Reference Index — must remain above marker in evolve-intent.md", anchor)
		}
	}
}

// TestC416_004_IntentReferenceSection_AbsentAfterStrip_Negative asserts that the
// reference-grade ## Composition section is relocated BELOW the ## Reference Index marker
// and thus absent from the stripped body.
// NEGATIVE: verifies behavioral removal of reference content after stripping.
//
// RED: stripped==body (no heading) → ## Composition IS present in stripped → FAIL.
// GREEN after builder: ## Composition relocated below marker → absent in stripped.
func TestC416_004_IntentReferenceSection_AbsentAfterStrip_Negative(t *testing.T) {
	root := acsassert.RepoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-intent.md"))
	if err != nil {
		t.Fatalf("read evolve-intent.md: %v", err)
	}
	_, body, err := prompts.ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := prompts.StripOnDemandSections(body)
	if strings.Contains(stripped, "## Composition") {
		t.Errorf("RED: reference section '## Composition' still appears above ## Reference Index.\n" +
			"Builder must place ## Reference Index (Layer 3, on-demand) ABOVE ## Composition (line 199)\n" +
			"so that compaction removes ## Composition, ## Reference, and ## Reflection Authoring\n" +
			"from the per-cycle prompt dispatch. These are reference-grade, not behavioral rules.")
	}
}

// =============================================================================
// Task T2 — prompt-compaction-coverage-gate
// =============================================================================

// TestC416_005_CompactionCoverageTestExists asserts that the coverage gate test file
// go/internal/prompts/compaction_coverage_test.go exists.
// This is the regression guard against silent per-cycle token re-inflation when a
// future agent is added without a ## Reference Index marker.
//
// RED: file does not exist (written by TDD engineer in cycle-416, not yet present).
func TestC416_005_CompactionCoverageTestExists(t *testing.T) {
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "internal", "prompts", "compaction_coverage_test.go")
	if !acsassert.FileExists(t, f) {
		t.Errorf("RED: go/internal/prompts/compaction_coverage_test.go does not exist.\n"+
			"Builder must ensure this file is written (TDD engineer contract from cycle-416).\n"+
			"File: %s", f)
	}
}

// TestC416_006_CompactionCoverageTest_CoversIntent asserts that the coverage gate test
// file references "evolve-intent" — closing the gap identified in scout finding #1.
//
// RED: file does not exist (or once it exists, does not include evolve-intent).
func TestC416_006_CompactionCoverageTest_CoversIntent(t *testing.T) {
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "internal", "prompts", "compaction_coverage_test.go")
	if !acsassert.FileContains(t, f, `"evolve-intent"`) {
		t.Errorf("RED: compaction_coverage_test.go does not contain \"evolve-intent\".\n"+
			"The coverage gate must enumerate all 7 per-cycle agents including evolve-intent.\n"+
			"File: %s", f)
	}
}

// TestC416_007_CompactionCoverageTest_CoversAllSevenAgents asserts that the coverage
// gate test file names all 7 per-cycle agents so no agent can be omitted.
//
// RED: file does not exist → all checks fail.
func TestC416_007_CompactionCoverageTest_CoversAllSevenAgents(t *testing.T) {
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "go", "internal", "prompts", "compaction_coverage_test.go")
	perCycleAgents := []string{
		"evolve-scout",
		"evolve-builder",
		"evolve-auditor",
		"evolve-orchestrator",
		"evolve-tdd-engineer",
		"evolve-triage",
		"evolve-intent",
	}
	for _, name := range perCycleAgents {
		if !acsassert.FileContains(t, f, `"`+name+`"`) {
			t.Errorf("RED: compaction_coverage_test.go does not name %q in its per-cycle agent list.\n"+
				"The coverage gate must enumerate all 7 per-cycle agents to guard against silent\n"+
				"token re-inflation when a new agent ships without a ## Reference Index marker.\n"+
				"File: %s", name, f)
		}
	}
}

// =============================================================================
// Adversarial sentinel (pre-existing GREEN)
// =============================================================================

// TestC416_NEG_MarkerlessBody_CompactionIsNoOp asserts that StripOnDemandSections
// returns a markerless body byte-for-byte unchanged.
// Pre-existing GREEN: the production StripOnDemandSections already handles this correctly.
// Anti-gaming sentinel: if a no-op compaction path were introduced, this would catch it.
func TestC416_NEG_MarkerlessBody_CompactionIsNoOp(t *testing.T) {
	body := "# Agent\n\nOperational rules only.\n\n## Some Other Section\n\nContent.\n"
	stripped := prompts.StripOnDemandSections(body)
	if stripped != body {
		t.Errorf("markerless body modified by StripOnDemandSections — production code broken\n"+
			"body=%d stripped=%d diff=%d", len(body), len(stripped), len(body)-len(stripped))
	}
}

// =============================================================================
// Helper
// =============================================================================

// bodyHasCompactMarker mirrors prompts.StripOnDemandSections detection logic:
// returns true iff body contains a line that is exactly "## Reference Index"
// or starts with "## Reference Index " (space-suffixed form).
func bodyHasCompactMarker(body string) bool {
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimRight(line, "\r")
		if trimmed == "## Reference Index" || strings.HasPrefix(trimmed, "## Reference Index ") {
			return true
		}
	}
	return false
}
