//go:build acs

// Package cycle523 materialises the cycle-523 acceptance criteria for the
// SINGLE triage-committed task (this lane's assigned fleet_scope id):
//
//	wave-seed-partitions-on-id-not-real-files — populate fleet.Todo.Files from
//	each triage-decision top_n[] card's declared files[] in
//	fleet.PlanFromTriage (go/internal/fleet/triageplan.go, today
//	Todo{ID: id, Files: []string{id}}), falling back to []string{id} only when
//	a card declares no files, so overlapping declared files collapse to ONE
//	partition lane while disjoint files still spread to `count` lanes.
//	→ C523_001..007
//
// TASK BINDING (cycle-522 lesson): cycle 522 FAILed because TDD bound to
// scout-report's broader "Task 2" while Builder bound to triage's narrower
// committed top_n. This file binds to the triage-report `## top_n` id
// (`wave-seed-partitions-on-id-not-real-files`) ONLY — NOT scout's bundled
// treediff-guard task nor the 0.94 cmd_loop_wave.go half, both of which
// triage-report `## deferred` to sibling lanes/future cycles. Predicates bind
// only to triage-committed work (R9.3).
//
// 1:1 AC-materialization (see the eval
// .evolve/evals/wave-seed-partitions-on-id-not-real-files.md): 7 predicates +
// 0 manual+checklist + 0 removed = 7 ACs total, none double-counted.
//
// Why these predicates exercise the SUT directly (cycle-85 predicate-quality):
// every load-bearing predicate CALLS fleet.PlanFromTriage in-process with a
// crafted triage-decision.json and asserts on the returned []CycleSpec lane
// grouping — never a "source file contains text X" check. The id-as-file
// placeholder the fix removes means today's code groups lanes by unique id,
// so an overlap-collapse assertion is RED now and GREEN only once real
// declared files[] are threaded through.
//
// RED strategy (verified in test-report.md "RED Run Output"): the package
// COMPILES against the current PlanFromTriage signature (adding a `files` JSON
// key to top_n cards is not a signature change; json.Unmarshal ignores the
// unknown field today), so C523_001 and C523_005 are RED on their ASSERTIONS
// — two cards sharing a declared file still land in SEPARATE lanes because the
// current code keys partitioning on the id-as-file placeholder, not the file.
// C523_002/003/004 are pre-existing-GREEN behavior-preservation pins (the fix
// must NOT break disjoint-spread, the no-files id fallback, or count<2
// single-lane collapse). C523_006/007 are repo-gate pins (fleet suite +
// vet stay green through the change).
//
// Adversarial diversity (skills/adversarial-testing SKILL §6):
//
//	Negative:   C523_005 — a disjoint card must NEVER be swept into the
//	            overlap lane, and an overlapping pair must NEVER be split; the
//	            single assertion kills BOTH the always-spread no-op (id-as-file
//	            → alpha/beta split) and the always-collapse fake (everything in
//	            one lane). C523_004 — count<2 must NOT fabricate extra lanes.
//	Edge / OOD: C523_003 — cards declaring NO files[] fall back to the id
//	            island (unknown-footprint boundary); C523_004 — count=1.
//	Semantic:   C523_001 (overlap → 1 lane) vs C523_002 (disjoint → 2 lanes)
//	            are DISTINCT behaviors driven only by the declared files — a
//	            fake that always returns the same lane count passes one and
//	            fails the other.
package cycle523

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// laneOf returns the index of the single spec whose Scope owns id, or -1 when
// no returned lane claims it. Partition guarantees an id appears in at most one
// spec, so the first match is authoritative.
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

// scopesOf renders every returned lane's Scope for failure diagnostics.
func scopesOf(specs []fleet.CycleSpec) [][]string {
	out := make([][]string, len(specs))
	for i, s := range specs {
		out[i] = s.Scope
	}
	return out
}

