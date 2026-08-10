package prompts

// compact_marker_gate_test.go — RED contract for cycle-415 tasks (tdd/triage marker) and
// cycle-422 triage threshold raise (triage-prompt-reference-index-expansion).
//
// RED state (before builder):
//   - evolve-tdd-engineer.md has no ## Reference Index heading → 0 bytes stripped (want ≥1500)
//   - evolve-triage.md has no ## Reference Index heading → 0 bytes stripped (want ≥1200)
//   - evolve-triage.md's versioned-historical sections appear in stripped body (want: absent after strip)
//   - TestAlwaysOnPhaseDocsHaveCompactMarker fails for tdd-engineer + triage (no heading)

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTddEngineerCompaction asserts that evolve-tdd-engineer.md has a line-anchored
// ## Reference Index heading that enables ≥1500 bytes of compaction, and that required
// behavior anchors survive above the heading.
// RED: heading absent → 0 bytes stripped (0 < 1500).
func TestTddEngineerCompaction(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-tdd-engineer.md"))
	if err != nil {
		t.Fatalf("read evolve-tdd-engineer.md: %v", err)
	}
	_, body, err := ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := StripOnDemandSections(body)
	saved := len(body) - len(stripped)
	// Floor recalibrated 2026-08-10 (was 1500): the old floor required the full
	// Predicate Quality Requirements section (REQUIRED reading) to sit below the
	// strip marker, deleting it from every dispatched tdd prompt with only the
	// above-marker summary anchors surviving (persona-strip lobotomy incident).
	// The marker now sits at EOF.
	if saved < 64 {
		t.Errorf("tdd-engineer compaction saved only %d bytes (want ≥64: the marker section itself); ## Reference Index heading missing? (body=%d stripped=%d)", saved, len(body), len(stripped))
	}
	// Behavior anchors must remain above the ## Reference Index marker.
	// Pre-existing GREEN: with no heading, stripped==body → all anchors present.
	// Regression guard: fires if builder accidentally buries one of these.
	for _, anchor := range []string{
		"RED phase is proof of understanding",
		"Do NOT implement production code",
		"15-turn boundary",
		"challenge-token",
		// cycle-85 anti-degenerate-predicate safeguard: cycle 415 buried the full
		// "Predicate Quality Requirements" section below the marker (compact mode
		// strips it). The above-marker REQUIRED summary must survive compaction so
		// the tdd-engineer never authors grep-only predicates blind to this rule.
		"MUST exercise the system under test",
		"degenerate-predicate failure mode",
	} {
		if !strings.Contains(stripped, anchor) {
			t.Errorf("required anchor %q lost below ## Reference Index — must stay above marker in evolve-tdd-engineer.md", anchor)
		}
	}
}

// TestTddEngineerCompaction_BuriedRuleNegative asserts the anti-gaming guard:
// a required rule buried below ## Reference Index in a synthetic body must NOT
// appear in the stripped output.
// Pre-existing GREEN: StripOnDemandSections correctly removes content below the heading.
func TestTddEngineerCompaction_BuriedRuleNegative(t *testing.T) {
	body := "Preamble content.\n\n## Reference Index\n\nDo NOT implement production code\n"
	stripped := StripOnDemandSections(body)
	if stripped == body {
		t.Error("synthetic: strip was a no-op despite ## Reference Index heading")
	}
	if strings.Contains(stripped, "Do NOT implement production code") {
		t.Error("synthetic: rule buried below ## Reference Index survived strip — StripOnDemandSections broken")
	}
}

// TestTriageCompaction asserts that evolve-triage.md has a line-anchored ## Reference Index
// heading enabling ≥4200 bytes of compaction, required output sections and gate-bearing
// rules survive above the heading, and versioned-historical subsections are relocated below it.
// Cycle-415 RED: heading absent → 0 bytes stripped (0 < 1200).
// Cycle-422 RED: ~3209B currently stripped; 3209 < 4200 → FAIL (threshold raised).
func TestTriageCompaction(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-triage.md"))
	if err != nil {
		t.Fatalf("read evolve-triage.md: %v", err)
	}
	_, body, err := ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := StripOnDemandSections(body)
	saved := len(body) - len(stripped)
	// Floor recalibrated 2026-08-10 (was 4200): the old floor REQUIRED burying
	// the inbox-ingestion and idempotency-skip-list instructions — operational
	// directives misclassified as "versioned-historical" because their headings
	// carry version tags — below the strip marker, deleting them from every
	// dispatched triage prompt (docs/incidents/2026-08-10-persona-strip-lobotomy.md;
	// the queue-starvation mechanism). The marker now sits at EOF; what may be
	// stripped is governed by phasecoherence/persona_strip_operational_test.go.
	if saved < 64 {
		t.Errorf("triage compaction saved only %d bytes (want ≥64: the marker section itself); ## Reference Index heading missing? (body=%d stripped=%d)", saved, len(body), len(stripped))
	}
	// Required output sections and gate-bearing rules must survive strip —
	// including the two the old "versioned-historical" negative wrongly forced
	// below the marker.
	for _, section := range []string{
		"## top_n",
		"## deferred",
		"## dropped",
		"carryoverTodos",
		"## Rationale",
		"Operator-queue priority floor",
		"Idempotency skip-list",
		"Inbox ingestion",
	} {
		if !strings.Contains(stripped, section) {
			t.Errorf("required section/rule %q lost below ## Reference Index — must stay above marker in evolve-triage.md", section)
		}
	}
}

// TestAlwaysOnPhaseDocsHaveCompactMarker asserts every always-on phase doc carries a
// line-anchored ## Reference Index heading so StripOnDemandSections fires every cycle.
// RED: evolve-tdd-engineer.md and evolve-triage.md lack the heading.
func TestAlwaysOnPhaseDocsHaveCompactMarker(t *testing.T) {
	root := repoRoot(t)
	for _, name := range []string{
		"evolve-tdd-engineer",
		"evolve-triage",
		"evolve-orchestrator",
		"evolve-auditor",
		"evolve-builder",
		"evolve-scout",
	} {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(root, "agents", name+".md"))
			if err != nil {
				t.Fatalf("read %s.md: %v", name, err)
			}
			_, body, err := ParseFrontmatter(string(raw))
			if err != nil {
				t.Fatalf("parse frontmatter: %v", err)
			}
			if !bodyHasCompactMarker(body) {
				t.Errorf("%s.md has no line-anchored ## Reference Index heading — add it so StripOnDemandSections compacts it on every cycle dispatch", name)
			}
		})
	}
}

// TestAlwaysOnPhaseDocsHaveCompactMarker_InlineMentionRejected asserts that an inline
// prose mention of ## Reference Index does NOT satisfy the marker gate.
// Negative / anti-gaming: a naive strings.Contains check would accept inline mentions.
// Pre-existing GREEN: bodyHasCompactMarker uses the same line-anchored logic as StripOnDemandSections.
func TestAlwaysOnPhaseDocsHaveCompactMarker_InlineMentionRejected(t *testing.T) {
	body := "See ## Reference Index below for details.\nMore content.\n"
	if bodyHasCompactMarker(body) {
		t.Error("inline prose mention of ## Reference Index counted as compact marker — gate must be line-anchored")
	}
}

// bodyHasCompactMarker mirrors StripOnDemandSections's detection logic:
// returns true iff body contains a line-anchored ## Reference Index heading.
func bodyHasCompactMarker(body string) bool {
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimRight(line, "\r")
		if trimmed == "## Reference Index" || strings.HasPrefix(trimmed, "## Reference Index ") {
			return true
		}
	}
	return false
}
