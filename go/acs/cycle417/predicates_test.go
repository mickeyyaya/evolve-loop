//go:build acs

package cycle417

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

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

func bodyHasCompactMarker(body string) bool {
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimRight(line, "\r")
		if trimmed == "## Reference Index" || strings.HasPrefix(trimmed, "## Reference Index ") {
			return true
		}
	}
	return false
}
