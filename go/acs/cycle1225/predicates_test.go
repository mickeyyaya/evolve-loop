//go:build acs

package cycle1225

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/reachabilityprobe"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC1225_001_DocCitesReachabilityProbeObligation(t *testing.T) {
	root := acsassert.RepoRoot(t)
	docPath := filepath.Join(root, "agents", "evolve-tdd-engineer.md")

	acsassert.FileExists(t, docPath)
	acsassert.FileContains(t, docPath, "cycle-644")
	acsassert.FileContains(t, docPath, "reachability")
	acsassert.FileMatchesRegex(t, docPath,
		`(?i)go build`)
}

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
