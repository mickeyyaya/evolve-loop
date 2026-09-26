package prompts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRouterCompaction_SectionByteFloor(t *testing.T) {
	root := repoRoot(t)
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

	const minBytes = 4000
	if sectionBytes < minBytes {
		t.Errorf("'## Phase Catalog — Core Values' section is only %d bytes (floor=%d); "+
			"compaction must NOT delete rows — 66 rows × ~60B minimum ≈ 3960B",
			sectionBytes, minBytes)
	}
}

func TestRouterCompaction_NoDuplicatePhaseNames(t *testing.T) {
	root := repoRoot(t)
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
	if nextSection := strings.Index(section[len(heading):], "\n## "); nextSection >= 0 {
		section = section[:len(heading)+nextSection]
	}

	seen := make(map[string]int)
	for i, line := range strings.Split(section, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "| `") || !strings.Contains(trimmed, "` |") {
			continue
		}
		inner := trimmed[3:] // skip "| `"
		end := strings.Index(inner, "`")
		if end < 0 {
			continue
		}
		name := inner[:end]
		if prev, dup := seen[name]; dup {
			t.Errorf("duplicate phase name %q: first seen at line %d, repeated at line %d",
				name, prev+1, i+1)
		} else {
			seen[name] = i
		}
	}
}

func TestRouterCompaction_TriggerMinimumLength(t *testing.T) {
	root := repoRoot(t)
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
	if nextSection := strings.Index(section[len(heading):], "\n## "); nextSection >= 0 {
		section = section[:len(heading)+nextSection]
	}

	const minTriggerLen = 10
	for _, line := range strings.Split(section, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "| `") || !strings.Contains(trimmed, "` |") {
			continue
		}
		parts := strings.SplitN(trimmed, "` | ", 2)
		if len(parts) < 2 {
			continue
		}
		trigger := strings.TrimSuffix(strings.TrimSpace(parts[1]), " |")
		trigger = strings.TrimSpace(trigger)
		if len(trigger) < minTriggerLen {
			t.Errorf("row trigger too short (%d chars, want ≥%d): %q — over-trimming removed dispatch guidance",
				len(trigger), minTriggerLen, line)
		}
	}
}

func TestReflectorStrippedBodyFloor(t *testing.T) {
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
	const minFloor = 3000
	if len(stripped) < minFloor {
		t.Errorf("reflector stripped body only %d bytes (floor=%d) — required operational "+
			"content (Workflow, Ledger Entry, Core Principles) may have been moved below ## Reference Index",
			len(stripped), minFloor)
	}
}

func TestReflectorCompaction_RealDocIdempotent(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-reflector.md"))
	if err != nil {
		t.Fatalf("read evolve-reflector.md: %v", err)
	}
	_, body, err := ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	once := StripOnDemandSections(body)
	twice := StripOnDemandSections(once)
	if once != twice {
		t.Errorf("StripOnDemandSections not idempotent on real evolve-reflector.md: "+
			"first=%d bytes, second=%d bytes", len(once), len(twice))
	}
}

func TestReflectorHasCompactMarkerViaGate(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-reflector.md"))
	if err != nil {
		t.Fatalf("read evolve-reflector.md: %v", err)
	}
	_, body, err := ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	if !bodyHasCompactMarker(body) {
		t.Error("evolve-reflector.md not recognized by canonical bodyHasCompactMarker — " +
			"## Reference Index heading is missing or malformed")
	}
}

func TestReflectorReferenceStubHasNarrativeHeading(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-reflector-reference.md"))
	if err != nil {
		t.Fatalf("read evolve-reflector-reference.md: %v", err)
	}
	if !strings.Contains(string(raw), "## Why this agent exists") {
		t.Error("evolve-reflector-reference.md missing '## Why this agent exists' heading — " +
			"stub must carry the relocated narrative from evolve-reflector.md, not be a placeholder")
	}
}

func TestReflectorCompaction_MarkerAfterOperationalSections(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-reflector.md"))
	if err != nil {
		t.Fatalf("read evolve-reflector.md: %v", err)
	}
	body := string(raw)

	const operationalAnchor = "## What NOT to do"
	const markerHeading = "## Reference Index"

	opIdx := strings.Index(body, operationalAnchor)
	if opIdx < 0 {
		t.Fatalf("evolve-reflector.md missing '%s' section", operationalAnchor)
	}
	markerIdx := strings.Index(body, markerHeading)
	if markerIdx < 0 {
		t.Fatalf("evolve-reflector.md missing '## Reference Index' marker")
	}
	if markerIdx <= opIdx {
		t.Errorf("## Reference Index marker (byte %d) must appear AFTER '%s' (byte %d) — "+
			"operational sections must remain above the strip marker to survive compaction",
			markerIdx, operationalAnchor, opIdx)
	}
}
