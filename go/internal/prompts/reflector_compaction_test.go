package prompts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// reflector_compaction_test.go — RED contract for cycle-417 task reflector-reference-ondemand-split.
//
// RED state (before builder):
//   - evolve-reflector.md has no ## Reference Index heading → StripOnDemandSections returns body unchanged
//   - "## Why this agent exists" historical narrative (lines ~172–179) is inline (not below marker)
//   - evolve-reflector-reference.md does not exist

// TestReflectorCompaction_MarkerPresent asserts that evolve-reflector.md contains a
// line-anchored ## Reference Index heading so StripOnDemandSections fires on every
// reflection dispatch.
// RED: evolve-reflector.md has no ## Reference Index heading.
func TestReflectorCompaction_MarkerPresent(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-reflector.md"))
	if err != nil {
		t.Fatalf("read evolve-reflector.md: %v", err)
	}
	_, body, err := ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	if !reflectorBodyHasCompactMarker(body) {
		t.Errorf("RED: evolve-reflector.md has no line-anchored ## Reference Index heading.\n" +
			"Builder must insert:\n" +
			"  ## Reference Index (Layer 3, on-demand)\n" +
			"above the '## Why this agent exists' section (line ~171) and move the narrative\n" +
			"into agents/evolve-reflector-reference.md — mirroring the triage/tdd-engineer pattern.")
	}
}

// TestReflectorCompaction_StripSavesBytes asserts that StripOnDemandSections applied to
// the real evolve-reflector.md body saves ≥200 bytes (the "## Why this agent exists"
// narrative is ~850 bytes; 200 is a conservative floor).
// RED: no heading → 0 bytes stripped (0 < 200).
func TestReflectorCompaction_StripSavesBytes(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-reflector.md"))
	if err != nil {
		t.Fatalf("read evolve-reflector.md: %v", err)
	}
	_, body, err := ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := StripOnDemandSections(body)
	saved := len(body) - len(stripped)
	if saved < 200 {
		t.Errorf("RED: reflector compaction saved only %d bytes (want ≥200).\n"+
			"Builder must place ## Reference Index (Layer 3, on-demand) before\n"+
			"'## Why this agent exists' so the narrative tail is stripped on every dispatch.\n"+
			"(body=%d stripped=%d)", saved, len(body), len(stripped))
	}
}

// TestReflectorCompaction_OperationalAnchorsAboveMarker asserts that required behavior-bearing
// sections survive StripOnDemandSections (remain above the ## Reference Index marker).
// Pre-existing GREEN: stripped==body (no heading) → all content present.
// Regression guard: fires if builder accidentally buries any of these below the marker.
func TestReflectorCompaction_OperationalAnchorsAboveMarker(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-reflector.md"))
	if err != nil {
		t.Fatalf("read evolve-reflector.md: %v", err)
	}
	_, body, err := ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := StripOnDemandSections(body)
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

// TestReflectorCompaction_NarrativeAbsentAfterStrip_Negative asserts that the
// "## Why this agent exists" historical narrative is relocated BELOW the ## Reference Index
// marker and thus absent from the stripped body.
// NEGATIVE: verifies behavioral removal of narrative content after stripping.
//
// RED: stripped==body (no heading) → "## Why this agent exists" IS present in stripped → FAIL.
// GREEN after builder: narrative relocated below marker → absent in stripped.
func TestReflectorCompaction_NarrativeAbsentAfterStrip_Negative(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-reflector.md"))
	if err != nil {
		t.Fatalf("read evolve-reflector.md: %v", err)
	}
	_, body, err := ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := StripOnDemandSections(body)
	if strings.Contains(stripped, "## Why this agent exists") {
		t.Errorf("RED: '## Why this agent exists' still appears above ## Reference Index.\n" +
			"Builder must insert ## Reference Index (Layer 3, on-demand) ABOVE this section\n" +
			"so compaction removes the historical narrative on every dispatch.\n" +
			"Move the narrative body to agents/evolve-reflector-reference.md.")
	}
}

// TestReflectorReferenceStubExists verifies that agents/evolve-reflector-reference.md
// exists and is non-empty. The stub must carry the "## Why this agent exists" narrative
// relocated from evolve-reflector.md, making it available for on-demand Layer 3 lookup.
// RED: file does not exist yet (written by builder as part of cycle-417).
func TestReflectorReferenceStubExists(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "agents", "evolve-reflector-reference.md")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("RED: evolve-reflector-reference.md does not exist: %v\n"+
			"Builder must create this file and move the '## Why this agent exists'\n"+
			"narrative from evolve-reflector.md into it — mirroring evolve-tdd-engineer-reference.md.", err)
	}
	if info.Size() == 0 {
		t.Error("evolve-reflector-reference.md is empty — must carry the 'Why this agent exists' narrative from evolve-reflector.md")
	}
}

// TestReflectorCompaction_SyntheticBuriedNarrativeNegative asserts that content placed
// below ## Reference Index in a synthetic reflector body does NOT appear in the stripped output.
// Anti-gaming sentinel: a no-op strip implementation would fail the byte-savings test.
// Pre-existing GREEN: StripOnDemandSections correctly handles this.
func TestReflectorCompaction_SyntheticBuriedNarrativeNegative(t *testing.T) {
	body := "Operational rules.\n\n## What NOT to do\n\nDo not invent causes.\n\n" +
		"## Reference Index (Layer 3, on-demand)\n\n## Why this agent exists\n\nHistorical narrative.\n"
	stripped := StripOnDemandSections(body)
	if stripped == body {
		t.Error("synthetic: strip was a no-op despite ## Reference Index heading")
	}
	if strings.Contains(stripped, "Historical narrative.") {
		t.Error("synthetic: content below ## Reference Index survived strip — StripOnDemandSections broken")
	}
	if !strings.Contains(stripped, "## What NOT to do") {
		t.Error("synthetic: '## What NOT to do' above marker was incorrectly stripped")
	}
}

// reflectorBodyHasCompactMarker mirrors prompts.StripOnDemandSections detection logic:
// returns true iff body contains a line that is exactly "## Reference Index"
// or starts with "## Reference Index " (space-suffixed form).
func reflectorBodyHasCompactMarker(body string) bool {
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimRight(line, "\r")
		if trimmed == "## Reference Index" || strings.HasPrefix(trimmed, "## Reference Index ") {
			return true
		}
	}
	return false
}
