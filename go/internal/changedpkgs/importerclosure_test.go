package changedpkgs

import (
	"path/filepath"
	"sort"
	"testing"
)

// importerclosure_test.go — RED contract for cycle-1253 Task 1
// (`tia-importer-closure`, from inbox item
// .evolve/inbox/2026-07-30T09-00-00Z-egps-regression-tia-selection.json,
// P1 weight 0.91, 3rd live instance).
//
// The defect. Every derivation in this package is FORWARD-ONLY: FileToPackage
// maps a changed file to the package it LIVES in, and ChangedPackages/FromGit/
// FromGitChecked never walk the import graph. So a change confined to
// `internal/router` never selects `internal/routingtest` — even though
// routingtest imports router and holds the keystone parity invariant. That is
// exactly the cycle-1250 miss: main stayed red for 5 commits because the only
// thing that would have caught it was a package the changed-package set could
// not name. Test-impact selection built on a forward-only set silently hides a
// whole regression class.
//
// The contract these tests freeze:
//
//	func ImporterClosure(repoRoot string, pkgs []string) []string
//
//   - repoRoot is the REPOSITORY root (the dir containing the `go/` module
//     dir) — same parameter meaning as FromGit/FromGitChecked, so callers that
//     already hold one can pass it straight through.
//   - pkgs are `./dir/...` go test patterns as emitted by FileToPackage.
//   - the result is the sorted, deduped UNION of the input patterns and a
//     `./dir/...` pattern for every module package that TRANSITIVELY imports
//     any input package. The input is never dropped: closure only ever widens.
//   - best-effort, exactly like the rest of this package: an empty or
//     nonexistent repoRoot, a junk pattern, or any `go list` failure yields the
//     input set unchanged (an EMPTY added closure) — never an error, never a
//     panic, never a lost input entry.
//
// RED today: ImporterClosure is undefined, so this package fails to COMPILE —
// a hard non-zero exit, never a silent pass. GREEN once Builder adds it.

// repoRootForTest resolves the repository root from this test's own location
// (go/internal/changedpkgs → ../../..). Derived, not hardcoded, so it is
// correct in the main tree and in every fleet worktree alike.
func repoRootForTest(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	return root
}

// contains (fromgit_test.go) is reused here rather than redeclared.

// TestImporterClosure_RouterRoutingtest is the cycle-1250 reproducer and the
// crux of this task: a change confined to internal/router MUST select
// internal/routingtest, because routingtest imports router (non-test edge, in
// agent.go/bricks.go/engine.go) and owns the keystone parity test that a
// forward-only set never runs. The input pattern must also survive.
func TestImporterClosure_RouterRoutingtest(t *testing.T) {
	got := ImporterClosure(repoRootForTest(t), []string{"./internal/router/..."})

	if !contains(got, "./internal/routingtest/...") {
		t.Errorf("closure of ./internal/router/... omits ./internal/routingtest/... (the cycle-1250 miss); got %v", got)
	}
	if !contains(got, "./internal/router/...") {
		t.Errorf("closure dropped its own input ./internal/router/...; got %v", got)
	}
}

// TestImporterClosure_ExcludesNonImporters is the anti-no-op negative. An
// implementation that just returns every package in the module would pass the
// reproducer above while making selection useless. internal/gitexec is a leaf
// (its only module dep is internal/sysexec) — it cannot import router
// transitively, so it MUST NOT appear in router's closure. Depending on a
// changed package is not the same relation as importing it.
func TestImporterClosure_ExcludesNonImporters(t *testing.T) {
	got := ImporterClosure(repoRootForTest(t), []string{"./internal/router/..."})

	if contains(got, "./internal/gitexec/...") {
		t.Errorf("closure of ./internal/router/... wrongly includes non-importer ./internal/gitexec/... (return-everything implementation?); got %v", got)
	}
	if len(got) == 0 {
		t.Fatalf("closure returned nothing for a real package")
	}
}

// TestImporterClosure_Transitive pins that the closure is TRANSITIVE, not
// one-hop. internal/changedpkgs imports internal/gitexec directly; internal/
// acssuite imports internal/changedpkgs. So a change in gitexec must select
// both — a single-hop implementation finds changedpkgs and stops, leaving the
// acssuite regression class unselected.
func TestImporterClosure_Transitive(t *testing.T) {
	got := ImporterClosure(repoRootForTest(t), []string{"./internal/gitexec/..."})

	if !contains(got, "./internal/changedpkgs/...") {
		t.Errorf("closure of ./internal/gitexec/... omits direct importer ./internal/changedpkgs/...; got %v", got)
	}
	if !contains(got, "./internal/acssuite/...") {
		t.Errorf("closure of ./internal/gitexec/... omits 2-hop importer ./internal/acssuite/... (one-hop-only implementation?); got %v", got)
	}
}

// TestImporterClosure_BestEffortOnBadInput pins the package's standing
// error contract on every degenerate input: no panic, no error return, and the
// input set is preserved verbatim (an EMPTY added closure). Losing an input
// entry on a bad repoRoot would silently NARROW selection below the
// forward-only baseline — strictly worse than not having this function.
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

// TestImporterClosure_SortedDedupedAndModuleRoot pins output SHAPE, matching
// ChangedPackages/FromGitChecked: sorted and deduped, so consumers can compare
// and cache sets. The module-root pattern "./..." already covers every package,
// so its closure is the identity — widening it would be meaningless churn.
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
