//go:build acs

// Package cycle1226 covers the reachabilityprobe-build-import-graph task
// (inbox tdd-structural-test-reachability-probe, weight 0.92; fleet-scoped
// todo tdd-reachability-probe-check).
//
// Gap closed: reachabilityprobe.ImportGraph (landed cycle-1225) is
// caller-supplied only — nothing in the repo builds it from the real
// toolchain, even though the package doc says the caller must derive it from
// `go list -deps`. This cycle adds
// reachabilityprobe.BuildImportGraph(repoRoot string, pkgs ...string)
// (ImportGraph, error), shelling out to `go list -deps -json` the same way
// go/internal/fleet/packagegraph.go's TransitivePackageSet already does for a
// different purpose (fleet partitioning), and proves the deterministic
// cycle-644 check (CheckCallSite) round-trips correctly against REAL
// toolchain output, not just hand-built literal graphs.
package cycle1226

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/reachabilityprobe"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// moduleRoot resolves the Go module root (the "go" directory containing
// go.mod) from the git repo root, matching the repoRoot convention already
// established by fleet.TransitivePackageSet (packagegraph_test.go: repoRoot
// is the module root, not the git repo root).
func moduleRoot(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

// TestC1226_001_BuildImportGraphCapturesKnownDirectImport is the primary
// positive case: go/internal/fleet imports go/internal/sysexec directly
// (packagegraph.go's own import list) — a known, stable, in-repo edge that
// isn't going away. BuildImportGraph, driven against the REAL toolchain, must
// surface that edge in its returned ImportGraph. A no-op stub returning an
// empty map would fail this immediately.
func TestC1226_001_BuildImportGraphCapturesKnownDirectImport(t *testing.T) {
	root := moduleRoot(t)

	graph, err := reachabilityprobe.BuildImportGraph(root, "./internal/fleet")
	if err != nil {
		t.Fatalf("BuildImportGraph(%q, ./internal/fleet) returned error: %v", root, err)
	}
	if graph == nil {
		t.Fatalf("BuildImportGraph(%q, ./internal/fleet) = nil graph, want a populated ImportGraph", root)
	}

	const fleetPkg = "github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	const sysexecPkg = "github.com/mickeyyaya/evolve-loop/go/internal/sysexec"

	imports, ok := graph[fleetPkg]
	if !ok {
		t.Fatalf("graph missing key %q; graph keys: %v", fleetPkg, keys(graph))
	}
	if !contains(imports, sysexecPkg) {
		t.Errorf("graph[%q] = %v, want it to contain %q (packagegraph.go imports sysexec)", fleetPkg, imports, sysexecPkg)
	}
}

// TestC1226_002_BuildImportGraphWrapsToolchainFailure is the negative case
// (strongest anti-no-op signal, skills/adversarial-testing/SKILL.md §6): an
// unresolvable package pattern must produce a non-nil, wrapped error (not a
// panic, not a silently empty graph) so callers can distinguish "toolchain
// failed" from "package has zero imports".
func TestC1226_002_BuildImportGraphWrapsToolchainFailure(t *testing.T) {
	root := moduleRoot(t)

	graph, err := reachabilityprobe.BuildImportGraph(root, "./internal/does/not/exist/nope")
	if err == nil {
		t.Fatalf("BuildImportGraph(%q, bogus package) = (%v, nil), want a non-nil error", root, graph)
	}
	if !strings.Contains(err.Error(), "reachabilityprobe") {
		t.Errorf("BuildImportGraph error = %q, want it wrapped with package context (\"reachabilityprobe\")", err.Error())
	}
}

// TestC1226_003_BuildImportGraphRoundTripsIntoCheckCallSite is the frozen
// regression test (doNotModifyTests:true — AC2): it proves the deterministic
// cycle-644 check works against a REAL go-list-deps-derived graph, not a
// hand-built literal, with both a positive (cycle detected) and a negative
// (no false cycle) case in one table.
func TestC1226_003_BuildImportGraphRoundTripsIntoCheckCallSite(t *testing.T) {
	root := moduleRoot(t)

	graph, err := reachabilityprobe.BuildImportGraph(root, "./internal/fleet", "./internal/reachabilityprobe")
	if err != nil {
		t.Fatalf("BuildImportGraph(%q) returned error: %v", root, err)
	}

	const fleetPkg = "github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	const sysexecPkg = "github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	const probePkg = "github.com/mickeyyaya/evolve-loop/go/internal/reachabilityprobe"

	cases := []struct {
		name      string
		site      reachabilityprobe.CallSite
		wantCycle bool
	}{
		{
			// fleet already imports sysexec (real edge, asserted in
			// TestC1226_001). Freezing a structural test that pins
			// fleet.SomeFunc( inside a sysexec-package file would require
			// sysexec to import fleet back — the cycle-644 shape, against
			// real toolchain data this time instead of a literal.
			name:      "sysexec pinning fleet is an unbuildable cycle (real edge)",
			site:      reachabilityprobe.CallSite{PinningPackage: sysexecPkg, ReferencedPackage: fleetPkg, Symbol: "TransitivePackageSet"},
			wantCycle: true,
		},
		{
			// reachabilityprobe imports nothing internal (package doc:
			// "this package only walks the graph it is given") and fleet
			// does not import reachabilityprobe, so pinning a reference to
			// fleet inside a reachabilityprobe-package file is legitimately
			// acyclic — must NOT be flagged.
			name:      "reachabilityprobe pinning fleet is acyclic (no reverse edge)",
			site:      reachabilityprobe.CallSite{PinningPackage: probePkg, ReferencedPackage: fleetPkg, Symbol: "TransitivePackageSet"},
			wantCycle: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := reachabilityprobe.CheckCallSite(graph, tc.site)
			if tc.wantCycle && v == nil {
				t.Errorf("CheckCallSite(realGraph, %+v) = nil, want a Violation", tc.site)
			}
			if !tc.wantCycle && v != nil {
				t.Errorf("CheckCallSite(realGraph, %+v) = %+v, want nil", tc.site, v)
			}
		})
	}
}

