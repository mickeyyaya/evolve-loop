package prompts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

// reflectorBodyHasCompactMarker must agree with StripOnDemandSections on what counts as the heading.
func reflectorBodyHasCompactMarker(body string) bool {
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimRight(line, "\r")
		if trimmed == "## Reference Index" || strings.HasPrefix(trimmed, "## Reference Index ") {
			return true
		}
	}
	return false
}
