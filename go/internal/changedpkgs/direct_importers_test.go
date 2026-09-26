package changedpkgs

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// writeSource writes only a package clause and imports: the deriver parses imports, never compiles.
func writeSource(t *testing.T, root, relPath, pkgName string, imports ...string) {
	t.Helper()
	var b strings.Builder
	b.WriteString("package " + pkgName + "\n")
	if len(imports) > 0 {
		b.WriteString("\nimport (\n")
		for _, imp := range imports {
			b.WriteString("\t\"" + imp + "\"\n")
		}
		b.WriteString(")\n")
	}
	full := filepath.Join(root, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", relPath, err)
	}
	if err := os.WriteFile(full, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write %s: %v", relPath, err)
	}
}

// importerFixture: bar imports foo, zed imports foo only from its test, qux imports bar (transitive),
// lone imports nothing, and decoy imports another module's internal/foo.
func importerFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "go"), 0o755); err != nil {
		t.Fatalf("mkdir go/: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "go", "go.mod"),
		[]byte("module example.com/m\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	writeSource(t, root, "go/internal/foo/foo.go", "foo")
	writeSource(t, root, "go/internal/foo/foo_test.go", "foo")
	writeSource(t, root, "go/internal/bar/bar.go", "bar", "example.com/m/internal/foo")
	writeSource(t, root, "go/internal/zed/zed.go", "zed")
	writeSource(t, root, "go/internal/zed/zed_test.go", "zed", "example.com/m/internal/foo")
	writeSource(t, root, "go/internal/qux/qux.go", "qux", "example.com/m/internal/bar")
	writeSource(t, root, "go/internal/lone/lone.go", "lone")
	writeSource(t, root, "go/internal/decoy/decoy.go", "decoy", "example.com/other/internal/foo")
	return root
}

func TestDirectImporters_WidensToReverseImportersIncludingTestOnly(t *testing.T) {
	root := importerFixture(t)

	got := DirectImporters(root, []string{"./internal/foo/..."})
	want := []string{"./internal/bar", "./internal/zed"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DirectImporters = %v, want %v\n"+
			"  bar imports foo from a non-test file and must be included;\n"+
			"  zed imports foo ONLY from zed_test.go and must be included (it is the covering test);\n"+
			"  qux imports bar (transitive) and lone/decoy import neither — all must be excluded,\n"+
			"  and the changed package foo itself is already in the corpus so it must not repeat.",
			got, want)
	}
}

func TestDirectImporters_AcceptsBothPatternForms(t *testing.T) {
	root := importerFixture(t)

	recursive := DirectImporters(root, []string{"./internal/foo/..."})
	bare := DirectImporters(root, []string{"./internal/foo"})
	if !reflect.DeepEqual(recursive, bare) {
		t.Fatalf("pattern forms disagree: recursive=%v bare=%v — both name the same package, "+
			"so the widening must not depend on which upstream seam produced the pattern", recursive, bare)
	}
}

func TestDirectImporters_DeterministicSortedAndDeduped(t *testing.T) {
	root := importerFixture(t)

	got := DirectImporters(root, []string{"./internal/bar", "./internal/foo/...", "./internal/foo"})
	want := []string{"./internal/qux", "./internal/zed"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DirectImporters(overlapping patterns) = %v, want %v — sorted, deduped, and with every "+
			"INPUT package (foo, bar) excluded from its own widening", got, want)
	}

	for i := 0; i < 3; i++ {
		if again := DirectImporters(root, []string{"./internal/bar", "./internal/foo/...", "./internal/foo"}); !reflect.DeepEqual(again, got) {
			t.Fatalf("run %d returned %v, first run returned %v — the derivation must be deterministic", i, again, got)
		}
	}
}

func TestDirectImporters_FailsOpenOnUnusableInput(t *testing.T) {
	populated := importerFixture(t)
	noModule := t.TempDir()

	cases := []struct {
		name     string
		root     string
		patterns []string
	}{
		{"empty root", "", []string{"./internal/foo/..."}},
		{"missing root", filepath.Join(t.TempDir(), "does-not-exist"), []string{"./internal/foo/..."}},
		{"root without a go module", noModule, []string{"./internal/foo/..."}},
		{"nil patterns", populated, nil},
		{"empty pattern slice", populated, []string{}},
		{"empty pattern string", populated, []string{""}},
		{"unknown package", populated, []string{"./internal/nope/..."}},
		{"module-wide pattern", populated, []string{"./..."}},
		{"escaping pattern", populated, []string{"./../../etc/..."}},
		{"absolute pattern", populated, []string{"/internal/foo"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DirectImporters(tc.root, tc.patterns); got != nil {
				t.Fatalf("DirectImporters = %v, want nil (fail-open: the phase must degrade to "+
					"today's changed-packages-only corpus, never block and never over-inject)", got)
			}
		})
	}
}

func TestDirectImporters_NoImportersIsNotAnError(t *testing.T) {
	root := importerFixture(t)
	if got := DirectImporters(root, []string{"./internal/lone"}); got != nil {
		t.Fatalf("DirectImporters(leaf package) = %v, want nil — nothing imports it, "+
			"which is a normal cycle, not a derivation error", got)
	}
}

func TestDirectImporters_ReachableFromProduction(t *testing.T) {
	moduleDir, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve module dir: %v", err)
	}

	var callers []string
	fset := token.NewFileSet()
	walkErr := filepath.Walk(moduleDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			switch info.Name() {
			case "acs", "testdata", "vendor", ".git":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return nil
		}
		if file.Name.Name == "changedpkgs" {
			return nil
		}
		ast.Inspect(file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok || sel.Sel == nil || sel.Sel.Name != "DirectImporters" {
				return true
			}
			if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "changedpkgs" {
				rel, _ := filepath.Rel(moduleDir, path)
				callers = append(callers, filepath.ToSlash(rel))
			}
			return true
		})
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk module dir: %v", walkErr)
	}

	if len(callers) == 0 {
		t.Fatalf("no non-test production file in the go/ module calls changedpkgs.DirectImporters — " +
			"the reverse-import widening is dead code, and the amplification agent still cannot see " +
			"the test packages that cover the cycle's changed code")
	}
	t.Logf("production callers of changedpkgs.DirectImporters: %v", callers)
}
