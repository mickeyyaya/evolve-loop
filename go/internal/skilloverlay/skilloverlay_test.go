package skilloverlay

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

func TestMaterialize_EmptyNamesIsNoop(t *testing.T) {
	prefix, missing := Materialize(t.TempDir(), nil)
	if prefix != "" {
		t.Errorf("prefix = %q, want empty", prefix)
	}
	if len(missing) != 0 {
		t.Errorf("missing = %v, want none", missing)
	}
}

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
	if !strings.Contains(prefix, "fable") {
		t.Errorf("prefix does not name the skill:\n%s", prefix)
	}
}

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

func TestStripFrontmatter_EdgeCases(t *testing.T) {
	noClose := "---\nname: x\nbody line with no closing marker\n"
	if got := stripFrontmatter(noClose); got != noClose {
		t.Errorf("no-closing-delimiter must return unchanged, got %q", got)
	}
	got := stripFrontmatter("\n\n---\nname: x\n---\nBODY\n")
	if strings.Contains(got, "name: x") || !strings.Contains(got, "BODY") {
		t.Errorf("leading-blank frontmatter not stripped: %q", got)
	}
	body := "# Heading\ntext\n"
	if got := stripFrontmatter(body); got != body {
		t.Errorf("no-frontmatter must pass through, got %q", got)
	}
}

func TestMaterialize_PathTraversalNameYieldsMissing(t *testing.T) {
	dir := t.TempDir()
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
