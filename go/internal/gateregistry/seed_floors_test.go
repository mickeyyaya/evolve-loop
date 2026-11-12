package gateregistry

import (
	"context"
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/repocontract"
)

const laneKey = "internal/policy.TestLaneBreaks"

type floorWant struct {
	name   string
	mark   string
	files  []string
	inputs []gatecontract.Declared
}

func floorInputs(verified ...string) []gatecontract.Declared {
	inputs := []gatecontract.Declared{{Path: "go/", Class: gatecontract.ClassSubject}}
	for _, path := range verified {
		inputs = append(inputs, gatecontract.Declared{Path: path, Class: gatecontract.ClassVerified})
	}
	return inputs
}

var floorWants = []floorWant{
	{
		name:   "repo-contract-floor",
		mark:   laneKey,
		files:  []string{"go/internal/core/repo_contract_floor.go", "go/internal/phases/ship/repocontract.go"},
		inputs: floorInputs(),
	},
	{
		name:   "unit-test-floor",
		mark:   "./internal/policy: unit tests FAIL",
		files:  []string{"go/internal/core/base_red_floor.go", "go/internal/core/build_floor_reviewer.go"},
		inputs: floorInputs("go/.apicover-enforce"),
	},
	{
		name:   "cover-changed-floor",
		mark:   "the tests of the changed packages fail under -tags integration",
		files:  []string{"go/internal/core/base_red_floor.go", "go/internal/core/cover_changed_floor.go"},
		inputs: floorInputs("go/.cover-strict"),
	},
}

const floorModule = "module github.com/mickeyyaya/evolve-loop/go\n\ngo 1.23\n"

const floorSource = "package policy\n\nfunc Value() int { return 1 }\n"

const floorGreenTest = "package policy\n\nimport \"testing\"\n\nfunc TestValue(t *testing.T) {\n\tif Value() != 1 {\n\t\tt.Fatal(\"value\")\n\t}\n}\n"

const floorRedTest = "package policy\n\nimport \"testing\"\n\nfunc TestLaneBreaks(t *testing.T) {\n\tt.Fatal(\"lane red\")\n}\n"

func packStubs() map[string]string {
	dirs := make([]string, 0, len(repocontract.Packages()))
	for _, pattern := range repocontract.Packages() {
		dirs = append(dirs, strings.TrimSuffix(pattern, "/..."))
	}
	for _, selection := range repocontract.TreeReadingTests() {
		dirs = append(dirs, selection.Package)
	}
	stubs := map[string]string{}
	for _, dir := range dirs {
		rel := strings.TrimPrefix(dir, "./")
		stubs["go/"+rel+"/doc.go"] = "package " + path.Base(rel) + "\n"
	}
	return stubs
}

func floorRepo(t *testing.T) (root, base string) {
	t.Helper()
	repo := gittest.Fixture(t)
	lay(t, repo.Dir, packStubs())
	lay(t, repo.Dir, map[string]string{
		"go/go.mod":                         floorModule,
		"go/internal/policy/policy.go":      floorSource,
		"go/internal/policy/policy_test.go": floorGreenTest,
	})
	repo.Git("add", "-A")
	repo.Git("commit", "-m", "base")
	return repo.Dir, strings.TrimSpace(repo.Git("rev-parse", "HEAD"))
}

func redsHold(reds []string, mark string) bool {
	return slices.ContainsFunc(reds, func(red string) bool { return strings.Contains(red, mark) })
}

func TestRegistry_FloorEntriesJudgeAFixtureAndReportAFailedReadAsAnError(t *testing.T) {
	for _, want := range floorWants {
		t.Run(want.name, func(t *testing.T) {
			run := entryNamed(t, want.name).Run
			root, base := floorRepo(t)
			lay(t, root, map[string]string{"go/internal/policy/lane_test.go": floorRedTest})
			verdict, err := run(context.Background(), gatecontract.Input{Root: root, Base: base})
			if err != nil || !redsHold(verdict.Reds, want.mark) {
				t.Errorf("Run over a lane with a failing test = %+v, %v, want a red that holds %q and no error", verdict, err, want.mark)
			}
			if _, err := run(context.Background(), gatecontract.Input{Root: root + "/absent", Base: base}); err == nil {
				t.Errorf("Run over a missing tree = nil error, want an error")
			}
		})
	}
}

func TestRegistry_FloorEntriesGiveAGreenLaneNoLaneRed(t *testing.T) {
	for _, want := range floorWants {
		t.Run(want.name, func(t *testing.T) {
			root, base := floorRepo(t)
			verdict, err := entryNamed(t, want.name).Run(context.Background(), gatecontract.Input{Root: root, Base: base})
			if err != nil || redsHold(verdict.Reds, want.mark) || redsHold(verdict.Reds, laneKey) {
				t.Errorf("Run over an unchanged tree = %+v, %v, want no lane red and no error", verdict, err)
			}
		})
	}
}

func TestRegistry_FloorEntriesReportAnUnreadableBaseAsAnErrorAndAGreenLaneAsNoRed(t *testing.T) {
	for _, want := range floorWants[1:] {
		t.Run(want.name, func(t *testing.T) {
			run := entryNamed(t, want.name).Run
			root, base := floorRepo(t)
			if verdict, err := run(context.Background(), gatecontract.Input{Root: root, Base: strings.Repeat("0", 40)}); err == nil || len(verdict.Reds) != 0 {
				t.Errorf("Run with an unknown base = %+v, %v, want an error and no red", verdict, err)
			}
			if verdict, err := run(context.Background(), gatecontract.Input{Root: root, Base: base}); err != nil || len(verdict.Reds) != 0 {
				t.Errorf("Run over an unchanged tree = %+v, %v, want no red and no error", verdict, err)
			}
		})
	}
}

func TestRegistry_FloorEntriesAreEnforcedBuildPhaseFloorsOverTheirFiles(t *testing.T) {
	t.Parallel()
	names := make([]string, 0, len(floorWants))
	for _, want := range floorWants {
		names = append(names, want.name)
		e := entryNamed(t, want.name)
		if e.Kind != gatecontract.KindPhaseFloor || !slices.Equal(e.Owners, []gatecontract.Owner{{Pattern: "*", Phase: "build"}}) {
			t.Errorf("%s: kind %q, owners %+v, want a phase floor that the build phase owns for each lane", want.name, e.Kind, e.Owners)
		}
		if !slices.Equal(e.Files, want.files) {
			t.Errorf("%s: files %q, want %q", want.name, e.Files, want.files)
		}
		if !slices.Equal(e.Inputs, want.inputs) {
			t.Errorf("%s: inputs %+v, want %+v", want.name, e.Inputs, want.inputs)
		}
	}
	if problems := stageProblems(Entries(), names); len(problems) > 0 {
		t.Errorf("a floor entry is not at enforce:\n%s", strings.Join(problems, "\n"))
	}
}
