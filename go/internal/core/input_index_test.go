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
	for _, n := range []int{1, 2} {
		if err := inputindex.WriteRecord(root, inputindex.Record{Cycle: n, ItemIDs: []string{"item-a"}}); err != nil {
			t.Fatal(err)
		}
	}
	writeIndexFixture(t, filepath.Join(root, ".evolve", "policy.json"), `{"isolation":{"lineage_max":1}}`)
	var warn strings.Builder
	writeInputIndex(indexTarget{projectRoot: root, workspace: filepath.Join(runs, "cycle-3"), cycle: 3}, PhaseBuild, []string{"item-a"}, &warn)
	idx, err := inputindex.Read(filepath.Join(runs, "cycle-3"))
	if err != nil || !slices.Equal(idx.Lineage, []int{2}) || idx.Attempt != 3 || warn.Len() != 0 {
		t.Fatalf("index = %+v err = %v warn = %q, want lineage [2] at attempt 3", idx, err, warn.String())
	}
}

func TestWriteInputIndex_WarnsLoudlyWhenTheIndexCannotBeBuiltOrWritten(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	var warn strings.Builder
	writeInputIndex(indexTarget{projectRoot: "relative", workspace: filepath.Join(root, "ws"), cycle: 3}, PhaseBuild, nil, &warn)
	if !strings.Contains(warn.String(), "WARN input index for build") {
		t.Fatalf("a failed build must warn, got %q", warn.String())
	}
	warn.Reset()
	stale := filepath.Join(root, "ws3")
	writeIndexFixture(t, inputindex.Path(stale), `{"schema":"evolve.inputs.index/1","entries":[{"path":"x"}]}`)
	if err := os.MkdirAll(filepath.Join(stale, "scout-report.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeInputIndex(indexTarget{projectRoot: root, workspace: stale, cycle: 3}, PhaseBuild, nil, &warn)
	if _, err := os.Stat(inputindex.Path(stale)); !os.IsNotExist(err) || !strings.Contains(warn.String(), "WARN input index for build") {
		t.Fatalf("a failed build must remove the stale index of the earlier phase, so no stale grant survives: stat err = %v, warn = %q", err, warn.String())
	}
	warn.Reset()
	writeIndexFixture(t, filepath.Join(root, "ws2", "inputs"), "a file where the inputs directory should be")
	writeInputIndex(indexTarget{projectRoot: root, workspace: filepath.Join(root, "ws2"), cycle: 3}, PhaseBuild, nil, &warn)
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
	if err := inputindex.WriteRecord(cr.req.ProjectRoot, inputindex.Record{Cycle: 4, ItemIDs: []string{"item-a"}}); err != nil {
		t.Fatal(err)
	}
	writeIndexFixture(t, filepath.Join(runs, "cycle-4", "retrospective-report.md"), "# lesson-text-of-cycle-4\n")
	cr.ctxSnap["fleet_scope"] = "item-a"
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

func TestDispatch_ALaneScopeThatACellRewroteNeitherNamesTheItemNorJoinsTheLineage(t *testing.T) {
	t.Parallel()
	cr := dispatchRunE4(t, &fakeLedger{}, &capturingRunner{}, PhaseBuild, 1)
	root := cr.req.ProjectRoot
	runs := filepath.Join(root, ".evolve", "runs")
	cr.cs.WorkspacePath = filepath.Join(runs, "cycle-5")
	cr.ctxSnap["fleet_scope"] = " item-a , "
	for cycle, item := range map[int]string{3: "item-z", 4: "item-a"} {
		if err := inputindex.WriteRecord(root, inputindex.Record{Cycle: cycle, ItemIDs: []string{item}}); err != nil {
			t.Fatal(err)
		}
	}
	writeIndexFixture(t, filepath.Join(cr.cs.WorkspacePath, "lane-scope.json"), `{"todo_ids":["item-z"]}`)
	writeIndexFixture(t, filepath.Join(runs, "cycle-6", "lane-scope.json"), `{"todo_ids":["item-a"]}`)
	if _, _, err := cr.dispatch(PhaseBuild); err != nil {
		t.Fatal(err)
	}
	idx, err := inputindex.Read(cr.cs.WorkspacePath)
	if err != nil || !slices.Equal(idx.ItemIDs, []string{"item-a"}) || !slices.Equal(idx.Lineage, []int{4}) {
		t.Fatalf("index = %+v err = %v; the item id comes from the fleet scope of the orchestrator and the lineage from the pipeline records only", idx, err)
	}
	raw, err := os.ReadFile(filepath.Join(root, ".evolve", "lineage", "cycle-5.json"))
	if err != nil || !strings.Contains(string(raw), `"item-a"`) || strings.Contains(string(raw), "item-z") {
		t.Fatalf("the dispatch must record the item of the cycle where no cell can write: %q %v", raw, err)
	}
}

func TestWriteInputIndex_TheFirstRecordOfTheRunWinsOverALaterScopeAndANewRunReplacesIt(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ws := filepath.Join(root, ".evolve", "runs", "cycle-5")
	run1 := indexTarget{projectRoot: root, workspace: ws, cycle: 5, runID: "run-1"}
	var warn strings.Builder
	if err := os.MkdirAll(filepath.Join(ws, "scout-report.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeInputIndex(run1, PhaseScout, []string{"item-a"}, &warn)
	if err := os.Remove(filepath.Join(ws, "scout-report.md")); err != nil {
		t.Fatal(err)
	}
	writeInputIndex(run1, PhaseBuild, []string{"item-z"}, &warn)
	idx, err := inputindex.Read(ws)
	if err != nil || !slices.Equal(idx.ItemIDs, []string{"item-a"}) {
		t.Fatalf("the record of the first dispatch must win even when its index failed, so a forged lane scope on resume changes nothing: %+v %v", idx, err)
	}
	run2 := run1
	run2.runID = "run-2"
	writeInputIndex(run2, PhaseScout, []string{"item-b"}, &warn)
	if idx, err := inputindex.Read(ws); err != nil || !slices.Equal(idx.ItemIDs, []string{"item-b"}) {
		t.Fatalf("a new run of the cycle number must not inherit a stale record: %+v %v", idx, err)
	}
}

func TestWriteInputIndex_WarnsWhenTheRecordCannotBeWritten(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeIndexFixture(t, filepath.Join(root, ".evolve", "lineage"), "a file where the record directory should be")
	var warn strings.Builder
	writeInputIndex(indexTarget{projectRoot: root, workspace: filepath.Join(root, "ws"), cycle: 3}, PhaseBuild, nil, &warn)
	if !strings.Contains(warn.String(), "WARN input index for build") {
		t.Fatalf("warn = %q", warn.String())
	}
}

func TestWriteInputIndex_CarriesThePolicyContextAndTheSpecInputs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ws := filepath.Join(root, ".evolve", "runs", "cycle-3")
	writeIndexFixture(t, filepath.Join(ws, "scout-report.md"), "# "+strings.Repeat("x", 40)+"\n")
	writeIndexFixture(t, filepath.Join(ws, "test-report.md"), "# t\n")
	writeIndexFixture(t, filepath.Join(root, ".evolve", "policy.json"), `{"isolation":{"context":{"budget":{"l1_tokens":5},"required":{"audit":["test-report"]}}}}`)
	var warn strings.Builder
	at := indexTarget{projectRoot: root, workspace: ws, cycle: 3, specInputs: []string{".evolve/runs/cycle-{cycle}/scout-report.md"}}
	writeInputIndex(at, PhaseBuild, nil, &warn)
	idx, err := inputindex.Read(ws)
	if err != nil || warn.Len() != 0 {
		t.Fatalf("index err = %v warn = %q", err, warn.String())
	}
	if b := idx.Budgets[inputindex.LayerUpstream]; b.LimitTokens != 5 || len(idx.Required) != 0 || len(idx.Notes) == 0 {
		t.Fatalf("the spec input scout-report is over the policy budget of 5 tokens: budgets %+v required %v notes %v", idx.Budgets, idx.Required, idx.Notes)
	}
	writeInputIndex(at, PhaseAudit, nil, &warn)
	idx, err = inputindex.Read(ws)
	if err != nil || len(idx.Required) != 1 || idx.Entries[slices.IndexFunc(idx.Entries, func(e inputindex.Entry) bool { return e.Required })].Kind != "test-report" {
		t.Fatalf("the policy list of the audit wins over the spec inputs: %+v err %v", idx.Entries, err)
	}
}
