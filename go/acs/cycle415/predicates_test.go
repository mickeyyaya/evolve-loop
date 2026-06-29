//go:build acs

// Package cycle415 materializes the cycle-415 acceptance criteria for three prompt-compaction tasks:
//   - tdd-prompt-reference-index-tail (Task 1)
//   - triage-prompt-reference-index-tail (Task 2)
//   - compact-marker-presence-gate (Task 3)
//
// Goal: add line-anchored ## Reference Index headings to evolve-tdd-engineer.md and
// evolve-triage.md so the shipped StripOnDemandSections compaction fires on every cycle
// (currently 0 bytes stripped from the two largest always-on phase docs).
//
// AC map (1:1 with scout-report.md top_n; R9.3 floor-binding):
//
//	tdd-prompt-reference-index-tail:
//	  AC1 stripped body ≥1500 B smaller than full tdd-engineer.md body               → C415_001 (RED)
//	  AC2 required behavior anchors survive above the ## Reference Index marker        → C415_002 (pre-existing GREEN)
//	  AC3 synthetic: buried rule absent from stripped body (negative/anti-gaming)      → C415_003 (pre-existing GREEN)
//
//	triage-prompt-reference-index-tail:
//	  AC1 stripped body ≥1200 B smaller than full triage.md body                      → C415_004 (RED)
//	  AC2 required output sections + carryover rule survive strip                      → C415_005 (pre-existing GREEN)
//	  AC3 versioned-historical sections absent from stripped body (negative — RED)     → C415_006 (RED)
//
//	compact-marker-presence-gate:
//	  AC1 all 6 always-on phase docs have a line-anchored ## Reference Index heading   → C415_007 (RED)
//	  AC2 inline prose mention does NOT satisfy the gate (negative)                    → C415_008 (pre-existing GREEN)
//
// Adversarial diversity (per SKILL §6):
//
//	Negative: synthetic buried rule → C415_003; inline prose mention → C415_008; versioned sections in stripped → C415_006.
//	Edge/OOD: heading at EOF (StripOnDemandSections covered by cycle-413); inline mention mid-body → C415_008.
//	Semantic: byte-delta (C415_001/C415_004) vs anchor-presence (C415_002/C415_005) vs gate (C415_007) are distinct behaviors.
//
// Deferred (zero predicates per R9.3): multi-marker config-driven strip (beyond-ask #2),
// report-size caps, orchestrator.md under-compaction (993 B).
package cycle415

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// ─────────────────────────────────────────────────────────────────────────────
// Task 1: tdd-prompt-reference-index-tail
// ─────────────────────────────────────────────────────────────────────────────

// TestC415_001_TddEngineerStripsAtLeast1500Bytes asserts that applying
// prompts.StripOnDemandSections to the real evolve-tdd-engineer.md body saves
// ≥1500 bytes — the minimum on-demand reference tail size required per AC.
// RED: evolve-tdd-engineer.md has no ## Reference Index heading; StripOnDemandSections
// returns the body unchanged (0 bytes stripped).
func TestC415_001_TddEngineerStripsAtLeast1500Bytes(t *testing.T) {
	root := acsassert.RepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "agents", "evolve-tdd-engineer.md"))
	if err != nil {
		t.Fatalf("read evolve-tdd-engineer.md: %v", err)
	}
	_, body, err := prompts.ParseFrontmatter(string(data))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := prompts.StripOnDemandSections(body)
	saved := len(body) - len(stripped)
	if saved < 1500 {
		t.Errorf("RED: tdd-engineer compaction saved only %d bytes (want ≥1500); add ## Reference Index heading with on-demand content below it (body=%d stripped=%d)",
			saved, len(body), len(stripped))
	}
}

// TestC415_002_TddEngineerBehaviorAnchorsAboveMarker asserts that the required
// behavior-bearing rules survive prompts.StripOnDemandSections (remain above marker).
// Pre-existing GREEN: stripped==body (no heading), so all content is "above" the marker.
// Regression guard: fires if builder accidentally buries any of these anchors.
func TestC415_002_TddEngineerBehaviorAnchorsAboveMarker(t *testing.T) {
	root := acsassert.RepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "agents", "evolve-tdd-engineer.md"))
	if err != nil {
		t.Fatalf("read evolve-tdd-engineer.md: %v", err)
	}
	_, body, err := prompts.ParseFrontmatter(string(data))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := prompts.StripOnDemandSections(body)
	for _, anchor := range []string{
		"RED phase is proof of understanding",
		"Do NOT implement production code",
		"15-turn boundary",
		"challenge-token",
	} {
		if !strings.Contains(stripped, anchor) {
			t.Errorf("required anchor %q lost below ## Reference Index — must remain above marker in evolve-tdd-engineer.md", anchor)
		}
	}
}

