package skilloverlay

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeSkill creates skillsDir/<name>/SKILL.md with the given content.
func writeSkill(t *testing.T, skillsDir, name, content string) {
	t.Helper()
	dir := filepath.Join(skillsDir, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}
}

// TestMaterialize_EmptyNamesIsNoop: no configured skills ⇒ empty prefix, no
// missing — a phase with no overlay is byte-identical to pre-feature behavior.
func TestMaterialize_EmptyNamesIsNoop(t *testing.T) {
	prefix, missing := Materialize(t.TempDir(), nil)
	if prefix != "" {
		t.Errorf("prefix = %q, want empty", prefix)
	}
	if len(missing) != 0 {
		t.Errorf("missing = %v, want none", missing)
	}
}

// TestMaterialize_ReadsPersonaBodyAndStripsFrontmatter: the injected block
// carries the skill's persona body but NOT its YAML frontmatter (the
// description is meta-instruction about the skill, not persona for the agent).
func TestMaterialize_ReadsPersonaBodyAndStripsFrontmatter(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "fable", "---\nname: fable\ndescription: meta about when to use\n---\n\n# Fable discipline\n\nEVIDENCE before opinion.\n")

	prefix, missing := Materialize(dir, []string{"fable"})
	if len(missing) != 0 {
		t.Fatalf("missing = %v, want none", missing)
	}
	if !strings.Contains(prefix, "EVIDENCE before opinion.") {
		t.Errorf("prefix missing persona body:\n%s", prefix)
	}
	if !strings.Contains(prefix, "# Fable discipline") {
		t.Errorf("prefix missing persona heading:\n%s", prefix)
	}
	if strings.Contains(prefix, "description: meta about when to use") {
		t.Errorf("prefix leaked YAML frontmatter:\n%s", prefix)
	}
	// The block must name the skill so an operator reading the prompt knows
	// what was preloaded and where it starts/ends.
	if !strings.Contains(prefix, "fable") {
		t.Errorf("prefix does not name the skill:\n%s", prefix)
	}
}

// TestMaterialize_MissingSkillReportedNotSilent: a configured skill whose
// SKILL.md is absent is reported in `missing` (so the caller WARNs loudly),
// never silently dropped, and does not abort the other skills.
func TestMaterialize_MissingSkillReportedNotSilent(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "present", "# Present\nbody\n")

	prefix, missing := Materialize(dir, []string{"present", "absent"})
	if len(missing) != 1 || missing[0] != "absent" {
		t.Fatalf("missing = %v, want [absent]", missing)
	}
	if !strings.Contains(prefix, "body") {
		t.Errorf("present skill must still be materialized despite a missing sibling:\n%s", prefix)
	}
}

// TestMaterialize_OrderPreserved: multiple skills concatenate in the given
// order (the policy resolver's stable order is authoritative).
func TestMaterialize_OrderPreserved(t *testing.T) {
	dir := t.TempDir()
	writeSkill(t, dir, "first", "# first\nAAA\n")
	writeSkill(t, dir, "second", "# second\nBBB\n")

	prefix, _ := Materialize(dir, []string{"first", "second"})
	iA := strings.Index(prefix, "AAA")
	iB := strings.Index(prefix, "BBB")
	if iA < 0 || iB < 0 || iA > iB {
		t.Errorf("order not preserved (AAA@%d, BBB@%d):\n%s", iA, iB, prefix)
	}
}

// TestStripFrontmatter_EdgeCases covers the malformed (no closing delimiter),
// leading-blank-line, and no-frontmatter branches so the persona body is
// preserved verbatim when there is nothing safe to strip.
func TestStripFrontmatter_EdgeCases(t *testing.T) {
	// Frontmatter opened but never closed → return unchanged (don't eat the body).
	noClose := "---\nname: x\nbody line with no closing marker\n"
	if got := stripFrontmatter(noClose); got != noClose {
		t.Errorf("no-closing-delimiter must return unchanged, got %q", got)
	}
	// Leading blank lines before the opening delimiter are skipped.
	got := stripFrontmatter("\n\n---\nname: x\n---\nBODY\n")
	if strings.Contains(got, "name: x") || !strings.Contains(got, "BODY") {
		t.Errorf("leading-blank frontmatter not stripped: %q", got)
	}
	// No frontmatter at all → passthrough.
	body := "# Heading\ntext\n"
	if got := stripFrontmatter(body); got != body {
		t.Errorf("no-frontmatter must pass through, got %q", got)
	}
}

// TestMaterialize_PathTraversalNameYieldsMissing: a name is a single registry
// entry, never a path fragment — a traversal-y name must not resolve a file
// outside skillsDir; it simply reports missing (defense-in-depth; the policy
// clamp already rejects non-registry names, this is the second layer).
func TestMaterialize_PathTraversalNameYieldsMissing(t *testing.T) {
	dir := t.TempDir()
	// plant a file one level up that a traversal name could target
	if err := os.WriteFile(filepath.Join(filepath.Dir(dir), "SKILL.md"), []byte("# escaped\n"), 0o644); err != nil {
		t.Fatalf("plant: %v", err)
	}
	prefix, missing := Materialize(dir, []string{"../"})
	if strings.Contains(prefix, "escaped") {
		t.Errorf("path traversal escaped skillsDir:\n%s", prefix)
	}
	if len(missing) != 1 {
		t.Errorf("missing = %v, want the traversal name reported missing", missing)
	}
}
