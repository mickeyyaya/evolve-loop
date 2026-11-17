package main

import (
	"context"
	"io"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/admission"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func fileCellsOrchestrator(t *testing.T, fileCells string) policy.OrchestratorConfig {
	t.Helper()
	p, err := policy.Parse([]byte(`{"fleet":{"count":2,"orchestrator":{"stage":"enforce","admission":"enforce","file_cells":` + fileCells + `}}}`))
	if err != nil {
		t.Fatal(err)
	}
	return p.FleetConfig().Orchestrator
}

func evolveFilesTodos() (running, sameFile, otherFile fleet.Todo) {
	return fleet.Todo{ID: "lane-a", Files: []string{"go/cmd/evolve/a.go"}},
		fleet.Todo{ID: "same-file", Files: []string{"go/cmd/evolve/a.go"}},
		fleet.Todo{ID: "other-file", Files: []string{"go/cmd/evolve/b.go"}}
}

func TestCellAdmission_PolicyFileCellsLetTwoFilesOfCmdEvolveRunBesideEachOther(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		fileCells string
		otherFile int
	}{
		"file_cells empty":         {`[]`, -1},
		"file_cells names the cmd": {`["cmd/evolve"]`, 0},
	} {
		t.Run(name, func(t *testing.T) {
			b, _, _ := supplyCoordinator(t, t.TempDir())
			b.loadGraph = coreRouterGraph
			running, sameFile, otherFile := evolveFilesTodos()
			admit := b.cellAdmission(fileCellsOrchestrator(t, tc.fileCells), 1, nil, nil)
			if got := admit([]fleet.Todo{running}, []fleet.Todo{otherFile}); got != tc.otherFile {
				t.Errorf("admit of b.go beside a lane on a.go = %d, want %d", got, tc.otherFile)
			}
			if got := admit([]fleet.Todo{running}, []fleet.Todo{sameFile}); got != -1 {
				t.Errorf("admit of a.go beside a lane on a.go = %d, want -1", got)
			}
		})
	}
}

func TestAdmissionShadow_PolicyFileCellsReachTheWaveShadow(t *testing.T) {
	t.Parallel()
	fn := admissionShadow(func(oc policy.OrchestratorConfig, _ int) (*admission.ModuleGraph, error) {
		return coreRouterGraph(context.Background(), "", oc.HubMinImporters)
	})
	_, a, b := evolveFilesTodos()
	for name, tc := range map[string]struct {
		fileCells string
		admitted  int
		waits     []string
	}{
		"file_cells empty":         {`[]`, 1, []string{"other-file"}},
		"file_cells names the cmd": {`["cmd/evolve"]`, 2, nil},
	} {
		got, err := fn(1, []fleet.Todo{a, b}, func(string) string { return "" }, fileCellsOrchestrator(t, tc.fileCells))
		if err != nil {
			t.Fatal(err)
		}
		var waits []string
		for _, w := range got.Waits {
			waits = append(waits, w.ID)
		}
		if got.Admitted != tc.admitted || !reflect.DeepEqual(waits, tc.waits) {
			t.Errorf("%s: admitted %d, waits %v, want %d and %v", name, got.Admitted, waits, tc.admitted, tc.waits)
		}
	}
}

func TestSupplyWiring_TheCensusTakesTheFileCellsOfThePolicy(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	w := supplyWiring{root: root, evolveDir: filepath.Join(root, ".evolve"), now: func() time.Time { return time.Time{} }, stderr: io.Discard,
		fleet: policy.FleetConfig{Count: 2, Orchestrator: fileCellsOrchestrator(t, `["cmd/evolve"]`)}}
	if got, want := w.census().FileCells, []string{"cmd/evolve"}; !reflect.DeepEqual(got, want) {
		t.Errorf("census FileCells = %v, want %v", got, want)
	}
}
