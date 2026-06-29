//go:build acs

// Package cycle417 materializes the cycle-417 acceptance criteria for two prompt-compaction tasks:
//   - router-catalog-prose-compaction (T1)
//   - reflector-reference-ondemand-split (T2)
//
// Goal: reduce per-agent token usage by (T1) compacting the verbose 66-row router catalog
// prose in-place and (T2) externalizing the evolve-reflector.md historical narrative tail
// via the proven on-demand-marker pattern.
//
// AC map (1:1 with scout-report.md top_n; R9.3 floor-binding):
//
//	router-catalog-prose-compaction (T1):
//	  AC1 "## Phase Catalog — Core Values" section < 8000B (was ~10507B)              → C417_001 (RED)
//	  AC2 all 66 phase rows retained (no row deleted by prose trim)                    → C417_002 (pre-existing GREEN)
//	  AC3 no empty-trigger rows in catalog after compaction (anti-gaming)              → C417_003 (pre-existing GREEN)
//
//	reflector-reference-ondemand-split (T2):
//	  AC1 evolve-reflector.md has line-anchored ## Reference Index heading             → C417_004 (RED)
//	  AC2 StripOnDemandSections saves ≥200 bytes from evolve-reflector.md body        → C417_005 (RED)
//	  AC3 required operational anchors survive above the ## Reference Index marker     → C417_006 (pre-existing GREEN)
//	  AC4 "## Why this agent exists" absent from stripped body (negative)              → C417_007 (RED)
//	  AC5 evolve-reflector-reference.md exists and is non-empty                       → C417_008 (RED)
//
// Adversarial diversity (SKILL §6):
//
//	Negative: empty trigger in catalog row → C417_003 (catches over-trim); "## Why this
//	          agent exists" in stripped body → C417_007 (catches missed marker placement);
//	          synthetic buried content unchanged → C417_NEG (pre-existing GREEN).
//	Edge/OOD: C417_002 — row count exactly 66, not ≥66 (catches deletion-by-compaction);
//	          C417_003 — trailing pipe stripped from trigger column.
//	Semantic: byte-delta (C417_001/C417_005) vs row-count (C417_002) vs trigger-content (C417_003)
//	          vs marker-presence (C417_004) vs anchor-survival (C417_006) vs section-absent (C417_007)
//	          vs stub-exists (C417_008) = 8 distinct dimensions.
//
// Deferred (zero predicates per R9.3): router catalog projection from phase-metadata SSOT
// (beyond-ask B1), per-agent prompt byte-budget ratchet (beyond-ask B2),
// envelope injection trim (beyond-ask B3).
//
// Floor binding (R9.3): predicates authored ONLY for tasks in the triage top_n.
// Deferred tasks get zero predicates.
//
// 1:1 enforcement:
//
//	T1: predicate=3 (C417_001–C417_003), manual+checklist=0, unverifiable-remove=0 → total AC=3 ✓
//	T2: predicate=5 (C417_004–C417_008), manual+checklist=0, unverifiable-remove=0 → total AC=5 ✓
//	Adversarial: C417_NEG (synthetic negative, pre-existing GREEN) → total=1 sentinel
package cycle417

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// =============================================================================
// Task T1 — router-catalog-prose-compaction
// =============================================================================

// TestC417_001_RouterCatalogSectionUnder8000Bytes asserts that the "## Phase Catalog —
// Core Values" section in evolve-router.md is <8000 bytes after prose compaction.
// BEHAVIORAL: measures the section byte weight directly from the file on disk.
//
// RED: current section is ~10507 bytes (want <8000B).
// Builder must compact per-row justification prose to a tight one-clause trigger per row.
func TestC417_001_RouterCatalogSectionUnder8000Bytes(t *testing.T) {
	root := acsassert.RepoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-router.md"))
	if err != nil {
		t.Fatalf("read evolve-router.md: %v", err)
	}
	body := string(raw)

	const heading = "## Phase Catalog — Core Values"
	idx := strings.Index(body, heading)
	if idx < 0 {
		t.Fatalf("evolve-router.md missing '## Phase Catalog — Core Values' section")
	}

	rest := body[idx+len(heading):]
	nextSection := strings.Index(rest, "\n## ")
	var sectionBytes int
	if nextSection < 0 {
		sectionBytes = len(heading) + len(rest)
	} else {
		sectionBytes = len(heading) + nextSection
	}

	const maxBytes = 8000
	if sectionBytes >= maxBytes {
		t.Errorf("RED: '## Phase Catalog — Core Values' is %d bytes (want <%d).\n"+
			"Builder must compact per-row justification prose to a one-clause trigger per row;\n"+
			"retain all 66 phase names verbatim. Current: %d bytes → target: <%d bytes.",
			sectionBytes, maxBytes, sectionBytes, maxBytes)
	}
}

