package prompts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// intent_compaction_test.go — RED contract for cycle-416 task intent-ondemand-reference-tail.
//
// RED state (before builder):
//   - evolve-intent.md has no ## Reference Index heading → 0 bytes stripped (want ≥500)
//   - ## Composition and ## Reference sections appear in stripped body (want: absent after strip)

// TestIntentCompaction_SavesAtLeast500Bytes asserts that applying StripOnDemandSections to the
// real evolve-intent.md body saves ≥500 bytes — the minimum on-demand reference tail size.
// RED: evolve-intent.md has no ## Reference Index heading; StripOnDemandSections returns the
// body unchanged (0 bytes stripped, 0 < 500).
func TestIntentCompaction_SavesAtLeast500Bytes(t *testing.T) {
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
	if saved < 500 {
		t.Errorf("intent compaction saved only %d bytes (want ≥500); add ## Reference Index heading with reference-grade tail below it (body=%d stripped=%d)", saved, len(body), len(stripped))
	}
}

// TestIntentCompaction_OperationalAnchorsAboveMarker asserts that required behavior-bearing
// rules survive StripOnDemandSections (remain above the ## Reference Index marker).
// Pre-existing GREEN: stripped==body (no heading), so all content is present.
// Regression guard: fires if builder accidentally buries any of these anchors.
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
		"Output contract",
		"INTENT_MODE",
	} {
		if !strings.Contains(stripped, anchor) {
			t.Errorf("required operational anchor %q lost below ## Reference Index — must remain above marker in evolve-intent.md", anchor)
		}
	}
}

// TestIntentCompaction_ReferenceContentAbsentAfterStrip_Negative asserts that the reference-grade
// sections (## Composition, ## Reference) are relocated BELOW the ## Reference Index marker
// and thus absent from the stripped body.
// RED: stripped==body (no heading) → ## Composition IS in stripped → FAIL.
// GREEN after builder: sections relocated below marker → absent in stripped.
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
	for _, refSection := range []string{
		"## Composition",
	} {
		if strings.Contains(stripped, refSection) {
			t.Errorf("RED: reference section %q still appears above ## Reference Index — relocate it below the marker in evolve-intent.md", refSection)
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