// TestC415_003_TddEngineerBuriedRuleNegative asserts that a rule placed below
// ## Reference Index in a synthetic body does NOT appear in the stripped output.
// Anti-gaming: a no-op implementation passes C415_001 only by being mistaken for GREEN.
// Pre-existing GREEN: prompts.StripOnDemandSections correctly strips below-marker content.
func TestC415_003_TddEngineerBuriedRuleNegative(t *testing.T) {
	body := "Preamble content.\n\n## Reference Index\n\nDo NOT implement production code\n"
	stripped := prompts.StripOnDemandSections(body)
	if stripped == body {
		t.Error("synthetic: strip was a no-op despite ## Reference Index heading")
	}
	if strings.Contains(stripped, "Do NOT implement production code") {
		t.Error("synthetic: rule buried below ## Reference Index survived strip — StripOnDemandSections broken")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Task 2: triage-prompt-reference-index-tail
// ─────────────────────────────────────────────────────────────────────────────

// TestC415_004_TriageStripsAtLeast1200Bytes asserts that applying
// prompts.StripOnDemandSections to the real evolve-triage.md body saves ≥1200 bytes.
// RED: evolve-triage.md has no ## Reference Index heading; 0 bytes stripped.
func TestC415_004_TriageStripsAtLeast1200Bytes(t *testing.T) {
	root := acsassert.RepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "agents", "evolve-triage.md"))
	if err != nil {
		t.Fatalf("read evolve-triage.md: %v", err)
	}
	_, body, err := prompts.ParseFrontmatter(string(data))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := prompts.StripOnDemandSections(body)
	saved := len(body) - len(stripped)
	if saved < 1200 {
		t.Errorf("RED: triage compaction saved only %d bytes (want ≥1200); add ## Reference Index heading (body=%d stripped=%d)",
			saved, len(body), len(stripped))
	}
}

// TestC415_005_TriageRequiredSectionsAboveMarker asserts that required output sections
// and gate-bearing rules survive prompts.StripOnDemandSections (remain above marker).
// Pre-existing GREEN: stripped==body (no heading); all content is present.
// Regression guard: fires if builder buries a required section.
func TestC415_005_TriageRequiredSectionsAboveMarker(t *testing.T) {
	root := acsassert.RepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "agents", "evolve-triage.md"))
	if err != nil {
		t.Fatalf("read evolve-triage.md: %v", err)
	}
	_, body, err := prompts.ParseFrontmatter(string(data))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := prompts.StripOnDemandSections(body)
	for _, section := range []string{
		"## top_n",
		"## deferred",
		"## dropped",
		"carryoverTodos",
		"## Rationale",
		"Operator-queue priority floor",
	} {
		if !strings.Contains(stripped, section) {
			t.Errorf("required section/rule %q lost below ## Reference Index — must stay above marker in evolve-triage.md", section)
		}
	}
}

// TestC415_006_TriageVersionedSectionsAboveMarker_Negative asserts that
// versioned-historical subsections are relocated BELOW the ## Reference Index marker
// (absent from stripped body).
// RED: stripped==body (no heading) → versioned sections ARE present in stripped → FAIL.
// GREEN after builder: sections relocated below marker → absent in stripped.
func TestC415_006_TriageVersionedSectionsAboveMarker_Negative(t *testing.T) {
	root := acsassert.RepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "agents", "evolve-triage.md"))
	if err != nil {
		t.Fatalf("read evolve-triage.md: %v", err)
	}
	_, body, err := prompts.ParseFrontmatter(string(data))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := prompts.StripOnDemandSections(body)
	for _, versioned := range []string{
		"Idempotency skip-list (v9.6.0+)",
		"Inbox ingestion (v9.5.0+)",
		"Reflection Authoring (v10.20.0+)",
	} {
		if strings.Contains(stripped, versioned) {
			t.Errorf("RED: versioned-historical %q still appears above ## Reference Index — relocate it below the marker in evolve-triage.md", versioned)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Task 3: compact-marker-presence-gate
// ─────────────────────────────────────────────────────────────────────────────

// TestC415_007_AllAlwaysOnDocsHaveCompactMarker asserts every always-on phase doc
// carries a line-anchored ## Reference Index heading so StripOnDemandSections fires
// on every cycle dispatch.
// RED: evolve-tdd-engineer.md and evolve-triage.md lack the heading.
func TestC415_007_AllAlwaysOnDocsHaveCompactMarker(t *testing.T) {
	root := acsassert.RepoRoot(t)
	alwaysOn := []string{
		"evolve-tdd-engineer",
		"evolve-triage",
		"evolve-orchestrator",
		"evolve-auditor",
		"evolve-builder",
		"evolve-scout",
	}
	for _, name := range alwaysOn {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(root, "agents", name+".md"))
			if err != nil {
				t.Fatalf("read %s.md: %v", name, err)
			}
			_, body, err := prompts.ParseFrontmatter(string(data))
			if err != nil {
				t.Fatalf("parse frontmatter: %v", err)
			}
			if !bodyHasCompactMarker(body) {
				t.Errorf("RED: %s.md has no line-anchored ## Reference Index heading; add it so StripOnDemandSections compacts it every cycle", name)
			}
		})
	}
}

// TestC415_008_InlineMentionNotCountedAsMarker asserts that an inline prose mention
// of ## Reference Index does NOT satisfy the compact-marker gate.
// Negative / anti-gaming: a naive strings.Contains check would accept inline mentions
// (the cycle-413 strip fix demonstrated this failure mode already).
// Pre-existing GREEN: bodyHasCompactMarker mirrors StripOnDemandSections's line-anchored logic.
func TestC415_008_InlineMentionNotCountedAsMarker(t *testing.T) {
	body := "See ## Reference Index below for details.\nMore content.\n"
	if bodyHasCompactMarker(body) {
		t.Error("inline prose mention of ## Reference Index counted as compact marker — gate must be line-anchored")
	}
}

// bodyHasCompactMarker mirrors prompts.StripOnDemandSections detection logic:
// returns true iff body contains a line that is exactly "## Reference Index"
// or starts with "## Reference Index " (space-prefixed suffix form).
func bodyHasCompactMarker(body string) bool {
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimRight(line, "\r")
		if trimmed == "## Reference Index" || strings.HasPrefix(trimmed, "## Reference Index ") {
			return true
		}
	}
	return false
}
