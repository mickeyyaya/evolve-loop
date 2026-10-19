//go:build acs

package cycle1972

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/admission"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
)

const hubMinImporters = 4

var fixtureModule = map[string][]string{
	"internal/sysexec":   nil,
	"internal/ipcenv":    {"internal/sysexec"},
	"internal/clihealth": {"internal/sysexec"},
	"internal/inboxrank": {"internal/sysexec"},
	"internal/fleet":     {"internal/sysexec", "internal/inboxrank"},
	"internal/router":    {"internal/sysexec", "internal/ipcenv"},
	"cmd/evolve":         {"internal/sysexec", "internal/ipcenv", "internal/clihealth", "internal/inboxrank", "internal/fleet", "internal/router"},
}

type leafGraph interface{ IsLeaf(pkg string) bool }

func fixtureListing() string {
	var b strings.Builder
	for pkg, deps := range fixtureModule {
		full := []string{"fmt", "strings"}
		for _, d := range deps {
			full = append(full, admission.ModulePath+"/"+d)
		}
		b.WriteString(admission.ModulePath + "/" + pkg + "|" + strings.Join(full, " ") + "\n")
	}
	return b.String()
}

func fixtureGraph(t *testing.T) *admission.ModuleGraph {
	t.Helper()
	g, err := admission.NewModuleGraph(fixtureListing(), hubMinImporters)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func todo(id string, files ...string) fleet.Todo { return fleet.Todo{ID: id, Files: files} }

func admittedIDs(t *testing.T, running fleet.Todo, candidates ...fleet.Todo) (admitted, waiting []string) {
	t.Helper()
	a := admission.Admitter{Graph: fixtureGraph(t)}
	adm, wait := a.Admit([]admission.Holding{{Lane: "lane-1", Todo: running}}, candidates, 32)
	for _, d := range adm {
		admitted = append(admitted, d.ID)
	}
	for _, d := range wait {
		waiting = append(waiting, d.ID)
	}
	return admitted, waiting
}

func TestC1972_001_ALeafLaneDoesNotHoldAnUnrelatedGoTodo(t *testing.T) {
	cases := []struct {
		name    string
		running fleet.Todo
		cand    fleet.Todo
	}{
		{"a cmd/evolve go file holds no router todo", todo("lane", "go/cmd/evolve/cmd_wave.go"), todo("router", "go/internal/router/router.go")},
		{"a cmd/evolve data file holds no router todo", todo("lane", "go/cmd/evolve/testdata/golden.json"), todo("router", "go/internal/router/router.go")},
		{"a cmd/evolve go file holds no ipcenv todo", todo("lane", "go/cmd/evolve/cmd_wave.go"), todo("ipcenv", "go/internal/ipcenv/env.go")},
		{"a router lane does not hold a cmd/evolve todo", todo("lane", "go/internal/router/router.go"), todo("cmd", "go/cmd/evolve/cmd_wave.go")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			admitted, waiting := admittedIDs(t, tc.running, tc.cand)
			if !reflect.DeepEqual(admitted, []string{tc.cand.ID}) || len(waiting) != 0 {
				t.Errorf("admitted = %v, waiting = %v, want [%s] admitted: a leaf package has no reach and nothing compiles against it", admitted, waiting, tc.cand.ID)
			}
		})
	}
}

func TestC1972_002_AMixedTodoKeepsTheFullReachOfItsNonLeafPackage(t *testing.T) {
	mixed := todo("mixed", "go/cmd/evolve/cmd_wave.go", "go/internal/router/router.go")
	admitted, waiting := admittedIDs(t, mixed, todo("ipcenv", "go/internal/ipcenv/env.go"))
	if len(admitted) != 0 || !reflect.DeepEqual(waiting, []string{"ipcenv"}) {
		t.Errorf("admitted = %v, waiting = %v, want ipcenv held: router imports ipcenv and router is edited by the running todo", admitted, waiting)
	}
	admitted, waiting = admittedIDs(t, todo("ipcenv", "go/internal/ipcenv/env.go"), mixed)
	if len(admitted) != 0 || !reflect.DeepEqual(waiting, []string{"mixed"}) {
		t.Errorf("admitted = %v, waiting = %v, want the mixed todo held by a running ipcenv lane", admitted, waiting)
	}
}

func TestC1972_003_ANonLeafLaneStillHoldsAPackageInItsReach(t *testing.T) {
	cases := []struct {
		name    string
		running fleet.Todo
		cand    fleet.Todo
	}{
		{"router lane holds an ipcenv todo", todo("lane", "go/internal/router/router.go"), todo("ipcenv", "go/internal/ipcenv/env.go")},
		{"fleet lane holds an inboxrank todo", todo("lane", "go/internal/fleet/pool.go"), todo("inboxrank", "go/internal/inboxrank/rank.go")},
		{"ipcenv lane holds a router todo", todo("lane", "go/internal/ipcenv/env.go"), todo("router", "go/internal/router/router.go")},
		{"router lane holds a router todo", todo("lane", "go/internal/router/router.go"), todo("router2", "go/internal/router/other.go")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			admitted, waiting := admittedIDs(t, tc.running, tc.cand)
			if len(admitted) != 0 || !reflect.DeepEqual(waiting, []string{tc.cand.ID}) {
				t.Errorf("admitted = %v, waiting = %v, want %s held", admitted, waiting, tc.cand.ID)
			}
		})
	}
}