// TestC417_002_RouterCatalog66RowsRetained asserts that the Phase Catalog retains exactly
// 66 phase rows — no row may be deleted during prose compaction.
// BEHAVIORAL: counts lines matching the backtick phase-name pattern in the catalog section.
//
// Pre-existing GREEN: catalog currently has 66 rows.
// Anti-gaming sentinel: catches any attempt to meet the byte target by deleting rows.
func TestC417_002_RouterCatalog66RowsRetained(t *testing.T) {
	root := acsassert.RepoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-router.md"))
	if err != nil {
		t.Fatalf("read evolve-router.md: %v", err)
	}
	body := string(raw)

	const heading = "## Phase Catalog — Core Values"
	idx := strings.Index(body, heading)
	if idx < 0 {
		t.Fatalf("evolve-router.md missing '## Phase Catalog — Core Values' section")
	}
	section := body[idx:]
	if next := strings.Index(section[len(heading):], "\n## "); next >= 0 {
		section = section[:len(heading)+next]
	}

	count := 0
	for _, line := range strings.Split(section, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "| `") && strings.Contains(trimmed, "` |") {
			count++
		}
	}

	const want = 66
	if count != want {
		t.Errorf("router catalog has %d phase rows (want %d) — prose compaction MUST NOT delete rows;\n"+
			"every phase name must be present verbatim after compaction", count, want)
	}
}

// TestC417_003_RouterCatalogNoEmptyTriggerRows_Negative asserts that no phase row in the
// catalog has an empty or whitespace-only second column (trigger) after compaction.
// BEHAVIORAL: parses each catalog data row and checks the trigger column.
//
// Pre-existing GREEN: all rows currently have substantive trigger text.
// Anti-gaming sentinel: catches over-trimming that leaves bare "| `name` | |" entries.
func TestC417_003_RouterCatalogNoEmptyTriggerRows_Negative(t *testing.T) {
	root := acsassert.RepoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-router.md"))
	if err != nil {
		t.Fatalf("read evolve-router.md: %v", err)
	}
	body := string(raw)

	const heading = "## Phase Catalog — Core Values"
	idx := strings.Index(body, heading)
	if idx < 0 {
		t.Fatalf("evolve-router.md missing '## Phase Catalog — Core Values' section")
	}
	section := body[idx:]
	if next := strings.Index(section[len(heading):], "\n## "); next >= 0 {
		section = section[:len(heading)+next]
	}

	for _, line := range strings.Split(section, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "| `") || !strings.Contains(trimmed, "` |") {
			continue
		}
		parts := strings.SplitN(trimmed, "` | ", 2)
		if len(parts) < 2 {
			t.Errorf("catalog row has no trigger column: %q", trimmed)
			continue
		}
		trigger := strings.TrimSuffix(strings.TrimSpace(parts[1]), " |")
		if strings.TrimSpace(trigger) == "" {
			t.Errorf("catalog row has empty trigger (second column blank): %q", trimmed)
		}
	}
}

// =============================================================================
// Task T2 — reflector-reference-ondemand-split
// =============================================================================

// TestC417_004_ReflectorMdHasCompactMarker asserts that evolve-reflector.md contains a
// line-anchored ## Reference Index heading so StripOnDemandSections fires on every
// reflection dispatch.
// BEHAVIORAL: scans body lines with the same prefix-match logic as StripOnDemandSections.
//
// RED: evolve-reflector.md body has no ## Reference Index heading → returns false.
func TestC417_004_ReflectorMdHasCompactMarker(t *testing.T) {
	root := acsassert.RepoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-reflector.md"))
	if err != nil {
		t.Fatalf("read evolve-reflector.md: %v", err)
	}
	_, body, err := prompts.ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	if !bodyHasCompactMarker(body) {
		t.Errorf("RED: evolve-reflector.md has no line-anchored ## Reference Index heading.\n" +
			"Builder must insert:\n" +
			"  ## Reference Index (Layer 3, on-demand)\n" +
			"above '## Why this agent exists' and move the narrative to evolve-reflector-reference.md.")
	}
}

// TestC417_005_ReflectorCompaction_SavesAtLeast200Bytes asserts that StripOnDemandSections
// applied to the real evolve-reflector.md body saves ≥200 bytes (the "## Why this agent exists"
// narrative is ~850 bytes; 200 is the conservative floor).
// BEHAVIORAL: calls prompts.StripOnDemandSections — the production SSOT.
//
// RED: no heading → 0 bytes stripped (0 < 200).
func TestC417_005_ReflectorCompaction_SavesAtLeast200Bytes(t *testing.T) {
	root := acsassert.RepoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-reflector.md"))
	if err != nil {
		t.Fatalf("read evolve-reflector.md: %v", err)
	}
	_, body, err := prompts.ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := prompts.StripOnDemandSections(body)
	saved := len(body) - len(stripped)
	if saved < 200 {
		t.Errorf("RED: reflector compaction saved only %d bytes (want ≥200).\n"+
			"Builder must place ## Reference Index (Layer 3, on-demand) before '## Why this agent exists'\n"+
			"so the ~850-byte narrative is stripped on every dispatch. (body=%d stripped=%d)",
			saved, len(body), len(stripped))
	}
}