// TestC1226_004_ReachabilityProbeRaceClean enforces AC4: the package's own
// test suite (including this cycle's new BuildImportGraph tests once
// unfrozen at build/test file level) must pass under -race. Scoped to the
// single named package (no trailing "/...") per the flaky-predicate-shape
// rules — a subtree sweep is contention-sensitive under fleet load
// (cycles 1173/1175/1178).
func TestC1226_004_ReachabilityProbeRaceClean(t *testing.T) {
	root := moduleRoot(t)
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", root, "test", "-race", "-tags", "acs", "./internal/reachabilityprobe")
	if err != nil {
		t.Errorf("go test -race ./internal/reachabilityprobe exited %d: %v\nstdout: %s\nstderr: %s", code, err, stdout, stderr)
	}
}

// TestC1226_005_ApicoverNamedTestCoversBuildImportGraph enforces the apicover
// house rule for an existing enrolled package gaining a new exported symbol:
// reachabilityprobe is already in go/.apicover-enforce (cycle-1225), so its
// apicover_named_test.go must name every exported symbol including the new
// BuildImportGraph — an enrolled-but-unnamed symbol trips the repo-wide gate
// later (cycle-1218 precedent).
func TestC1226_005_ApicoverNamedTestCoversBuildImportGraph(t *testing.T) {
	root := acsassert.RepoRoot(t)
	namedTest := filepath.Join(root, "go", "internal", "reachabilityprobe", "apicover_named_test.go")

	acsassert.FileExists(t, namedTest)
	acsassert.FileContains(t, namedTest, "BuildImportGraph")
}

func keys(g reachabilityprobe.ImportGraph) []string {
	out := make([]string, 0, len(g))
	for k := range g {
		out = append(out, k)
	}
	return out
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}
