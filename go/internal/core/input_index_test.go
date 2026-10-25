package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/inputindex"
)

func writeIndexFixture(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDispatch_WritesTheInputIndexOfThePhaseBeforeTheLaunch(t *testing.T) {
	t.Parallel()
	var seen []string
	runner := &dispatchScriptRunnerE4{name: "build", steps: []func() (PhaseResponse, error){
		func() (PhaseResponse, error) { return PhaseResponse{Phase: "build", Verdict: VerdictPASS}, nil },
	}}
	cr := dispatchRunE4(t, &fakeLedger{}, runner, PhaseBuild, 1)
	cr.cs.WorkspacePath = filepath.Join(cr.req.ProjectRoot, ".evolve", "runs", "cycle-5")
	writeIndexFixture(t, filepath.Join(cr.cs.WorkspacePath, "scout-report.md"), "# Scout of cycle 5\n")
	runner.steps[0] = func() (PhaseResponse, error) {
		idx, err := inputindex.Read(cr.cs.WorkspacePath)
		if err != nil {
			t.Errorf("the index must exist when the phase starts: %v", err)
		}
		for _, e := range idx.Entries {
			seen = append(seen, e.Kind)
		}
		return PhaseResponse{Phase: "build", Verdict: VerdictPASS}, nil
	}
	if _, _, err := cr.dispatch(PhaseBuild); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(seen, []string{"scout-report"}) {
		t.Fatalf("the build saw the entries %v, want [scout-report]", seen)
	}
}

func TestWriteInputIndex_UsesThePolicyLineageMax(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	runs := filepath.Join(root, ".evolve", "runs")
	for _, n := range []string{"1", "2", "3"} {
		writeIndexFixture(t, filepath.Join(runs, "cycle-"+n, "lane-scope.json"), `{"todo_ids":["item-a"]}`)
	}
	writeIndexFixture(t, filepath.Join(root, ".evolve", "policy.json"), `{"isolation":{"lineage_max":1}}`)
	var warn strings.Builder
	writeInputIndex(root, filepath.Join(runs, "cycle-3"), 3, PhaseBuild, &warn)
	idx, err := inputindex.Read(filepath.Join(runs, "cycle-3"))
	if err != nil || !slices.Equal(idx.Lineage, []int{2}) || idx.Attempt != 3 || warn.Len() != 0 {
		t.Fatalf("index = %+v err = %v warn = %q, want lineage [2] at attempt 3", idx, err, warn.String())
	}
}

func TestWriteInputIndex_WarnsLoudlyWhenTheIndexCannotBeBuiltOrWritten(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	var warn strings.Builder
	writeInputIndex("relative", filepath.Join(root, "ws"), 3, PhaseBuild, &warn)
	if !strings.Contains(warn.String(), "WARN input index for build") {
		t.Fatalf("a failed build must warn, got %q", warn.String())
	}
	warn.Reset()
	stale := filepath.Join(root, "ws3")
	writeIndexFixture(t, inputindex.Path(stale), `{"schema":"evolve.inputs.index/1","entries":[{"path":"x"}]}`)
	if err := os.MkdirAll(filepath.Join(stale, "scout-report.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeInputIndex(root, stale, 3, PhaseBuild, &warn)
	if _, err := os.Stat(inputindex.Path(stale)); !os.IsNotExist(err) || !strings.Contains(warn.String(), "WARN input index for build") {
		t.Fatalf("a failed build must remove the stale index of the earlier phase, so no stale grant survives: stat err = %v, warn = %q", err, warn.String())
	}
	warn.Reset()
	writeIndexFixture(t, filepath.Join(root, "ws2", "inputs"), "a file where the inputs directory should be")
	writeInputIndex(root, filepath.Join(root, "ws2"), 3, PhaseBuild, &warn)
	if !strings.Contains(warn.String(), "WARN input index for build") {
		t.Fatalf("a failed write must warn, got %q", warn.String())
	}
}

func TestDispatch_TheIndexIsTheOnlyLineageInputAndNoEntryIsPreloadedIntoTheRequest(t *testing.T) {
	t.Parallel()
	runner := &capturingRunner{}
	cr := dispatchRunE4(t, &fakeLedger{}, runner, PhaseBuild, 1)
	runs := filepath.Join(cr.req.ProjectRoot, ".evolve", "runs")
	cr.cs.WorkspacePath = filepath.Join(runs, "cycle-5")
	writeIndexFixture(t, filepath.Join(runs, "cycle-4", "lane-scope.json"), `{"todo_ids":["item-a"]}`)
	writeIndexFixture(t, filepath.Join(runs, "cycle-4", "retrospective-report.md"), "# lesson-text-of-cycle-4\n")
	writeIndexFixture(t, filepath.Join(cr.cs.WorkspacePath, "lane-scope.json"), `{"todo_ids":["item-a"]}`)
	if _, _, err := cr.dispatch(PhaseBuild); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(runner.req)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "lesson-text-of-cycle-4") {
		t.Fatalf("the lineage lesson must stay on demand behind the index, but the phase request carries it: %s", raw)
	}
	idx, err := inputindex.Read(cr.cs.WorkspacePath)
	if err != nil || !slices.Equal(idx.Lineage, []int{4}) {
		t.Fatalf("the index must list the lineage instead: %+v %v", idx, err)
	}
}

type capturingRunner struct{ req PhaseRequest }

func (r *capturingRunner) Name() string { return "build" }
func (r *capturingRunner) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	r.req = req
	return PhaseResponse{Phase: "build", Verdict: VerdictPASS}, nil
}
