package prompts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Despite the name, the floor is 256 bytes: the Composition and Reference tail.
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
	if saved < 256 {
		t.Errorf("intent compaction saved only %d bytes (want ≥256: the Composition+Reference tail); ## Reference Index heading missing? (body=%d stripped=%d)", saved, len(body), len(stripped))
	}
}

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

// Also pins the operational content that must survive stripping.
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
	for _, absent := range []string{"## Composition"} {
		if strings.Contains(stripped, absent) {
			t.Errorf("on-demand content %q still appears in stripped body — relocate it below ## Reference Index in evolve-intent.md", absent)
		}
	}
	for _, kept := range []string{
		"intent-delta.md",
		"## Re-run behavior",
		"intent-reflection.yaml",
	} {
		if !strings.Contains(stripped, kept) {
			t.Errorf("operational content %q lost below ## Reference Index — it must survive into dispatched intent prompts", kept)
		}
	}
}

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
