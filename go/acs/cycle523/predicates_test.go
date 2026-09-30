//go:build acs

package cycle523

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func laneOf(specs []fleet.CycleSpec, id string) int {
	for i, s := range specs {
		for _, x := range s.Scope {
			if x == id {
				return i
			}
		}
	}
	return -1
}

func scopesOf(specs []fleet.CycleSpec) [][]string {
	out := make([][]string, len(specs))
	for i, s := range specs {
		out[i] = s.Scope
	}
	return out
}

func TestC523_001_OverlappingDeclaredFilesCollapseToOneLane(t *testing.T) {
	decisionJSON := []byte(`{"top_n":[
		{"id":"alpha","files":["go/internal/fleet/triageplan.go"]},
		{"id":"beta","files":["go/internal/fleet/triageplan.go"]}
	]}`)
	specs, err := fleet.PlanFromTriage(decisionJSON, nil, 2)
	if err != nil {
		t.Fatalf("PlanFromTriage returned error: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("two cards declaring the SAME file must collapse to 1 lane, got %d lanes %v — PlanFromTriage is still keying partitioning on the id-as-file placeholder instead of each card's declared files[]", len(specs), scopesOf(specs))
	}
	if a, b := laneOf(specs, "alpha"), laneOf(specs, "beta"); a != b || a < 0 {
		t.Errorf("alpha (lane %d) and beta (lane %d) share a declared file but are not co-located; scopes=%v", a, b, scopesOf(specs))
	}
}

func TestC523_002_DisjointDeclaredFilesSpreadToCountLanes(t *testing.T) {
	decisionJSON := []byte(`{"top_n":[
		{"id":"alpha","files":["go/internal/fleet/a.go"]},
		{"id":"beta","files":["go/internal/fleet/b.go"]}
	]}`)
	specs, err := fleet.PlanFromTriage(decisionJSON, nil, 2)
	if err != nil {
		t.Fatalf("PlanFromTriage returned error: %v", err)
	}
	if len(specs) != 2 {
		t.Fatalf("two cards with disjoint declared files and count=2 must spread to 2 lanes, got %d %v", len(specs), scopesOf(specs))
	}
	if a, b := laneOf(specs, "alpha"), laneOf(specs, "beta"); a == b {
		t.Errorf("disjoint cards alpha and beta collapsed into the same lane %d; scopes=%v", a, scopesOf(specs))
	}
}

func TestC523_003_NoDeclaredFilesFallsBackToIdIsland(t *testing.T) {
	decisionJSON := []byte(`{"top_n":[
		{"id":"gamma"},
		{"id":"delta"}
	]}`)
	specs, err := fleet.PlanFromTriage(decisionJSON, nil, 2)
	if err != nil {
		t.Fatalf("PlanFromTriage returned error: %v", err)
	}
	if len(specs) != 2 {
		t.Fatalf("two file-less id-distinct cards must fall back to id-islands and spread to 2 lanes, got %d %v", len(specs), scopesOf(specs))
	}
	if a, b := laneOf(specs, "gamma"), laneOf(specs, "delta"); a == b {
		t.Errorf("file-less cards gamma and delta must remain separate id-islands but collapsed into lane %d; scopes=%v", a, scopesOf(specs))
	}
}

func TestC523_004_CountBelowTwoCollapsesAllToSingleLane(t *testing.T) {
	decisionJSON := []byte(`{"top_n":[
		{"id":"alpha","files":["go/internal/fleet/a.go"]},
		{"id":"beta","files":["go/internal/fleet/b.go"]}
	]}`)
	specs, err := fleet.PlanFromTriage(decisionJSON, nil, 1)
	if err != nil {
		t.Fatalf("PlanFromTriage returned error: %v", err)
	}
	if len(specs) != 1 {
		t.Fatalf("count=1 must yield exactly 1 lane regardless of declared files, got %d %v", len(specs), scopesOf(specs))
	}
	if laneOf(specs, "alpha") != 0 || laneOf(specs, "beta") != 0 {
		t.Errorf("count=1 must place every card in the single lane; scopes=%v", scopesOf(specs))
	}
}

func TestC523_005_MixedBacklogGroupsOverlapAndIsolatesDisjoint(t *testing.T) {
	decisionJSON := []byte(`{"top_n":[
		{"id":"alpha","files":["go/internal/fleet/shared.go"]},
		{"id":"beta","files":["go/internal/fleet/shared.go"]},
		{"id":"gamma","files":["go/internal/fleet/other.go"]}
	]}`)
	specs, err := fleet.PlanFromTriage(decisionJSON, nil, 2)
	if err != nil {
		t.Fatalf("PlanFromTriage returned error: %v", err)
	}
	if len(specs) != 2 {
		t.Fatalf("overlapping pair + one disjoint card at count=2 must yield exactly 2 lanes (pair collapses, disjoint isolated), got %d %v", len(specs), scopesOf(specs))
	}
	a, b, g := laneOf(specs, "alpha"), laneOf(specs, "beta"), laneOf(specs, "gamma")
	if a != b || a < 0 {
		t.Errorf("alpha (lane %d) and beta (lane %d) share a declared file and must co-locate; scopes=%v", a, b, scopesOf(specs))
	}
	if g == a {
		t.Errorf("disjoint gamma (lane %d) must NOT be swept into the overlap lane %d; scopes=%v", g, a, scopesOf(specs))
	}
}

func TestC523_006_FleetSuiteStaysGreen(t *testing.T) {
	root := acsassert.RepoRoot(t)
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-C", root+"/go", "-count=1", "./internal/fleet/")
	if err != nil || code != 0 {
		t.Fatalf("go test ./internal/fleet/ failed (code=%d err=%v)\nstdout:\n%s\nstderr:\n%s", code, err, stdout, stderr)
	}
}

func TestC523_007_FleetPackageVetsClean(t *testing.T) {
	root := acsassert.RepoRoot(t)
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "vet", "-C", root+"/go", "./internal/fleet/")
	if err != nil || code != 0 {
		t.Fatalf("go vet ./internal/fleet/ reported problems (code=%d err=%v)\nstdout:\n%s\nstderr:\n%s", code, err, stdout, stderr)
	}
}
