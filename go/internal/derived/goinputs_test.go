package derived

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"testing/fstest"
)

func TestCodeRegistrarDirs_ListsEachPackageThatRegistersACode(t *testing.T) {
	fsys := fstest.MapFS{
		"go/internal/a/a.go":                   {Data: []byte("package a\nfunc init() { signalcenter.RegisterCode(m, c, \"d\") }\n")},
		"go/internal/a/b.go":                   {Data: []byte("package a\nfunc init() { signalcenter.RegisterCode(m, d, \"d\") }\n")},
		"go/internal/b/b.go":                   {Data: []byte("package b\n")},
		"go/internal/c/c_test.go":              {Data: []byte("package c\nfunc init() { signalcenter.RegisterCode(m, c, \"d\") }\n")},
		"go/internal/d/notes.md":               {Data: []byte("RegisterCode(\n")},
		"go/internal/signalcenter/registry.go": {Data: []byte("package signalcenter\nfunc RegisterCode(m Module, c Code, doc string) {}\n")},
	}

	got, err := CodeRegistrarDirs(fsys)

	want := []string{"go/internal/a", "go/internal/signalcenter"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("CodeRegistrarDirs = (%v, %v), want (%v, nil)", got, err, want)
	}
}

func TestCodeRegistrarDirs_AnUnreadableTreeIsAnError(t *testing.T) {
	broken := t.TempDir()
	if err := os.MkdirAll(filepath.Join(broken, "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(broken, "missing.go"), filepath.Join(broken, "go", "dangling.go")); err != nil {
		t.Fatal(err)
	}
	for name, fsys := range map[string]fstest.MapFS{"no go directory": {"README.md": {}}} {
		if _, err := CodeRegistrarDirs(fsys); err == nil {
			t.Errorf("%s: CodeRegistrarDirs = nil error, want one", name)
		}
	}
	if _, err := CodeRegistrarDirs(os.DirFS(broken)); err == nil {
		t.Error("a dangling Go file: CodeRegistrarDirs = nil error, want one")
	}
}

func TestModuleDirs_KeepsOnlyTheMainModulePackages(t *testing.T) {
	listing := "\n/m/go/internal/b\t/m/go\n\n/m/go/internal/a\t/m/go\n"

	got := moduleDirs(listing)

	if want := []string{"go/internal/a", "go/internal/b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("moduleDirs = %v, want %v", got, want)
	}
}
