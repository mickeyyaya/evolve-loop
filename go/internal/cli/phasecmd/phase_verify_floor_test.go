package phasecmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

type recordedFloor struct {
	roots    []string
	inputs   []core.ReviewInput
	failures []string
}

func (r *recordedFloor) floorFor(projectRoot string) core.BuildHandoffFloor {
	r.roots = append(r.roots, projectRoot)
	return core.BuildHandoffFloor{{Name: "recorded", Run: func(_ context.Context, in core.ReviewInput) []string {
		r.inputs = append(r.inputs, in)
		return r.failures
	}}}
}

func floorVerify(t *testing.T, floor BuildHandoffFloorFor, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := NewRunPhase(floor)(append([]string{"verify"}, args...), nil, &out, &errb)
	return code, out.String(), errb.String()
}

func floorWorkspace(t *testing.T) (root, ws string) {
	t.Helper()
	root = t.TempDir()
	ws = filepath.Join(root, ".evolve", "runs", "cycle-1788")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "build-report.md"), []byte("## Changes\n- go/internal/evalqualitycheck/unsatisfiable.go\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	t.Setenv("EVOLVE_CYCLE_STATE_FILE", "")
	return root, ws
}

func TestPhaseVerifyBuild_RunsTheInjectedFloorOnTheBoundCycleInTheFloorsWords(t *testing.T) {
	root, ws := floorWorkspace(t)
	state := core.CycleState{
		CycleID: 1788, Phase: "build", RunID: "run-1788", WorkspacePath: ws, ActiveWorktree: t.TempDir(),
		WorktreeBaseSHA: strings.Repeat("6", 40), ExplanationDocumentationVersion: 1,
	}
	body, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, core.RunStateFile), body, 0o644); err != nil {
		t.Fatal(err)
	}
	const floorLine = "Explanation Documentation: build-report.md is missing the required ## Explanation Documentation section"
	floor := &recordedFloor{failures: []string{floorLine}}
	code, _, errb := floorVerify(t, floor.floorFor, "build", "--workspace", ws)
	if code != 1 || !strings.Contains(errb, "["+codeBuildHandoffFloor+"] "+floorLine) {
		t.Fatalf("a floor failure is a verify violation in the floor's own words: exit=%d stderr=%s", code, errb)
	}
	if want := core.ReviewInputFor(state, core.PhaseBuild, root); len(floor.inputs) != 1 || !reflect.DeepEqual(floor.inputs[0], want) {
		t.Fatalf("verify must run the floor once on the input the cycle's floor reviews:\ngot  %+v\nwant %+v", floor.inputs, want)
	}
	if !reflect.DeepEqual(floor.roots, []string{root}) {
		t.Fatalf("the floor is resolved for the project root: %q", floor.roots)
	}
	code, out, _ := floorVerify(t, floor.floorFor, "build", "--workspace", ws, "--json")
	if code != 1 || !strings.Contains(out, `"code": "`+codeBuildHandoffFloor+`"`) {
		t.Fatalf("--json carries the floor violation: exit=%d\n%s", code, out)
	}
}

func TestPhaseVerifyBuild_WithoutACycleBindingRunsTheFloorUnboundAndSaysSo(t *testing.T) {
	root, ws := floorWorkspace(t)
	wt := t.TempDir()
	floor := &recordedFloor{}
	code, _, errb := floorVerify(t, floor.floorFor, "build", "--workspace", ws, "--worktree", wt)
	want := core.ReviewInput{Phase: string(core.PhaseBuild), Workspace: ws, Worktree: wt, ProjectRoot: root}
	if len(floor.inputs) != 1 || !reflect.DeepEqual(floor.inputs[0], want) {
		t.Fatalf("with no binding the floor runs on the tree it was given: got %+v", floor.inputs)
	}
	if !strings.Contains(errb, "no cycle binding") || !strings.Contains(errb, core.RunStateFile+" unreadable") {
		t.Fatalf("a probe that cannot bind the cycle must say so, naming the missing state: stderr=%s", errb)
	}
	if code != 0 {
		t.Fatalf("a passing floor and a well-formed report exit 0: exit=%d stderr=%s", code, errb)
	}
}

func TestPhaseVerify_OnlyBuildRunsTheFloor(t *testing.T) {
	_, ws := floorWorkspace(t)
	floor := &recordedFloor{failures: []string{"never"}}
	for _, phase := range []string{"tdd", "audit", "scout"} {
		floorVerify(t, floor.floorFor, phase, "--workspace", ws)
	}
	if len(floor.roots)+len(floor.inputs) != 0 {
		t.Fatalf("the build handoff floor judges build only; ran for %q", floor.roots)
	}
}

func TestPhaseVerifyBuild_WithoutAWiredFloorSaysTheFloorDidNotRun(t *testing.T) {
	_, ws := floorWorkspace(t)
	code, _, errb := floorVerify(t, nil, "build", "--workspace", ws)
	if code != 0 || !strings.Contains(errb, "no build handoff floor is wired") {
		t.Fatalf("an unwired command must say the floor did not run: exit=%d stderr=%s", code, errb)
	}
}

func TestPhaseVerifyBuild_OneCycleBindingFeedsTheContractAndTheFloor(t *testing.T) {
	root, ws := floorWorkspace(t)
	boundWorktree := t.TempDir()
	if err := os.WriteFile(filepath.Join(boundWorktree, "build-report.md"), []byte("stray\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	state := core.CycleState{CycleID: 1788, Phase: "build", RunID: "run-1788", WorkspacePath: ws, ActiveWorktree: boundWorktree}
	body, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, core.RunStateFile), body, 0o644); err != nil {
		t.Fatal(err)
	}
	floor := &recordedFloor{}
	code, _, errb := floorVerify(t, floor.floorFor, "build", "--workspace", ws)
	if len(floor.inputs) != 1 || floor.inputs[0].Worktree != boundWorktree {
		t.Fatalf("the floor half reviews the bound worktree: %+v", floor.inputs)
	}
	if code != 1 || !strings.Contains(errb, "stray_in_worktree") || !strings.Contains(errb, boundWorktree) {
		t.Fatalf("the contract half must judge the same bound worktree the floor judged (the host gate's stray check), not a second source: exit=%d stderr=%s", code, errb)
	}
	if want := core.ReviewInputFor(state, core.PhaseBuild, root); !reflect.DeepEqual(floor.inputs[0], want) {
		t.Fatalf("floor input = %+v, want %+v", floor.inputs[0], want)
	}
}