func TestC1972_004_TwoCmdEvolveTodosNeverRunAtOnceAndTheHubRuleHolds(t *testing.T) {
	admitted, waiting := admittedIDs(t, todo("lane", "go/cmd/evolve/cmd_wave.go"), todo("second", "go/cmd/evolve/cmd_loop.go"))
	if len(admitted) != 0 || !reflect.DeepEqual(waiting, []string{"second"}) {
		t.Errorf("admitted = %v, waiting = %v, want the second cmd/evolve todo held: same package cell", admitted, waiting)
	}
	admitted, waiting = admittedIDs(t, todo("lane", "go/go.mod"), todo("second", "go/Makefile"), todo("sysexec", "go/internal/sysexec/command.go"))
	if len(admitted) != 0 || !reflect.DeepEqual(waiting, []string{"second", "sysexec"}) {
		t.Errorf("admitted = %v, waiting = %v, want every hub todo held behind the running hub lane", admitted, waiting)
	}
	admitted, waiting = admittedIDs(t, todo("lane", "go/cmd/evolve/cmd_wave.go"), todo("hub", "go/internal/sysexec/command.go"))
	if len(admitted) != 1 || len(waiting) != 0 {
		t.Errorf("admitted = %v, waiting = %v, want a hub todo admitted beside a leaf lane: the leaf lane is no hub", admitted, waiting)
	}
}

func TestC1972_005_ModuleGraphIsLeafIsTrueOnlyForPackagesNothingImports(t *testing.T) {
	g, ok := any(fixtureGraph(t)).(leafGraph)
	if !ok {
		t.Fatal("ModuleGraph has no IsLeaf(pkg string) bool method")
	}
	cases := []struct {
		pkg  string
		want bool
	}{
		{"cmd/evolve", true},
		{"internal/router", false},
		{"internal/ipcenv", false},
		{"internal/sysexec", false},
		{"internal/fleet", false},
	}
	for _, tc := range cases {
		if got := g.IsLeaf(admission.ModulePath + "/" + tc.pkg); got != tc.want {
			t.Errorf("IsLeaf(%s) = %v, want %v", tc.pkg, got, tc.want)
		}
	}
}

func TestC1972_006_EveryPackageHoldingAGraphImplementationStillVets(t *testing.T) {
	goDir, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	for _, pkg := range []string{"./internal/admission", "./internal/loopwave", "./internal/orchestrator"} {
		t.Run(pkg, func(t *testing.T) {
			cmd := exec.Command("go", "vet", pkg)
			cmd.Dir = goDir
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Errorf("go vet %s failed: a Graph fake that lacks IsLeaf breaks CI `vet + fmt`:\n%s", pkg, out)
			}
		})
	}
}

func TestC1972_007_AMixedTodoKeepsTheFullReachOfItsLeafPackageToo(t *testing.T) {
	mixed := todo("mixed", "go/cmd/evolve/cmd_wave.go", "go/internal/router/router.go")
	for _, pkg := range []string{"clihealth", "inboxrank", "fleet"} {
		t.Run("a running mixed lane holds a "+pkg+" todo", func(t *testing.T) {
			cand := todo(pkg, "go/internal/"+pkg+"/file.go")
			admitted, waiting := admittedIDs(t, mixed, cand)
			if len(admitted) != 0 || !reflect.DeepEqual(waiting, []string{pkg}) {
				t.Errorf("admitted = %v, waiting = %v, want %s held: cmd/evolve imports it and the running todo edits cmd/evolve", admitted, waiting, pkg)
			}
		})
		t.Run("a running "+pkg+" lane holds the mixed todo", func(t *testing.T) {
			running := todo(pkg, "go/internal/"+pkg+"/file.go")
			admitted, waiting := admittedIDs(t, running, mixed)
			if len(admitted) != 0 || !reflect.DeepEqual(waiting, []string{"mixed"}) {
				t.Errorf("admitted = %v, waiting = %v, want the mixed todo held: its cmd/evolve edit reaches %s", admitted, waiting, pkg)
			}
		})
	}
}

func TestC1972_008_ALeafOnlyLaneStillAdmitsTwoUnrelatedGoTodos(t *testing.T) {
	leafOnly := todo("leaf", "go/cmd/evolve/cmd_wave.go")
	admitted, waiting := admittedIDs(t, leafOnly, todo("cli", "go/internal/clihealth/health.go"), todo("rt", "go/internal/router/router.go"))
	if !reflect.DeepEqual(admitted, []string{"cli", "rt"}) || len(waiting) != 0 {
		t.Errorf("admitted = %v, waiting = %v, want both unrelated Go todos admitted beside a leaf-only lane", admitted, waiting)
	}
}

// acs-predicate: config-check
func TestC1972_009_TheDesignDocsRecordApproachBTheRejectedOptionsAndTheWaveEvidence(t *testing.T) {
	goDir, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	repo := filepath.Dir(goDir)
	for _, rel := range []string{"docs/plans/wave-orchestrator-2026-10.md", "docs/architecture/wave-orchestrator.md"} {
		t.Run(rel, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(repo, rel))
			if err != nil {
				t.Fatal(err)
			}
			doc := strings.ToLower(string(raw))
			for _, want := range []string{"wave 131", "leaf", "approach (b)", "direct import", "exported"} {
				if !strings.Contains(doc, want) {
					t.Errorf("%s does not record %q: the item asks for approach (b), the rejected (a) direct-imports-only and (c) exported-API reach, and the wave 131 evidence", rel, want)
				}
			}
		})
	}
}
