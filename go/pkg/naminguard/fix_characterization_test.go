package naminguard

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func readFile(t *testing.T, dir, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestFixCharacterization_SkipsBinaryFiles(t *testing.T) {
	dir := initRepo(t)
	binary := "\x00" + deadSlug() + "\n"
	writeTracked(t, dir, "blob.dat", binary)
	writeTracked(t, dir, "text.md", deadSlug()+"\n")

	changed, err := Fix(dir, testManifest())
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(changed, []string{"text.md"}) {
		t.Errorf("changed = %q, want [text.md]", changed)
	}
	if got := readFile(t, dir, "blob.dat"); got != binary {
		t.Errorf("binary file rewritten to %q", got)
	}
}

func TestFixCharacterization_ReportsChangedPathsSorted(t *testing.T) {
	dir := initRepo(t)
	for _, rel := range []string{"c.md", "a.md", "b/z.md", "b/a.md"} {
		writeTracked(t, dir, rel, "x "+deadSlug()+"\n")
	}

	changed, err := Fix(dir, testManifest())
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"a.md", "b/a.md", "b/z.md", "c.md"}; !slices.Equal(changed, want) {
		t.Errorf("changed = %q, want %q", changed, want)
	}
}

func TestFixCharacterization_AppliesEntriesInManifestOrder(t *testing.T) {
	dir := initRepo(t)
	writeTracked(t, dir, "f.md", "foo\n")
	m := &Manifest{Forbidden: []Forbidden{
		{Token: "foo", Replacement: "bar"},
		{Token: "bar", Replacement: "baz"},
	}}

	changed, err := Fix(dir, m)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(changed, []string{"f.md"}) {
		t.Errorf("changed = %q, want [f.md]", changed)
	}
	if got := readFile(t, dir, "f.md"); got != "baz\n" {
		t.Errorf("f.md = %q, want %q", got, "baz\n")
	}
}

func TestFixCharacterization_ReadFailureKeepsCause(t *testing.T) {
	orig := gitGrep
	t.Cleanup(func() { gitGrep = orig })
	gitGrep = func(root string, args ...string) (string, int, error) {
		return "missing.md\n", 0, nil
	}

	changed, err := Fix(t.TempDir(), testManifest())
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("err = %v, want one wrapping fs.ErrNotExist", err)
	}
	if changed != nil {
		t.Errorf("changed = %q, want nil on error", changed)
	}
}

func TestFixCharacterization_WriteFailureKeepsCause(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir := initRepo(t)
	writeTracked(t, dir, "ro/f.md", deadSlug()+"\n")
	roDir := filepath.Join(dir, "ro")
	if err := os.Chmod(roDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(roDir, 0o755) })

	changed, err := Fix(dir, testManifest())
	if !errors.Is(err, fs.ErrPermission) {
		t.Errorf("err = %v, want one wrapping fs.ErrPermission", err)
	}
	if changed != nil {
		t.Errorf("changed = %q, want nil on error", changed)
	}
}
