package sizeratchet

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFuncKey_NamesReceiversByTheirBaseType(t *testing.T) {
	src := `package p
func Free() {}
func (T) Value() {}
func (*T) Pointer() {}
func ((T)) Paren() {}
func (g G[K]) Generic() {}
func (g *G[K]) PointerGeneric() {}
func (g G[K, V]) Pair() {}
func (a.B) Qualified() {}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "p.go", src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"Free":           "pkg/dir.Free",
		"Value":          "pkg/dir.T.Value",
		"Pointer":        "pkg/dir.T.Pointer",
		"Paren":          "pkg/dir.T.Paren",
		"Generic":        "pkg/dir.G.Generic",
		"PointerGeneric": "pkg/dir.G.PointerGeneric",
		"Pair":           "pkg/dir.G.Pair",
		"Qualified":      "pkg/dir.*ast.SelectorExpr.Qualified",
	}
	got := map[string]string{}
	for _, d := range file.Decls {
		if decl, ok := d.(*ast.FuncDecl); ok {
			got[decl.Name.Name] = funcKey("pkg/dir", decl)
		}
	}
	for name, key := range want {
		if got[name] != key {
			t.Errorf("funcKey for %s = %q, want %q", name, got[name], key)
		}
	}
}

func TestWalk_ErrorsNameTheRootAndTheCause(t *testing.T) {
	t.Run("root that does not exist", func(t *testing.T) {
		spans, err := Walk(filepath.Join(t.TempDir(), "absent"))
		if err == nil || !strings.Contains(err.Error(), "sizeratchet: walk ") || spans != nil {
			t.Fatalf("spans=%v err=%v", spans, err)
		}
	})
	t.Run("a file that does not parse", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "bad.go"), []byte("package p\nfunc ("), 0o644); err != nil {
			t.Fatal(err)
		}
		spans, err := Walk(root)
		if err == nil || !strings.Contains(err.Error(), "bad.go") || spans != nil {
			t.Fatalf("spans=%v err=%v", spans, err)
		}
	})
	t.Run("an unreadable subdirectory", func(t *testing.T) {
		root := t.TempDir()
		sub := filepath.Join(root, "sub")
		if err := os.Mkdir(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(sub, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(sub, 0o755) })
		if _, err := os.ReadDir(sub); err == nil {
			t.Skip("directory permissions are not enforced for this user")
		}
		if _, err := Walk(root); err == nil {
			t.Fatal("an unreadable directory must fail the walk, not be skipped silently")
		}
	})
}

func TestWalkFile_PathOutsideTheRootCannotBeRelativised(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.go")
	if err := os.WriteFile(path, []byte("package p\nfunc A() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	spans, err := walkFile(token.NewFileSet(), "relative-root", path)

	if err == nil || spans != nil {
		t.Fatalf("spans=%v err=%v", spans, err)
	}
}

func TestScan_MissingWalkRootAndMissingOffenderListAreErrors(t *testing.T) {
	if err := Scan(filepath.Join(t.TempDir(), "absent")); err == nil || !strings.Contains(err.Error(), "sizeratchet: walk") {
		t.Fatalf("absent root: %v", err)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package p\nfunc A() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Scan(root); err == nil || !strings.Contains(err.Error(), "read offender list") {
		t.Fatalf("absent offender list: %v", err)
	}
}
