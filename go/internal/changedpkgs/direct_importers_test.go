package changedpkgs

// RED contract for cycle-1267 Task 1 (`scope-test-amplification-context`,
// inbox test-amplification-context-scope, w=0.89) — the HALF that is still
// missing.
//
// The inbox item's how_to_apply spells the corpus as:
//
//	packages touched by the cycle diff -> their *_test.go files
//	                                   +  direct reverse-import test packages
//
// The first half landed (CoveringTests, covering_tests.go, cycle-1255). The
// second half did NOT: CoveringTests walks ONLY the changed packages' own
// directories, so a package whose *_test.go imports the changed package — the
// literal definition of a covering test — is still invisible to the
// amplification agent, which then falls back to the whole-repo Grep this task
// exists to remove. There is no reverse-dependency seam in the module today
// (the cycle-1267 fault-localization report cites a
// `changedpkgs.ImporterClosure`; it does not exist — verified by grep over
// go/internal, go/cmd and go/pkg).
//
// This file pins the missing seam:
//
//	DirectImporters(repoRoot string, pkgPatterns []string) []string
//
// Given the same go-test package patterns the rest of this package speaks
// ("./internal/foo/..." or the bare "./internal/foo"), it returns the sorted,
// deduped, bare-form patterns of the module packages that DIRECTLY import any
// of them — counting imports from *_test.go files, which is precisely how a
// covering test package depends on the code it covers. The input packages
// themselves are never returned (they are already in the corpus), and
// transitive importers are not (DIRECT means one hop: the item says "direct
// reverse-import", and an unbounded closure would re-inflate the very context
// this task shrinks).
//
// Fail-open, like every other deriver here: any unusable input yields nil —
// never an error, never a panic — and the corpus degrades to exactly today's
// changed-packages-only set.
//
// Import-shape probe (the cycle-644 obligation): every symbol pinned here is
// pinned from INSIDE package changedpkgs, so no new import edge is introduced
// in either direction; the reachability test below resolves its caller from the
// parsed import graph rather than by importing it. changedpkgs imports only
// internal/gitexec and internal/gopkgpattern, so the production caller this
// contract requires (internal/core, which already imports changedpkgs) adds no
// cycle. Confirmed against the current graph before freezing this pin.

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

// writeSource materialises one repo-relative, slash-separated Go source file
// with the given package clause and imports. The files are parsed by the
// deriver, never compiled, so a minimal body is enough.
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

// importerFixture builds a synthetic module whose reverse-import graph exercises
// every axis of the contract:
//
//	internal/foo   — the CHANGED package (the input)
//	internal/bar   — imports foo from a NON-test file      => direct importer
//	internal/zed   — imports foo from zed_test.go ONLY     => direct importer
//	                 (the covering-test case: the whole point of the widening)
//	internal/qux   — imports bar, not foo                  => TRANSITIVE, excluded
//	internal/lone  — imports nothing                       => unrelated, excluded
//	internal/decoy — imports a DIFFERENT module's foo      => excluded (the
//	                 suffix "internal/foo" must not match across module paths)
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

// TestDirectImporters_WidensToReverseImportersIncludingTestOnly — AC1, the crux.
// The covering-test corpus must reach the packages whose tests exercise the
// changed package. A test-only importer (zed) is the case that matters most: it
// is invisible to CoveringTests today, and it is exactly what "covering test"
// means. Transitive importers, unrelated packages, a same-suffix package from a
// DIFFERENT module, and the input package itself must all stay out — that
// exclusion IS the token saving this task exists for.
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

// TestDirectImporters_AcceptsBothPatternForms — AC2. ChangedPackages/FromGit emit
// the recursive "./internal/foo/..." form and core's changedGoTestPackages emits
// the bare "./internal/foo" form; the widening sits downstream of both, so the
// two must derive identically. Output is always the bare form, which is what
// CoveringTests consumes.
func TestDirectImporters_AcceptsBothPatternForms(t *testing.T) {
	root := importerFixture(t)

	recursive := DirectImporters(root, []string{"./internal/foo/..."})
	bare := DirectImporters(root, []string{"./internal/foo"})
	if !reflect.DeepEqual(recursive, bare) {
		t.Fatalf("pattern forms disagree: recursive=%v bare=%v — both name the same package, "+
			"so the widening must not depend on which upstream seam produced the pattern", recursive, bare)
	}
}

// TestDirectImporters_DeterministicSortedAndDeduped — AC3. The corpus is written
// into a run-dir artifact that lands in an agent's prompt; a set that reorders
// between runs churns the prompt cache and makes before/after token measurement
// (the item's own success metric) unreadable. Overlapping patterns must also
// collapse: bar imports foo and qux imports bar, so asking for both foo and bar
// must report bar/zed/qux each exactly once.
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

// TestDirectImporters_FailsOpenOnUnusableInput — AC4, the negative half and the
// item's explicit guard ("scoping must fail-open to today's behaviour on
// derivation error, never block the phase"). Every unusable input yields nil,
// never a panic and never an error: the corpus then contains exactly the
// changed packages, which is today's behaviour. The module-wide "./..." pattern
// is unusable ON PURPOSE — widening from every package would name the whole
// repo, which is strictly worse than the blind search this artifact replaces.
func TestDirectImporters_FailsOpenOnUnusableInput(t *testing.T) {
	populated := importerFixture(t)
	noModule := t.TempDir() // a directory with no go/go.mod at all

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

// TestDirectImporters_NoImportersIsNotAnError — AC5. A cycle that touches a leaf
// package nothing imports is the common case, not a failure: it must yield nil
// so the corpus is the changed packages alone, byte-identically to today.
func TestDirectImporters_NoImportersIsNotAnError(t *testing.T) {
	root := importerFixture(t)
	if got := DirectImporters(root, []string{"./internal/lone"}); got != nil {
		t.Fatalf("DirectImporters(leaf package) = %v, want nil — nothing imports it, "+
			"which is a normal cycle, not a derivation error", got)
	}
}

// TestDirectImporters_ReachableFromProduction — AC6, the WIRING proof. A widening
// seam whose only caller is a test injects nothing into the phase and saves zero
// tokens (the cycle-1255 precedent for CoveringTests itself). Callers are
// resolved from the parsed import graph of the whole go/ module: at least one
// NON-test file outside package changedpkgs — and outside go/acs, whose
// predicates are the gate, not the product — must reference
// changedpkgs.DirectImporters.
func TestDirectImporters_ReachableFromProduction(t *testing.T) {
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
		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return nil // unparseable file is not evidence either way
		}
		if file.Name.Name == "changedpkgs" {
			return nil // the definition site is not a caller
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
