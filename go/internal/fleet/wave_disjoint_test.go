package fleet

// wave_disjoint_test.go — fleet-s3-guards AC4 (cycle 467): the WAVE-LEVEL
// disjointness regression pin. TestPartition_CrossBucketFileDisjoint pins the
// invariant at the bucket level only; nothing pinned it on the []CycleSpec
// output of PlanWaves / PlanFromTriage — the shape the wave launcher actually
// consumes (scout Key Finding 5). These are regression pins over EXISTING
// behavior (expected pre-existing GREEN once the package compiles): if a
// future change lets two specs of one wave share a file, concurrent lanes
// would collide on the shared tree at ship time.

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

// specFiles maps a spec's scoped todo IDs back to the files those todos
// declared, so disjointness is asserted on FILES (the collision unit), not
// just IDs.
func specFiles(t *testing.T, spec CycleSpec, byID map[string]Todo) map[string]bool {
	t.Helper()
	files := map[string]bool{}
	for _, id := range spec.Scope {
		td, ok := byID[id]
		if !ok {
			t.Fatalf("spec scope references unknown todo id %q", id)
		}
		for _, f := range td.Files {
			files[f] = true
		}
	}
	return files
}

// assertWaveFileDisjoint fails if any file is reachable from two DISTINCT
// specs of the same wave.
func assertWaveFileDisjoint(t *testing.T, wave []CycleSpec, byID map[string]Todo) {
	t.Helper()
	owner := map[string]int{}
	for si, spec := range wave {
		for f := range specFiles(t, spec, byID) {
			if prev, ok := owner[f]; ok && prev != si {
				t.Errorf("file %q co-scheduled in specs %d AND %d of one wave — concurrent lanes would collide", f, prev, si)
			}
			owner[f] = si
		}
	}
}

// TestPlanWaves_WaveLevelFileDisjoint: within ONE wave, todos that
// (transitively) share a file must land in the SAME spec — never spread
// across two concurrently-launched specs. A file MAY reappear in a LATER
// wave (dependency-ordered, runs after the earlier wave lands): the
// invariant is per-wave, not global.
func TestPlanWaves_WaveLevelFileDisjoint(t *testing.T) {
	todos := []Todo{
		{ID: "a", Files: []string{"f1.go"}},
		{ID: "b", Files: []string{"f1.go", "f2.go"}}, // bridges a and c transitively
		{ID: "c", Files: []string{"f2.go"}},
		{ID: "d", Files: []string{"f3.go"}},
		{ID: "e", Files: []string{"f1.go"}, DependsOn: []string{"a"}}, // later wave reuses f1 — allowed
	}
	byID := map[string]Todo{}
	for _, td := range todos {
		byID[td.ID] = td
	}
	waves, err := PlanWaves(todos)
	if err != nil {
		t.Fatalf("PlanWaves: %v", err)
	}
	if len(waves) != 2 {
		t.Fatalf("len(waves) = %d, want 2 (e depends on a)", len(waves))
	}
	for wi, wave := range waves {
		assertWaveFileDisjoint(t, wave, byID)
		if wi == 0 {
			// a,b,c share files transitively → exactly one spec; d is disjoint →
			// its own spec. A wave that merged everything into one spec would
			// hide the concurrency; one that split a/b/c would collide.
			if len(wave) != 2 {
				t.Fatalf("wave 0: len(specs) = %d, want 2 (merged {a,b,c} + {d})", len(wave))
			}
		}
	}
}

// TestPlanFromTriage_WaveLevelFileDisjoint: PlanFromTriage's single-wave
// []CycleSpec output must never scope the same todo id (== file scope; each
// triage todo's Files is its own id) into two specs, even when the decision
// repeats an id across its sources — the duplicate must collapse, not
// co-schedule.
func TestPlanFromTriage_WaveLevelFileDisjoint(t *testing.T) {
	decision := []byte(`{"committed_floors":["bridge","core","bridge","audit"]}`)
	specs, err := PlanFromTriage(decision, []string{"core"}, 3)
	if err != nil {
		t.Fatalf("PlanFromTriage: %v", err)
	}
	if len(specs) == 0 || len(specs) > 3 {
		t.Fatalf("len(specs) = %d, want 1..3", len(specs))
	}
	owner := map[string]int{}
	covered := map[string]bool{}
	for si, spec := range specs {
		for _, id := range strings.Split(spec.Env[ipcenv.FleetScopeKey], ",") {
			if id == "" {
				continue
			}
			if prev, ok := owner[id]; ok && prev != si {
				t.Errorf("todo id %q co-scheduled in specs %d AND %d of one wave", id, prev, si)
			}
			owner[id] = si
			covered[id] = true
		}
	}
	for _, want := range []string{"bridge", "core", "audit"} {
		if !covered[want] {
			t.Errorf("todo id %q missing from the wave's scopes — dedup must collapse duplicates, not drop work", want)
		}
	}
}
