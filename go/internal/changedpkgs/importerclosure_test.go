package changedpkgs

import (
	"path/filepath"
	"sort"
	"testing"
)

func repoRootForTest(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	return root
}

func TestImporterClosure_RouterRoutingtest(t *testing.T) {
	got := ImporterClosure(repoRootForTest(t), []string{"./internal/router/..."})

	if !contains(got, "./internal/routingtest/...") {
		t.Errorf("closure of ./internal/router/... omits ./internal/routingtest/... (the cycle-1250 miss); got %v", got)
	}
	if !contains(got, "./internal/router/...") {
		t.Errorf("closure dropped its own input ./internal/router/...; got %v", got)
	}
}

// gitexec cannot reach router, but its tests import test/fixtures, whose deps do; a transitive test-import walk would include it.
func TestImporterClosure_ExcludesNonImporters(t *testing.T) {
	got := ImporterClosure(repoRootForTest(t), []string{"./internal/router/..."})

	if contains(got, "./internal/gitexec/...") {
		t.Errorf("closure of ./internal/router/... wrongly includes non-importer ./internal/gitexec/... (return-everything implementation?); got %v", got)
	}
	if len(got) == 0 {
		t.Fatalf("closure returned nothing for a real package")
	}
}

// acssuite reaches gitexec only through changedpkgs.
func TestImporterClosure_Transitive(t *testing.T) {
	got := ImporterClosure(repoRootForTest(t), []string{"./internal/gitexec/..."})

	if !contains(got, "./internal/changedpkgs/...") {
		t.Errorf("closure of ./internal/gitexec/... omits direct importer ./internal/changedpkgs/...; got %v", got)
	}
	if !contains(got, "./internal/acssuite/...") {
		t.Errorf("closure of ./internal/gitexec/... omits 2-hop importer ./internal/acssuite/... (one-hop-only implementation?); got %v", got)
	}
}

func TestImporterClosure_BestEffortOnBadInput(t *testing.T) {
	in := []string{"./internal/router/..."}

	cases := []struct {
		name     string
		repoRoot string
		pkgs     []string
		want     []string
	}{
		{"empty repoRoot", "", in, in},
		{"nonexistent repoRoot", filepath.Join(t.TempDir(), "no-such-repo"), in, in},
		{"non-repo repoRoot", t.TempDir(), in, in},
		{"nil pkgs", repoRootForTest(t), nil, nil},
		{"empty pkgs", repoRootForTest(t), []string{}, nil},
		{"junk pattern", "", []string{"not-a-pattern"}, []string{"not-a-pattern"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("ImporterClosure panicked: %v", r)
				}
			}()
			got := ImporterClosure(tc.repoRoot, tc.pkgs)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestImporterClosure_SortedDedupedAndModuleRoot(t *testing.T) {
	root := repoRootForTest(t)

	got := ImporterClosure(root, []string{"./internal/router/...", "./internal/router/..."})
	if !sort.StringsAreSorted(got) {
		t.Errorf("closure output is not sorted: %v", got)
	}
	seen := map[string]int{}
	for _, g := range got {
		seen[g]++
		if seen[g] > 1 {
			t.Errorf("closure output has duplicate %q: %v", g, got)
		}
	}

	rootGot := ImporterClosure(root, []string{"./..."})
	if len(rootGot) != 1 || rootGot[0] != "./..." {
		t.Errorf("closure of module-root ./... must be the identity; got %v", rootGot)
	}
}

// routingeval's tests import core; its build deps do not.
func TestImporterClosure_TestOnlyImporter(t *testing.T) {
	got := ImporterClosure(repoRootForTest(t), []string{"./internal/core/..."})
	if !contains(got, "./internal/routingeval/...") {
		t.Errorf("closure of ./internal/core/... omits the test-only importer ./internal/routingeval/... (build-deps-only walk?); got %d patterns", len(got))
	}
}

// go/acs/cycle8 holds only `//go:build acs` files.
func TestImporterClosureChecked_TestableExcludesTagOnlyDirs(t *testing.T) {
	c, ok := ImporterClosureChecked(repoRootForTest(t), []string{"./acs/cycle8/...", "./internal/gitexec/..."})
	if !ok {
		t.Fatal("the real module lists")
	}
	if !contains(c.Patterns, "./acs/cycle8/...") || contains(c.Testable, "./acs/cycle8/...") {
		t.Errorf("tag-only dir: Patterns=%v Testable=%v", contains(c.Patterns, "./acs/cycle8/..."), contains(c.Testable, "./acs/cycle8/..."))
	}
	if !contains(c.Testable, "./internal/gitexec/...") {
		t.Errorf("a plain package is testable; Testable=%v", c.Testable)
	}
	if !sort.StringsAreSorted(c.Testable) {
		t.Errorf("Testable is sorted: %v", c.Testable)
	}
}

func TestImporterClosureChecked_NotDerivableOutsideAModule(t *testing.T) {
	in := []string{"./internal/router/..."}
	c, ok := ImporterClosureChecked(t.TempDir(), in)
	if ok || len(c.Patterns) != 1 || c.Patterns[0] != in[0] || len(c.Testable) != 0 {
		t.Errorf("non-module root: ok=%v closure=%+v", ok, c)
	}
	if c, ok := ImporterClosureChecked(repoRootForTest(t), nil); !ok || len(c.Patterns) != 0 {
		t.Errorf("nil input: ok=%v closure=%+v", ok, c)
	}
}

func TestClosure_TestableIsASubsetOfPatterns(t *testing.T) {
	c, ok := ImporterClosureChecked(repoRootForTest(t), []string{"./internal/gitexec/..."})
	if !ok || len(c.Patterns) == 0 {
		t.Fatalf("closure = %+v ok=%v", c, ok)
	}
	for _, p := range c.Testable {
		if !contains(c.Patterns, p) {
			t.Errorf("Testable %q is not in Patterns %v", p, c.Patterns)
		}
	}
	if len(c.Testable) == 0 || !contains(c.Testable, "./internal/gitexec/...") {
		t.Errorf("the changed package itself is testable: %v", c.Testable)
	}
	var zero Closure
	if len(zero.Patterns) != 0 || len(zero.Testable) != 0 {
		t.Errorf("zero Closure is empty both ways: %+v", zero)
	}
}
