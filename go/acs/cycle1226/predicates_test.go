//go:build acs

package cycle1226

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/reachabilityprobe"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func moduleRoot(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

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
			name:      "sysexec pinning fleet is an unbuildable cycle (real edge)",
			site:      reachabilityprobe.CallSite{PinningPackage: sysexecPkg, ReferencedPackage: fleetPkg, Symbol: "TransitivePackageSet"},
			wantCycle: true,
		},
		{
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

func TestC1226_004_ReachabilityProbeRaceClean(t *testing.T) {
	root := moduleRoot(t)
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "-C", root, "test", "-race", "-tags", "acs", "./internal/reachabilityprobe")
	if err != nil {
		t.Errorf("go test -race ./internal/reachabilityprobe exited %d: %v\nstdout: %s\nstderr: %s", code, err, stdout, stderr)
	}
}

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
