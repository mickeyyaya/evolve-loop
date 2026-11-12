package gateregistry

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
)

const floorSubject = "go/"

type floorSpec struct {
	Name     string
	Files    []string
	Judge    core.FloorJudge
	Verified []string
}

func floorEntries() []gatecontract.Entry {
	return []gatecontract.Entry{
		floorEntry(floorSpec{
			Name:  "repo-contract-floor",
			Files: []string{"go/internal/core/repo_contract_floor.go", "go/internal/phases/ship/repocontract.go"},
			Judge: core.RepoContractFloorJudge(ship.RunRepoContractPack),
		}),
		floorEntry(floorSpec{
			Name:     "unit-test-floor",
			Files:    []string{"go/internal/core/base_red_floor.go", "go/internal/core/build_floor_reviewer.go"},
			Judge:    core.UnitTestFloorJudge,
			Verified: []string{"go/.apicover-enforce"},
		}),
		floorEntry(floorSpec{
			Name:     "cover-changed-floor",
			Files:    []string{"go/internal/core/base_red_floor.go", "go/internal/core/cover_changed_floor.go"},
			Judge:    core.CoverChangedFloorJudge,
			Verified: []string{"go/.cover-strict"},
		}),
	}
}

func floorEntry(spec floorSpec) gatecontract.Entry {
	inputs := []gatecontract.Declared{{Path: floorSubject, Class: gatecontract.ClassSubject}}
	for _, list := range spec.Verified {
		inputs = append(inputs, gatecontract.Declared{Path: list, Class: gatecontract.ClassVerified})
	}
	return gatecontract.Entry{
		Name:   spec.Name,
		Kind:   gatecontract.KindPhaseFloor,
		Files:  spec.Files,
		Owners: []gatecontract.Owner{{Pattern: "*", Phase: "build"}},
		Inputs: inputs,
		Stage:  gatecontract.StageEnforce,
		Run:    core.FloorEntryRun(spec.Judge),
	}
}
