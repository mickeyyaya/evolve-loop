package fleet

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

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
			if len(wave) != 2 {
				t.Fatalf("wave 0: len(specs) = %d, want 2 (merged {a,b,c} + {d})", len(wave))
			}
		}
	}
}

func TestPlanFromTriage_WaveLevelFileDisjoint(t *testing.T) {
	decision := []byte(`{"committed_floors":["bridge","core","bridge","audit"]}`)
	specs, _, err := PlanFromTriage(decision, []string{"core"}, 3, nil)
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
