package changedpkgs

import (
	"errors"
	"go/ast"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestImportsTarget_SkipsMalformedSpecsAndMatchesExactPathsOnly(t *testing.T) {
	targets := map[string]string{"example.com/m/internal/foo": "internal/foo"}
	file := &ast.File{Imports: []*ast.ImportSpec{
		nil,
		{Path: nil},
		{Path: &ast.BasicLit{Value: "unquoted/path"}},
		{Path: &ast.BasicLit{Value: `"example.com/m/internal/foobar"`}},
	}}
	if importsTarget(file, targets) {
		t.Fatal("malformed specs and a longer package name must not match")
	}
	file.Imports = append(file.Imports, &ast.ImportSpec{Path: &ast.BasicLit{Value: `"example.com/m/internal/foo"`}})
	if !importsTarget(file, targets) {
		t.Fatal("the exact import path must match after the malformed ones")
	}
	if importsTarget(&ast.File{}, targets) {
		t.Fatal("a file without imports imports nothing")
	}
}

func TestImportersOf_IgnoresProductDirsUnparseableFilesAndUnreadableSubtrees(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	const imp = "package p\n\nimport _ \"example.com/m/internal/foo\"\n"
	write("internal/real/real.go", imp)
	write("internal/input/input.go", imp)
	write("acs/cycle1/p.go", imp)
	write("vendor/v/v.go", imp)
	write("internal/x/testdata/t.go", imp)
	write("node_modules/n/n.go", imp)
	write("internal/broken/broken.go", "package p\nimport (")
	write("internal/notes/readme.md", imp)
	write("locked/locked.go", imp)
	locked := filepath.Join(root, "locked")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	got := importersOf(root, map[string]string{"example.com/m/internal/foo": "internal/foo"}, map[string]struct{}{"internal/input": {}})

	want := map[string]struct{}{"internal/real": {}}
	if _, err := os.ReadDir(locked); err != nil {
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("importers = %v, want %v", got, want)
		}
		return
	}
	want["locked"] = struct{}{}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("importers = %v, want %v", got, want)
	}
}

func TestImportersOf_APathThatCannotBeRelativisedContributesNothing(t *testing.T) {
	orig := importersRel
	t.Cleanup(func() { importersRel = orig })
	importersRel = func(string, string) (string, error) { return "", errors.New("rel failed") }
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package p\n\nimport _ \"example.com/m/internal/foo\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := importersOf(root, map[string]string{"example.com/m/internal/foo": "internal/foo"}, nil)

	if len(got) != 0 {
		t.Fatalf("importers = %v", got)
	}
}
