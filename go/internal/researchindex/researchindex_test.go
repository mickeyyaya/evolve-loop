package researchindex

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const staticHead = "# Research Index\n\nThe hand-written part.\n\n"

func project(t *testing.T, index string, entries map[string]string) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, IndexPath, index)
	for name, body := range entries {
		writeFile(t, root, EntriesDir+"/"+name, body)
	}
	return root
}

func writeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readIndex(t *testing.T, root string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, IndexPath))
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

const (
	entryA = "## 2026-10-21 — a\n\n| Document | What it records |\n|---|---|\n| [a](a.md) | A. |\n"
	entryB = "## 2026-10-21 — b\n\n| Document | What it records |\n|---|---|\n| [b](b.md) | B. |\n"
)

func TestGenerate_WritesEachEntryInNameOrderInsideTheRegion(t *testing.T) {
	root := project(t, staticHead, map[string]string{"2026-10-21-b.md": entryB, "2026-10-21-a.md": entryA, "notes.txt": "ignored"})

	changed, err := Generate(root)

	want := staticHead + Begin + "\n\n" + entryA + "\n" + entryB + "\n" + End + "\n"
	if err != nil || !changed || readIndex(t, root) != want {
		t.Fatalf("Generate = (%v, %v), index:\n%s\nwant:\n%s", changed, err, readIndex(t, root), want)
	}
	if changed, err := Generate(root); err != nil || changed {
		t.Errorf("a second Generate = (%v, %v), want no change", changed, err)
	}
	if err := Check(root); err != nil {
		t.Errorf("Check after Generate = %v, want nil", err)
	}
}

func TestCheck_NamesAnEntryWithNoIndexLineAndPassesAfterGenerate(t *testing.T) {
	root := project(t, staticHead, map[string]string{"2026-10-21-a.md": entryA})
	if _, err := Generate(root); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, EntriesDir+"/2026-10-21-b.md", entryB)

	err := Check(root)

	if !errors.Is(err, ErrStale) || !strings.Contains(err.Error(), "2026-10-21-b.md") || strings.Contains(err.Error(), "2026-10-21-a.md") {
		t.Fatalf("Check = %v, want ErrStale that names only the entry with no index line", err)
	}
	if _, err := Generate(root); err != nil {
		t.Fatal(err)
	}
	if err := Check(root); err != nil {
		t.Errorf("Check after Generate = %v, want nil", err)
	}
}

func TestCheck_AHandEditInsideTheRegionIsStale(t *testing.T) {
	root := project(t, staticHead, map[string]string{"2026-10-21-a.md": entryA})
	if _, err := Generate(root); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, IndexPath, strings.Replace(readIndex(t, root), "| A. |", "| edited |", 1))

	err := Check(root)

	if !errors.Is(err, ErrStale) || !strings.Contains(err.Error(), "evolve docs index generate") {
		t.Fatalf("Check = %v, want ErrStale that names the generator", err)
	}
}

func TestEntries_RefusesAnEntryWithNoHeadingAndAnUnreadableTree(t *testing.T) {
	root := project(t, staticHead, map[string]string{"2026-10-21-a.md": "no heading\n"})
	if _, err := Generate(root); err == nil || !strings.Contains(err.Error(), "2026-10-21-a.md") {
		t.Errorf("Generate = %v, want a refusal that names the entry with no heading", err)
	}
	if err := Check(root); err == nil || errors.Is(err, ErrStale) {
		t.Errorf("Check = %v, want the heading refusal, not a stale index", err)
	}
	empty := t.TempDir()
	if _, err := Generate(empty); err == nil {
		t.Error("Generate with no entry directory and no index must fail")
	}
	if err := Check(empty); err == nil {
		t.Error("Check with no entry directory and no index must fail")
	}
	noIndex := t.TempDir()
	writeFile(t, noIndex, EntriesDir+"/2026-10-21-a.md", entryA)
	if _, err := Generate(noIndex); err == nil {
		t.Error("Generate with no index file must fail")
	}
}

func TestEntries_ABadGlobPatternIsAnError(t *testing.T) {
	root := filepath.Join(t.TempDir(), "a[b")
	writeFile(t, root, IndexPath, staticHead)
	writeFile(t, root, EntriesDir+"/2026-10-21-a.md", entryA)
	if _, err := Generate(root); err == nil || !strings.Contains(err.Error(), "list the entries") {
		t.Errorf("Generate under a root with a glob character = %v, want a list error", err)
	}
}

func TestEntries_AnUnreadableEntryIsAnError(t *testing.T) {
	root := project(t, staticHead, map[string]string{"2026-10-21-a.md": entryA})
	if err := os.Mkdir(filepath.Join(root, EntriesDir, "2026-10-21-dir.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Generate(root); err == nil || !strings.Contains(err.Error(), "2026-10-21-dir.md") {
		t.Errorf("Generate = %v, want an error that names the unreadable entry", err)
	}
}

func TestGenerate_ABrokenRegionIsAnErrorAndAReadOnlyIndexFailsTheWrite(t *testing.T) {
	root := project(t, staticHead+Begin+"\nno end\n", map[string]string{"2026-10-21-a.md": entryA})
	if _, err := Generate(root); err == nil {
		t.Error("Generate over a region with no END marker must fail")
	}
	if err := Check(root); err == nil || errors.Is(err, ErrStale) {
		t.Errorf("Check = %v, want the splice error", err)
	}
	ro := project(t, staticHead, map[string]string{"2026-10-21-a.md": entryA})
	dir := filepath.Join(ro, filepath.Dir(IndexPath))
	if err := os.Chmod(filepath.Join(ro, IndexPath), 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	if _, err := Generate(ro); err == nil {
		t.Error("Generate over a read-only index must fail")
	}
}
