// packagegraph_test.go — cycle-871 TDD contract for merge ladder RUNG 1
// (inbox merge-rung1-package-graph-disjoint; research
// knowledge-base/research/merge-concurrency-2026/README.md finding #2:
// "file-level disjointness is explicitly insufficient (merge skew: rename +
// call-site)"). fleet.Partition today buckets todos by literal file paths
// only; it has no notion of Go package import-graph reachability, so two
// todos that touch disjoint files but are connected through the import graph
// (e.g. one edits a package, the other edits a caller of that package) are
// wrongly treated as safe to co-schedule concurrently.
//
// This file pins the package-graph resolver's contract using REAL repo
// packages (no synthetic fixtures needed): go/internal/fleet transitively
// imports go/internal/ipcenv (see partition.go's import block), while
// go/internal/acsrunner has zero import relationship with fleet in either
// direction (verified via `go list -deps` at authoring time).
//
// RED status at authoring (cycle 871): every test below fails to COMPILE —
// TransitivePackageSet, IsGlobalZone and GlobalZoneFiles do not exist yet.
// That is the correct RED signal (Builder's job is to add packagegraph.go).
package fleet

import "testing"

// repoRoot is the go module root as seen from this package's directory
// (go/internal/fleet) when `go test` runs with its package dir as cwd.
const repoRoot = "../.."

func TestTransitivePackageSet_ResolvesTransitiveImport(t *testing.T) {
	set, err := TransitivePackageSet([]string{"internal/fleet/partition.go"}, repoRoot)
	if err != nil {
		t.Fatalf("TransitivePackageSet: %v", err)
	}
	const wantPkg = "github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	if !set[wantPkg] {
		t.Errorf("fleet transitively imports ipcenv (see partition.go), but %q missing from set=%v", wantPkg, set)
	}
}

func TestTransitivePackageSet_NoCrossContaminationFromUnrelatedPackage(t *testing.T) {
	set, err := TransitivePackageSet([]string{"internal/fleet/partition.go"}, repoRoot)
	if err != nil {
		t.Fatalf("TransitivePackageSet: %v", err)
	}
	const unrelated = "github.com/mickeyyaya/evolve-loop/go/internal/acsrunner"
	if set[unrelated] {
		t.Errorf("fleet does not import acsrunner (verified via go list -deps); set must not contain %q", unrelated)
	}
}

func TestTransitivePackageSet_EmptyFiles_ReturnsEmptySet(t *testing.T) {
	set, err := TransitivePackageSet(nil, repoRoot)
	if err != nil {
		t.Fatalf("TransitivePackageSet(nil): %v", err)
	}
	if len(set) != 0 {
		t.Errorf("no files -> no packages, got set=%v", set)
	}
}

func TestTransitivePackageSet_UnknownFile_ReturnsError(t *testing.T) {
	if _, err := TransitivePackageSet([]string{"internal/does/not/exist/nope.go"}, repoRoot); err == nil {
		t.Errorf("a file outside any real package must error, not silently resolve")
	}
}

func TestIsGlobalZone_MatchesGoModAndGoSum(t *testing.T) {
	for _, f := range []string{"go.mod", "./go.mod", "go.sum"} {
		if !IsGlobalZone(f) {
			t.Errorf("IsGlobalZone(%q) = false, want true (global-zone file)", f)
		}
	}
}

func TestIsGlobalZone_OrdinarySourceFile_NotGlobalZone(t *testing.T) {
	if IsGlobalZone("internal/fleet/partition.go") {
		t.Errorf("an ordinary source file must not be classified as global-zone")
	}
}

func TestGlobalZoneFiles_NonEmpty(t *testing.T) {
	if len(GlobalZoneFiles()) == 0 {
		t.Errorf("GlobalZoneFiles() must list at least go.mod/go.sum")
	}
}