// TestC523_001_OverlappingDeclaredFilesCollapseToOneLane (AC-1, positive —
// THE fix). Two top_n cards declaring the SAME repo file must co-locate in ONE
// partition lane. This is RED against today's code: PlanFromTriage sets
// Todo{Files: []string{id}}, so "alpha" and "beta" are trivially file-disjoint
// (each "file" is its own id) and spread to two lanes. It goes GREEN only when
// the card's declared files[] populate Todo.Files, letting fleet.Partition see
// the shared file and cluster the pair. Gaming fake killed: keeping the
// id-as-file placeholder (the exact defect this task removes).
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

// TestC523_002_DisjointDeclaredFilesSpreadToCountLanes (AC-2, positive —
// baseline the fix must preserve). Cards touching DISJOINT files still spread
// to `count` independent lanes, so the fix does not over-collapse. Pre-existing
// GREEN (distinct ids already spread today), retained as a regression pin: an
// implementation that force-collapses everything to satisfy C523_001 would
// break this.
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

// TestC523_003_NoDeclaredFilesFallsBackToIdIsland (AC-3, edge — fallback
// preservation). A card that declares NO files[] falls back to []string{id}
// (an island unique to itself), so two id-distinct file-less cards remain
// independent and spread. Pre-existing GREEN behavior-preservation pin: the fix
// must keep the id fallback for cards with no declared footprint, not error or
// force them together.
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

// TestC523_004_CountBelowTwoCollapsesAllToSingleLane (AC-4, edge — legacy
// preservation). count<2 must reproduce today's single-lane behavior byte-for-
// byte regardless of declared files: even disjoint-file cards land in ONE spec
// carrying every id. Pre-existing GREEN pin (PlanCycles' n<1→1 branch), retained
// so the files change cannot leak an extra lane at count=1.
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

// TestC523_005_MixedBacklogGroupsOverlapAndIsolatesDisjoint (AC-5, negative /
// anti-gaming — the strongest predicate). Backlog: {alpha,beta} share a file,
// gamma is disjoint. With count=2 the planner must produce EXACTLY 2 lanes —
// {alpha,beta} co-located, {gamma} alone. RED today: with the id-as-file
// placeholder all three ids are distinct "files", so Partition spreads them
// least-loaded into {alpha,gamma},{beta} — alpha and beta end up SPLIT. This
// single assertion kills BOTH gaming fakes at once: the always-spread no-op
// (leaves the placeholder → alpha/beta split, fails the co-location check) and
// the always-collapse shortcut (one lane → fails len==2 and sweeps gamma in).
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

// TestC523_006_FleetSuiteStaysGreen (AC-6, repo-gate pin — scout AC-8). The
// whole fleet package suite, INCLUDING the new overlap-collapse regression the
// task mandates in partition_test.go, must pass. Exercises the real toolchain
// via a subprocess (`go test -C <root>/go ./internal/fleet/`) — not a source
// text scan.
func TestC523_006_FleetSuiteStaysGreen(t *testing.T) {
	root := acsassert.RepoRoot(t)
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-C", root+"/go", "-count=1", "./internal/fleet/")
	if err != nil || code != 0 {
		t.Fatalf("go test ./internal/fleet/ failed (code=%d err=%v)\nstdout:\n%s\nstderr:\n%s", code, err, stdout, stderr)
	}
}

// TestC523_007_FleetPackageVetsClean (AC-7, repo-gate pin — scout AC-9). The
// fleet package must stay `go vet`-clean through the change. Subprocess against
// the real toolchain.
func TestC523_007_FleetPackageVetsClean(t *testing.T) {
	root := acsassert.RepoRoot(t)
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "vet", "-C", root+"/go", "./internal/fleet/")
	if err != nil || code != 0 {
		t.Fatalf("go vet ./internal/fleet/ reported problems (code=%d err=%v)\nstdout:\n%s\nstderr:\n%s", code, err, stdout, stderr)
	}
}
