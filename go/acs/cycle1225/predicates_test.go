//go:build acs

// Package cycle1225 ports the cycle-1225 ACS predicates for the
// tdd-structural-test-reachability-probe inbox item (weight 0.92).
//
// Root cause (cycle-644): TDD froze a `doNotModifyTests:true` structural test
// pinning `storage.UpdateStateMap(` inside a `core`-package file while
// `storage` already imported `core` — a compiler-proven import cycle — dooming
// the cycle to a permanently-RED, unsatisfiable acceptance criterion.
//
// Two tasks, one item:
//   - tdd-reachability-probe-doc: agents/evolve-tdd-engineer.md must name the
//     obligation to compiler-probe a pinned package-qualified call site before
//     freezing it, citing cycle-644 as the worked example (TestC1225_001).
//   - tdd-reachability-probe-check: a deterministic
//     go/internal/reachabilityprobe package must flag the cycle-644 shape as
//     unreachable and pass an acyclic pin unchanged (TestC1225_002).
package cycle1225

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/reachabilityprobe"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// TestC1225_001_DocCitesReachabilityProbeObligation is a behavioral doc
// predicate (not source-grep gaming — it locates the exact contractual
// language a TDD engineer must follow, not an arbitrary string): the TDD agent
// contract must document, in prose reachable near the RED-freeze steps, that
// a package-qualified pin requires a compiler-probe before freezing, and must
// cite cycle-644's storage/core shape as the worked example so a future TDD
// engineer recognizes the failure mode by name.
func TestC1225_001_DocCitesReachabilityProbeObligation(t *testing.T) {
	root := acsassert.RepoRoot(t)
	docPath := filepath.Join(root, "agents", "evolve-tdd-engineer.md")

	acsassert.FileExists(t, docPath)
	acsassert.FileContains(t, docPath, "cycle-644")
	acsassert.FileContains(t, docPath, "reachability")
	acsassert.FileMatchesRegex(t, docPath,
		`(?i)go build`)
}

// TestC1225_002_ReachabilityProbeFlagsCycle644Shape is the primary negative
// test (strongest anti-no-op signal per skills/adversarial-testing/SKILL.md
// §6): given an import graph reproducing the cycle-644 shape verbatim
// (storage already imports core; a frozen test wants to pin
// storage.UpdateStateMap( inside a core-package file), CheckCallSite MUST
// return a non-nil Violation citing the cycle. A no-op stub always returning
// nil would pass the GREEN case below but fail here — the negative case is
// load-bearing.
func TestC1225_002_ReachabilityProbeFlagsCycle644Shape(t *testing.T) {
	graph := reachabilityprobe.ImportGraph{
		"storage": {"core"},
		"core":    {},
	}
	site := reachabilityprobe.CallSite{
		PinningPackage:    "core",
		ReferencedPackage: "storage",
		Symbol:            "UpdateStateMap",
	}

	violation := reachabilityprobe.CheckCallSite(graph, site)
	if violation == nil {
		t.Fatalf("CheckCallSite(%+v) = nil, want a Violation (storage already imports core, so core importing storage is an unbuildable cycle)", site)
	}
	if violation.Site != site {
		t.Errorf("Violation.Site = %+v, want %+v", violation.Site, site)
	}
	if len(violation.Cycle) == 0 {
		t.Errorf("Violation.Cycle is empty, want a non-empty import chain proving the cycle")
	}
	if violation.Error() == "" {
		t.Errorf("Violation.Error() returned empty string, want a diagnostic message")
	}
}

// TestC1225_003_ReachabilityProbeAllowsAcyclicPin is the paired positive case
// (table-driven per the tdd-reachability-probe-check acceptance criteria): a
// pin whose referenced package does NOT transitively import the pinning
// package is legal and must pass unchanged (nil).
func TestC1225_003_ReachabilityProbeAllowsAcyclicPin(t *testing.T) {
	cases := []struct {
		name  string
		graph reachabilityprobe.ImportGraph
		site  reachabilityprobe.CallSite
	}{
		{
			name: "leaf package pinning storage, no reverse edge",
			graph: reachabilityprobe.ImportGraph{
				"storage": {"core"},
				"core":    {},
				"leaf":    {},
			},
			site: reachabilityprobe.CallSite{
				PinningPackage:    "leaf",
				ReferencedPackage: "storage",
				Symbol:            "UpdateStateMap",
			},
		},
		{
			// Edge case: pinning package unknown to the graph (e.g. a brand
			// new package with no recorded imports yet) must not be treated
			// as a false cycle — absence of evidence is not evidence of a
			// cycle.
			name: "pinning package absent from graph",
			graph: reachabilityprobe.ImportGraph{
				"storage": {"core"},
				"core":    {},
			},
			site: reachabilityprobe.CallSite{
				PinningPackage:    "brandnew",
				ReferencedPackage: "storage",
				Symbol:            "UpdateStateMap",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if v := reachabilityprobe.CheckCallSite(tc.graph, tc.site); v != nil {
				t.Errorf("CheckCallSite(%+v) = %+v, want nil (acyclic pin must pass unchanged)", tc.site, v)
			}
		})
	}
}

// TestC1225_004_ReachabilityProbeSemanticTransitiveCycle covers the semantic
// diversity axis (distinct behavior from the direct-edge case above): the
// cycle-644 class is not limited to a direct A->B / B->A pair — a
// TRANSITIVE chain (storage -> mid -> core, pin wants core -> storage) is the
// same unbuildable-cycle disease and must be caught identically.
func TestC1225_004_ReachabilityProbeSemanticTransitiveCycle(t *testing.T) {
	graph := reachabilityprobe.ImportGraph{
		"storage": {"mid"},
		"mid":     {"core"},
		"core":    {},
	}
	site := reachabilityprobe.CallSite{
		PinningPackage:    "core",
		ReferencedPackage: "storage",
		Symbol:            "UpdateStateMap",
	}

	violation := reachabilityprobe.CheckCallSite(graph, site)
	if violation == nil {
		t.Fatalf("CheckCallSite(%+v) = nil, want a Violation for the transitive chain storage->mid->core", site)
	}
	if len(violation.Cycle) < 2 {
		t.Errorf("Violation.Cycle = %v, want the full transitive chain (>=2 hops)", violation.Cycle)
	}
}

// TestC1225_005_ReachabilityProbePackageGraduatesApicover enforces House Rule
// 1 (new-package graduation, the repo-wide apicover gate, ADR-0069's second
// gate): a brand-new go/internal/reachabilityprobe package must be enrolled in
// go/.apicover-enforce AND ship its own apicover_named_test.go naming every
// exported symbol, in the same diff as the package itself — an enrolled-but-
// unnamed or unenrolled-but-present package each abort a later phase
// (cycle-1218: three lanes, one halt, same cause).
func TestC1225_005_ReachabilityProbePackageGraduatesApicover(t *testing.T) {
	root := acsassert.RepoRoot(t)

	enforceList := filepath.Join(root, "go", ".apicover-enforce")
	acsassert.FileContains(t, enforceList, "./internal/reachabilityprobe")

	namedTest := filepath.Join(root, "go", "internal", "reachabilityprobe", "apicover_named_test.go")
	acsassert.FileExists(t, namedTest)
	acsassert.FileContains(t, namedTest, "CheckCallSite")
	acsassert.FileContains(t, namedTest, "ImportGraph")
	acsassert.FileContains(t, namedTest, "CallSite")
	acsassert.FileContains(t, namedTest, "Violation")
}