// TestC417_006_ReflectorOperationalAnchors_AboveMarker asserts that required behavior-bearing
// sections survive prompts.StripOnDemandSections (remain above the ## Reference Index marker).
// Pre-existing GREEN: stripped==body (no heading) → all content present.
// Regression guard: fires if builder accidentally buries any of these below the marker.
func TestC417_006_ReflectorOperationalAnchors_AboveMarker(t *testing.T) {
	root := acsassert.RepoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-reflector.md"))
	if err != nil {
		t.Fatalf("read evolve-reflector.md: %v", err)
	}
	_, body, err := prompts.ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := prompts.StripOnDemandSections(body)
	for _, anchor := range []string{
		"## What NOT to do",
		"aggregate-reflections.sh",
		"## Ledger Entry",
		"Single-writer invariant",
		"## Core Principles",
	} {
		if !strings.Contains(stripped, anchor) {
			t.Errorf("required operational anchor %q lost below ## Reference Index — must remain above marker in evolve-reflector.md", anchor)
		}
	}
}

// TestC417_007_ReflectorNarrativeSection_AbsentAfterStrip_Negative asserts that the
// "## Why this agent exists" historical narrative is relocated BELOW the ## Reference Index
// marker and thus absent from the stripped body.
// NEGATIVE: verifies behavioral removal of narrative content after stripping.
//
// RED: stripped==body (no heading) → "## Why this agent exists" IS present in stripped → FAIL.
// GREEN after builder: narrative relocated below marker → absent in stripped.
func TestC417_007_ReflectorNarrativeSection_AbsentAfterStrip_Negative(t *testing.T) {
	root := acsassert.RepoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-reflector.md"))
	if err != nil {
		t.Fatalf("read evolve-reflector.md: %v", err)
	}
	_, body, err := prompts.ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := prompts.StripOnDemandSections(body)
	if strings.Contains(stripped, "## Why this agent exists") {
		t.Errorf("RED: '## Why this agent exists' still appears above ## Reference Index.\n" +
			"Builder must insert ## Reference Index (Layer 3, on-demand) ABOVE this section\n" +
			"so compaction removes the historical narrative. Move it to evolve-reflector-reference.md.")
	}
}

// TestC417_008_ReflectorReferenceStubExists asserts that agents/evolve-reflector-reference.md
// exists and is non-empty. The stub must carry the "## Why this agent exists" narrative.
// RED: file does not exist yet.
func TestC417_008_ReflectorReferenceStubExists(t *testing.T) {
	root := acsassert.RepoRoot(t)
	f := filepath.Join(root, "agents", "evolve-reflector-reference.md")
	if !acsassert.FileExists(t, f) {
		t.Errorf("RED: agents/evolve-reflector-reference.md does not exist.\n"+
			"Builder must create this stub and move '## Why this agent exists'\n"+
			"narrative from evolve-reflector.md into it (mirrors evolve-tdd-engineer-reference.md).\n"+
			"File: %s", f)
		return
	}
	info, err := os.Stat(f)
	if err != nil {
		t.Fatalf("stat evolve-reflector-reference.md: %v", err)
	}
	if info.Size() == 0 {
		t.Error("evolve-reflector-reference.md is empty — must carry the '## Why this agent exists' narrative")
	}
}

// =============================================================================
// Adversarial sentinel (pre-existing GREEN)
// =============================================================================

// TestC417_NEG_SyntheticBuriedContentRemoved asserts that content placed below
// ## Reference Index in a synthetic reflector body is correctly stripped.
// Pre-existing GREEN: the production StripOnDemandSections already handles this.
// Anti-gaming sentinel: if a no-op strip were introduced, this catches it.
func TestC417_NEG_SyntheticBuriedContentRemoved(t *testing.T) {
	body := "Operational rules.\n\n## What NOT to do\n\nDo not invent causes.\n\n" +
		"## Reference Index (Layer 3, on-demand)\n\n## Why this agent exists\n\nNarrative.\n"
	stripped := prompts.StripOnDemandSections(body)
	if stripped == body {
		t.Error("synthetic: strip was a no-op despite ## Reference Index heading — StripOnDemandSections broken")
	}
	if strings.Contains(stripped, "Narrative.") {
		t.Error("synthetic: content below ## Reference Index survived strip — StripOnDemandSections broken")
	}
	if !strings.Contains(stripped, "## What NOT to do") {
		t.Error("synthetic: '## What NOT to do' above marker was incorrectly stripped")
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
