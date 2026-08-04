// RED contract for cycle-1255 task `test-amplification-covering-tests-scope`.
//
// test-amplification is the pipeline's per-run context outlier (81.2M cache-read
// tokens / 15 runs = 5.4M per run, 13.5 min avg — knowledge-base/research/
// token-usage-history-2026-07-20.md). Root cause: its phase spec declares only
// tdd-contract.md + build-report.md as inputs, and the agent is forbidden from
// reading diffs/implementation, so it must Grep/Glob the WHOLE repo to discover
// which existing test files cover the paths it was handed.
//
// The fix: derive the covering-test set deterministically (changed packages →
// their _test.go files) and inject it as an explicit input, so the agent is TOLD
// the in-scope tests instead of searching for them. The black-box constraint is
// untouched: a list of test-file PATHS is not the diff and not the implementation.
//
// CoveringTests is the deriver. Contract pinned below:
//
//	CoveringTests(repoRoot string, pkgPatterns []string) []string
//
// It maps go-test package patterns (the exact strings ChangedPackages/FromGit
// already emit, e.g. "./internal/foo/...") to the sorted, deduped, repo-relative
// slash-separated paths of the _test.go files in those packages. It is fail-open
// like the rest of this package: any unusable input yields nil, never an error
// and never a panic — the phase then behaves exactly as it does today.
//
// Import-shape probe (the cycle-644 obligation): this test pins the symbol from
// INSIDE package changedpkgs (no new import at all), and the reachability test
// below resolves callers from the parsed import graph rather than by importing
// them, so no new edge is introduced in either direction. changedpkgs imports
// only internal/gitexec, so a caller in core/phases/router adds no cycle.
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

// writeFiles materialises a fake repo tree (paths repo-relative, slash-separated)
// under root and returns root.
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

// TestCoveringTests_DerivesTestFilesForChangedPackagesOnly — AC5 (the crux).
// Only the _test.go files of the CHANGED packages come back: non-test sources are
// excluded (they are implementation, which the phase may not read), and untouched
// packages are excluded (that exclusion IS the token saving). Sub-packages of a
// "/..." pattern are included, because that is what the pattern means to go test.
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

// TestCoveringTests_DedupesAcrossOverlappingPatterns — AC6. ChangedPackages can
// emit a parent and a child pattern for the same diff; a duplicated path would
// inflate the injected corpus the fix exists to shrink.
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

// TestCoveringTests_AcceptsNonRecursivePatternForm — AC7, the EDGE axis. A caller
// may hand a bare package path without the "/..." suffix; the deriver must not
// silently return nothing (a silent empty set is indistinguishable from
// fail-open, so the phase would keep its blind search forever).
func TestCoveringTests_AcceptsNonRecursivePatternForm(t *testing.T) {
	root := writeFiles(t, t.TempDir(), "go/internal/foo/foo_test.go")

	got := CoveringTests(root, []string{"./internal/foo"})
	want := []string{"go/internal/foo/foo_test.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CoveringTests(bare pattern) = %v, want %v", got, want)
	}
}

// TestCoveringTests_FailsOpenOnUnusableInput — AC8, the NEGATIVE axis and the
// task's own hard constraint: never block the phase. Every unusable input must
// yield nil (the injection is skipped and the agent searches exactly as it does
// today), never a panic. The module-wide "./..." pattern is included here on
// purpose: injecting every test file in the repo is strictly worse than today's
// behaviour, so it must fail open rather than "succeed" hugely.
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

// TestCoveringTests_ReachableFromProduction — AC9, the WIRING proof. A deriver
// whose only caller is a test is dead code and saves zero tokens. This resolves
// callers from the parsed import graph of the whole go/ module: at least one
// NON-test file outside package changedpkgs (and outside go/acs, whose predicates
// are the gate, not the product) must reference changedpkgs.CoveringTests.
func TestCoveringTests_ReachableFromProduction(t *testing.T) {
	moduleDir, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve module dir: %v", err)
	}

	var callers []string
	fset := token.NewFileSet()
	walkErr := filepath.Walk(moduleDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // best-effort walk; an unreadable dir is not a verdict
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
			return nil // the definition site is not a caller
		}
		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return nil // unparseable file is not evidence either way
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
