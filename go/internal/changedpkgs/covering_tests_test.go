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

func writeFiles(t *testing.T, root string, paths ...string) string {
	t.Helper()
	for _, p := range paths {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", p, err)
		}
		if err := os.WriteFile(full, []byte("package x\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}
	return root
}

func TestCoveringTests_DerivesTestFilesForChangedPackagesOnly(t *testing.T) {
	root := writeFiles(t, t.TempDir(),
		"go/internal/foo/foo.go",
		"go/internal/foo/foo_test.go",
		"go/internal/foo/extra_test.go",
		"go/internal/foo/sub/sub_test.go",
		"go/internal/bar/bar_test.go", // untouched package — must NOT appear
	)

	got := CoveringTests(root, []string{"./internal/foo/..."})
	want := []string{
		"go/internal/foo/extra_test.go",
		"go/internal/foo/foo_test.go",
		"go/internal/foo/sub/sub_test.go",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CoveringTests = %v, want %v (sorted, test files of the changed package only)", got, want)
	}
}

func TestCoveringTests_DedupesAcrossOverlappingPatterns(t *testing.T) {
	root := writeFiles(t, t.TempDir(),
		"go/internal/foo/foo_test.go",
		"go/internal/foo/sub/sub_test.go",
	)

	got := CoveringTests(root, []string{"./internal/foo/...", "./internal/foo/sub/...", "./internal/foo/..."})
	want := []string{"go/internal/foo/foo_test.go", "go/internal/foo/sub/sub_test.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CoveringTests = %v, want %v (deduped across overlapping patterns)", got, want)
	}
}

func TestCoveringTests_AcceptsNonRecursivePatternForm(t *testing.T) {
	root := writeFiles(t, t.TempDir(), "go/internal/foo/foo_test.go")

	got := CoveringTests(root, []string{"./internal/foo"})
	want := []string{"go/internal/foo/foo_test.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CoveringTests(bare pattern) = %v, want %v", got, want)
	}
}

func TestCoveringTests_FailsOpenOnUnusableInput(t *testing.T) {
	populated := writeFiles(t, t.TempDir(), "go/internal/foo/foo_test.go")

	cases := []struct {
		name     string
		root     string
		patterns []string
	}{
		{"empty root", "", []string{"./internal/foo/..."}},
		{"missing root", filepath.Join(t.TempDir(), "does-not-exist"), []string{"./internal/foo/..."}},
		{"nil patterns", populated, nil},
		{"empty pattern string", populated, []string{""}},
		{"unknown package", populated, []string{"./internal/nope/..."}},
		{"module-wide pattern", populated, []string{"./..."}},
		{"escaping pattern", populated, []string{"./../../etc/..."}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CoveringTests(tc.root, tc.patterns); got != nil {
				t.Fatalf("CoveringTests = %v, want nil (fail-open: the phase must degrade to today's behaviour, never block or over-inject)", got)
			}
		})
	}
}

func TestCoveringTests_ReachableFromProduction(t *testing.T) {
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
		if filepath.Dir(path) == moduleDir+string(os.PathSeparator)+filepath.Join("internal", "changedpkgs") {
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
			if !ok || sel.Sel == nil || sel.Sel.Name != "CoveringTests" {
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
		t.Fatalf("no non-test production file in the go/ module calls changedpkgs.CoveringTests — " +
			"a deriver reached only from tests is dead code and injects nothing into the test-amplification phase")
	}
	t.Logf("production callers of changedpkgs.CoveringTests: %v", callers)
}
