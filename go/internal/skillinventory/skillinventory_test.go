package skillinventory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// writeSkill drops a SKILL.md under <root>/skills/<name>/SKILL.md with
// the given frontmatter and body.
func writeSkill(t *testing.T, root, name, frontmatter string) {
	t.Helper()
	dir := filepath.Join(root, "skills", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\n" + frontmatter + "\n---\n# " + name + "\nbody\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readInventory(t *testing.T, path string) Inventory {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var inv Inventory
	if err := json.Unmarshal(b, &inv); err != nil {
		t.Fatal(err)
	}
	return inv
}

// TestBuild_HappyPath_WritesInventoryFile — basic write-then-read.
func TestBuild_HappyPath_WritesInventoryFile(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "alpha", `name: alpha
description: first
categories: [testing, python]`)
	writeSkill(t, root, "beta", `name: beta
description: second
categories: [testing]`)

	res, err := Build(Options{ProjectRoot: root, Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.SkillCount != 2 {
		t.Errorf("SkillCount=%d, want 2", res.SkillCount)
	}

	inv := readInventory(t, res.OutputPath)
	if inv.SkillCount != 2 {
		t.Errorf("inv.SkillCount=%d, want 2", inv.SkillCount)
	}
	// Skills sorted alphabetically
	if len(inv.Skills) != 2 || inv.Skills[0].Name != "alpha" || inv.Skills[1].Name != "beta" {
		t.Errorf("Skills not sorted: %+v", inv.Skills)
	}
	if inv.CategoryIndex["testing"] == nil || len(inv.CategoryIndex["testing"]) != 2 {
		t.Errorf("CategoryIndex[testing] missing both skills: %v", inv.CategoryIndex["testing"])
	}
	if inv.CategoryIndex["python"] == nil || inv.CategoryIndex["python"][0] != "alpha" {
		t.Errorf("CategoryIndex[python] missing alpha: %v", inv.CategoryIndex["python"])
	}
}

// TestBuild_MissingProjectRoot_ReturnsError — required field check.
func TestBuild_MissingProjectRoot_ReturnsError(t *testing.T) {
	_, err := Build(Options{})
	if err == nil {
		t.Error("Build with empty ProjectRoot: want error")
	}
}

// TestBuild_NoSkillsDir_EmptyInventory — fresh project without skills/
// produces an empty inventory, not an error.
func TestBuild_NoSkillsDir_EmptyInventory(t *testing.T) {
	root := t.TempDir()
	res, err := Build(Options{ProjectRoot: root, Force: true})
	if err != nil {
		t.Fatal(err)
	}
	inv := readInventory(t, res.OutputPath)
	if inv.SkillCount != 0 {
		t.Errorf("expected empty inventory, got %d skills", inv.SkillCount)
	}
}

// TestBuild_UncategorizedSkill — skills without frontmatter categories
// land in the "uncategorized" bucket so they're still discoverable.
func TestBuild_UncategorizedSkill(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "loner", `name: loner`)
	res, err := Build(Options{ProjectRoot: root, Force: true})
	if err != nil {
		t.Fatal(err)
	}
	inv := readInventory(t, res.OutputPath)
	if list := inv.CategoryIndex["uncategorized"]; len(list) != 1 || list[0] != "loner" {
		t.Errorf("uncategorized bucket missing loner: %v", list)
	}
}

// TestBuild_StringCategoriesParsed — comma-separated categories string
// is parsed equivalently to a YAML list.
func TestBuild_StringCategoriesParsed(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "csv", `name: csv
categories: testing, golang, ci`)
	res, err := Build(Options{ProjectRoot: root, Force: true})
	if err != nil {
		t.Fatal(err)
	}
	inv := readInventory(t, res.OutputPath)
	wantCats := []string{"ci", "golang", "testing"}
	got := []string{}
	for cat := range inv.CategoryIndex {
		if cat == "uncategorized" {
			continue
		}
		got = append(got, cat)
	}
	sort.Strings(got)
	if len(got) != 3 {
		t.Fatalf("want 3 categories, got %v", got)
	}
	for i, w := range wantCats {
		if got[i] != w {
			t.Errorf("category[%d]=%q, want %q", i, got[i], w)
		}
	}
}

