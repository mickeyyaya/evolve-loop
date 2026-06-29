package prompts

// cycle417_amplified_test.go — Adversarial amplification for cycle-417 tasks.
//
// Probes gaps NOT covered by:
//   router_compaction_test.go (3 tests: <8000B, 66 rows, no empty triggers),
//   reflector_compaction_test.go (6 tests: marker present, strip saves ≥200B,
//     operational anchors above marker, narrative absent after strip, reference stub
//     exists, synthetic anti-gaming).
//
// New adversarial angles:
//   Router:
//     - Section byte floor (≥4000B): prevents gaming via row over-deletion
//     - No duplicate phase names: catches row merging or silent deduplication
//     - Trigger minimum length (≥10 chars): catches over-trimming to meaningless stubs
//   Reflector:
//     - Stripped body floor (≥3000B): parallel to triage/tdd-engineer floor guards
//     - Real-doc idempotency: strip twice == strip once on the live file
//     - bodyHasCompactMarker gate recognizes reflector (canonical gate, not custom helper)
//     - Reference stub carries the narrative heading (stronger than size-only guard)
//     - Marker position: marker must appear AFTER "## What NOT to do" (position ordering)

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRouterCompaction_SectionByteFloor asserts that the "## Phase Catalog — Core Values"
// section in evolve-router.md is at least 4,000 bytes after compaction.
//
// Anti-gaming: the upper-bound test (<8000B) alone is insufficient — a builder could game
// the byte limit by deleting rows while inflating the remaining ones. A floor prevents
// the opposite extreme: 66 rows × ~60B minimum content ≈ 3960B.
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

// TestRouterCompaction_NoDuplicatePhaseNames asserts that all phase rows in the catalog
// have unique names. A builder could inadvertently create duplicates during prose trimming,
// or game row-count checks by duplicating compact rows while deleting others.
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

	seen := make(map[string]int) // name → first line index
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

// TestRouterCompaction_TriggerMinimumLength asserts no phase row has a trigger shorter
// than 10 characters. The build-report spec requires "tight one-clause triggers" — the
// existing no-empty-trigger test catches blank triggers, but over-trimming can produce
// triggers that are technically non-empty yet too short to carry dispatch information.
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

// TestReflectorStrippedBodyFloor asserts that the stripped evolve-reflector.md body
// retains at least 3,000 bytes — guarding against a misplaced ## Reference Index marker
// that would strip required operational content (Workflow, Ledger Entry, Core Principles).
// Parallel to TestTddEngineerStrippedBodyFloor and TestTriageStrippedBodyFloor (cycle-415).
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

// TestReflectorCompaction_RealDocIdempotent asserts that applying StripOnDemandSections
// twice to the real evolve-reflector.md body produces the same result as once.
// Distinct from TestReflectorCompaction_SyntheticBuriedNarrativeNegative (synthetic body);
// exercises the real "## Reference Index (Layer 3, on-demand)" heading in the live file.
// Parallel to TestIntentCompaction_RealDocIdempotent (cycle-416).
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

// TestReflectorHasCompactMarkerViaGate asserts that evolve-reflector.md is recognized by
// the canonical bodyHasCompactMarker gate — the same function used in
// TestAlwaysOnPhaseDocsHaveCompactMarker.
//
// Gap: reflector_compaction_test.go uses a local reflectorBodyHasCompactMarker helper with
// identical logic. This test ensures the canonical gate also accepts the marker, catching any
// future divergence between the two implementations.
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

// TestReflectorReferenceStubHasNarrativeHeading asserts that evolve-reflector-reference.md
// contains the "## Why this agent exists" heading — confirming the narrative was relocated
// into the stub rather than just creating a near-empty placeholder.
//
// Gap: TestReflectorReferenceStubExists (TDD contract) only checks existence + non-empty;
// this test verifies the stub carries the specific relocated content.
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

// TestReflectorCompaction_MarkerAfterOperationalSections asserts that the ## Reference Index
// marker in evolve-reflector.md appears AFTER "## What NOT to do" — position ordering guard.
//
// The build-report states: "Added ## Reference Index (Layer 3, on-demand) marker AFTER
// ## What NOT to do section (line 170)." If the marker was accidentally placed before this
// operational section, the section would be stripped on every dispatch.
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
