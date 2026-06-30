package prompts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// intent_compaction_test.go — RED contract updated for cycle-422 task intent-prompt-reference-index-expansion.
//
// Cycle-416 original state: heading absent → 0 bytes stripped (want ≥500);
//   ## Composition and ## Reference appear in stripped body.
// Cycle-422 updated state:
//   - ## Output contract (INTENT_MODE), ## Re-run behavior, ## Reflection Authoring relocated below marker
//   - Byte floor raised from ≥500 to ≥2200 (from ~755B currently stripped)
//   - "Output contract" and "INTENT_MODE" removed from must-survive anchors (they move below marker);
//     "30-80 line" and "EMERGENCY EXIT" added (confirmed above marker)

// TestIntentCompaction_SavesAtLeast2200Bytes asserts that applying StripOnDemandSections to the
// real evolve-intent.md body saves ≥2200 bytes — the cycle-422 floor after relocating
// ## Output contract (INTENT_MODE), ## Re-run behavior, and ## Reflection Authoring below the marker.
// RED (cycle-422): currently only ~755B stripped; 755 < 2200 → FAIL.
func TestIntentCompaction_SavesAtLeast2200Bytes(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-intent.md"))
	if err != nil {
		t.Fatalf("read evolve-intent.md: %v", err)
	}
	_, body, err := ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := StripOnDemandSections(body)
	saved := len(body) - len(stripped)
	if saved < 2200 {
		t.Errorf("intent compaction saved only %d bytes (want ≥2200); relocate ## Output contract (INTENT_MODE), ## Re-run behavior, ## Reflection Authoring below ## Reference Index in evolve-intent.md (body=%d stripped=%d)", saved, len(body), len(stripped))
	}
}

// TestIntentCompaction_OperationalAnchorsAboveMarker asserts that required every-cycle
// behavior-bearing anchors survive StripOnDemandSections (remain above the ## Reference Index
// marker) in evolve-intent.md.
// Pre-existing GREEN: all listed anchors are above the marker in current file.
// Regression guard: fires if builder accidentally buries any of these anchors below the marker.
// NOTE (cycle-422): "Output contract" and "INTENT_MODE" removed — those sections are
// intentionally relocated below the marker by the cycle-422 builder task. "30-80 line"
// and "EMERGENCY EXIT" added per cycle-422 AC5.
func TestIntentCompaction_OperationalAnchorsAboveMarker(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-intent.md"))
	if err != nil {
		t.Fatalf("read evolve-intent.md: %v", err)
	}
	_, body, err := ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := StripOnDemandSections(body)
	for _, anchor := range []string{
		"IMKI",
		"STOP CRITERION",
		"challenged_premise",
		"30-80 line",
		"EMERGENCY EXIT",
	} {
		if !strings.Contains(stripped, anchor) {
			t.Errorf("required operational anchor %q lost below ## Reference Index — must remain above marker in evolve-intent.md", anchor)
		}
	}
}

// TestIntentCompaction_ReferenceContentAbsentAfterStrip_Negative asserts that reference-grade
// and on-demand sections are relocated BELOW the ## Reference Index marker and thus absent
// from the stripped body.
// Cycle-416: ## Composition (already below marker — pre-existing GREEN after cycle-416).
// Cycle-422 (RED): ## Output contract (INTENT_MODE), ## Re-run behavior, ## Reflection Authoring
// are still above the marker → their unique identifiers appear in stripped body → FAIL.
// GREEN after builder: sections relocated below marker → identifiers absent in stripped.
func TestIntentCompaction_ReferenceContentAbsentAfterStrip_Negative(t *testing.T) {
	root := repoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "agents", "evolve-intent.md"))
	if err != nil {
		t.Fatalf("read evolve-intent.md: %v", err)
	}
	_, body, err := ParseFrontmatter(string(raw))
	if err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	stripped := StripOnDemandSections(body)
	for _, absent := range []string{
		"## Composition",
		// cycle-422: these must move below the marker
		"intent-delta.md",        // unique to ## Output contract (INTENT_MODE)
		"## Re-run behavior",     // heading of the re-run section
		"intent-reflection.yaml", // unique to ## Reflection Authoring (v10.20.0+)
	} {
		if strings.Contains(stripped, absent) {
			t.Errorf("RED: on-demand content %q still appears in stripped body — relocate it below ## Reference Index in evolve-intent.md", absent)
		}
	}
}

// TestIntentCompaction_BuriedRuleNegative asserts that a required rule placed below
// ## Reference Index in a synthetic body does NOT appear in the stripped output.
// Anti-gaming: a no-op strip implementation would pass the byte-savings test via
// moving operational content below the marker.
// Pre-existing GREEN: StripOnDemandSections correctly removes below-marker content.
func TestIntentCompaction_BuriedRuleNegative(t *testing.T) {
	body := "Preamble operational content.\n\n## Reference Index\n\nchallenged_premise rule buried here\n"
	stripped := StripOnDemandSections(body)
	if stripped == body {
		t.Error("synthetic: strip was a no-op despite ## Reference Index heading")
	}
	if strings.Contains(stripped, "challenged_premise rule buried here") {
		t.Error("synthetic: rule buried below ## Reference Index survived strip — StripOnDemandSections broken")
	}
}