// TestBuild_CacheHit_SkipsRebuild — fresh cache means no rebuild.
func TestBuild_CacheHit_SkipsRebuild(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "alpha", `name: alpha`)

	t0 := time.Unix(1_700_000_000, 0)
	now := t0
	clock := func() time.Time { return now }

	res, err := Build(Options{ProjectRoot: root, NowFn: clock, TTL: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if res.CacheHit {
		t.Error("first build should not be a cache hit")
	}

	// Advance clock by 30min — still under TTL.
	now = t0.Add(30 * time.Minute)
	res2, err := Build(Options{ProjectRoot: root, NowFn: clock, TTL: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if !res2.CacheHit {
		t.Error("second build (30min later) should be cache hit")
	}
}

// TestBuild_CacheStale_Rebuilds — past-TTL cache triggers rebuild.
func TestBuild_CacheStale_Rebuilds(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "alpha", `name: alpha`)

	// First build at t0.
	_, _ = Build(Options{ProjectRoot: root})

	// Set the output file's mtime far into the past.
	out := filepath.Join(root, ".evolve", "skill-inventory.json")
	staleTime := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(out, staleTime, staleTime); err != nil {
		t.Fatal(err)
	}

	res, err := Build(Options{ProjectRoot: root, TTL: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if res.CacheHit {
		t.Error("stale cache should trigger rebuild")
	}
}

// TestBuild_ForceFlag_BypassesCache — --force always rebuilds.
func TestBuild_ForceFlag_BypassesCache(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "alpha", `name: alpha`)
	_, _ = Build(Options{ProjectRoot: root})

	res, err := Build(Options{ProjectRoot: root, Force: true, TTL: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if res.CacheHit {
		t.Error("Force=true should bypass cache hit")
	}
}

// TestBuild_AtomicWrite_NoPartialFile — a successful Build leaves
// either a complete file or the previous version, never a half-written
// one. We can't easily simulate a crash mid-write, but we verify the
// output is valid JSON after every successful call.
func TestBuild_AtomicWrite_NoPartialFile(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "alpha", `name: alpha`)
	res, err := Build(Options{ProjectRoot: root, Force: true})
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(res.OutputPath)
	if err != nil {
		t.Fatal(err)
	}
	var inv Inventory
	if err := json.Unmarshal(b, &inv); err != nil {
		t.Errorf("output is not valid JSON: %v", err)
	}
}

// TestBuild_SkillsPathIsFile_ScanError — when <root>/skills exists but is a
// regular file (not a directory), the loader's ReadDir fails with a non-
// IsNotExist error (ENOTDIR), so scan propagates it and Build wraps it as a
// "scan" error rather than emitting an empty inventory.
func TestBuild_SkillsPathIsFile_ScanError(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "skills"), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Build(Options{ProjectRoot: root, Force: true})
	if err == nil {
		t.Fatal("Build with skills-as-file: want scan error")
	}
	if !strings.Contains(err.Error(), "skillinventory: scan:") {
		t.Errorf("error %q must be wrapped as a scan failure", err)
	}
}

// TestBuild_OutputPathIsDir_WriteError — when the destination
// .evolve/skill-inventory.json is itself a directory, atomicwrite's final
// os.Rename fails, and Build surfaces it as a wrapped "write" error.
func TestBuild_OutputPathIsDir_WriteError(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "alpha", `name: alpha`)
	// Pre-create .evolve/skill-inventory.json as a directory so MkdirAll(.evolve)
	// still succeeds but the rename-onto-it cannot.
	outDir := filepath.Join(root, ".evolve", "skill-inventory.json")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := Build(Options{ProjectRoot: root, Force: true})
	if err == nil {
		t.Fatal("Build with output path as a directory: want write error")
	}
	if !strings.Contains(err.Error(), "skillinventory: write:") {
		t.Errorf("error %q must be wrapped as a write failure", err)
	}
}

// TestExtractCategories_NoCategoriesField — frontmatter without
// categories returns nil (caller maps to "uncategorized").
func TestExtractCategories_NoCategoriesField(t *testing.T) {
	if got := extractCategories(map[string]any{"name": "x"}); got != nil {
		t.Errorf("got %v, want nil", got)
	}
}

// TestExtractCategories_EmptyStrings_Skipped — empty entries don't
// leak into the inventory.
func TestExtractCategories_EmptyStrings_Skipped(t *testing.T) {
	got := extractCategories(map[string]any{"categories": []any{"a", "", "b"}})
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("got %v, want [a b]", got)
	}
}

// TestExtractCategories_NonStringField — defensive: unsupported types
// degrade to nil rather than panic.
func TestExtractCategories_NonStringField(t *testing.T) {
	if got := extractCategories(map[string]any{"categories": 42}); got != nil {
		t.Errorf("got %v, want nil", got)
	}
}

// TestExtractCategories_StringSliceFormat — the common path where the
// prompts parser hands back []string from inline `[a, b]` arrays.
func TestExtractCategories_StringSliceFormat(t *testing.T) {
	got := extractCategories(map[string]any{"categories": []string{"a", "b"}})
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("got %v, want [a b]", got)
	}
}

// TestExtractCategories_EmptyStringSlice_ReturnsNil — empty []string
// degrades to nil so caller maps to "uncategorized".
func TestExtractCategories_EmptyStringSlice_ReturnsNil(t *testing.T) {
	if got := extractCategories(map[string]any{"categories": []string{}}); got != nil {
		t.Errorf("got %v, want nil", got)
	}
	if got := extractCategories(map[string]any{"categories": []string{"", ""}}); got != nil {
		t.Errorf("all-empty got %v, want nil", got)
	}
}

// TestExtractCategories_AllEmptyAny_ReturnsNil — same for []any path.
func TestExtractCategories_AllEmptyAny_ReturnsNil(t *testing.T) {
	if got := extractCategories(map[string]any{"categories": []any{"", ""}}); got != nil {
		t.Errorf("got %v, want nil", got)
	}
}

// TestExtractCategories_EmptyString_ReturnsNil — empty string variant.
func TestExtractCategories_EmptyString_ReturnsNil(t *testing.T) {
	if got := extractCategories(map[string]any{"categories": ""}); got != nil {
		t.Errorf("got %v, want nil", got)
	}
	if got := extractCategories(map[string]any{"categories": ", ,"}); got != nil {
		t.Errorf("only-separators got %v, want nil", got)
	}
}

// TestBuild_MkdirFails_ReturnsError — when .evolve cannot be created
// (parent is a file), Build surfaces the error.
func TestBuild_MkdirFails_ReturnsError(t *testing.T) {
	root := t.TempDir()
	// Block creation of <root>/.evolve by writing a file there.
	if err := os.WriteFile(filepath.Join(root, ".evolve"), []byte("blocked"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeSkill(t, root, "alpha", `name: alpha`)
	_, err := Build(Options{ProjectRoot: root, Force: true})
	if err == nil {
		t.Error("Build with un-creatable .evolve dir: want error")
	}
}
