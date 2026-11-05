package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
)

func blockLineage(t *testing.T, root string) string {
	t.Helper()
	evolve := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolve, 0o755); err != nil {
		t.Fatal(err)
	}
	blocker := filepath.Join(evolve, "lineage")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return blocker
}

func TestDispatch_AFleetCycleWhoseRecordCannotBeWrittenStartsNoCell(t *testing.T) {
	t.Parallel()
	ran := 0
	var cr *cycleRun
	step := func() (PhaseResponse, error) {
		ran++
		writeLaneScopeFixture(t, cr.cs.WorkspacePath, []string{"b", "c"}, "forged")
		return PhaseResponse{Phase: "scout", Verdict: VerdictPASS}, nil
	}
	runner := &dispatchScriptRunnerE4{name: "scout", steps: []func() (PhaseResponse, error){step}}
	cr = dispatchRunE4(t, &fakeLedger{}, runner, PhaseScout, 1)
	root := t.TempDir()
	cr.req.ProjectRoot = root
	cr.req.GoalHash = "g"
	cr.cs.WorkspacePath = filepath.Join(root, ".evolve", "runs", "cycle-5")
	cr.ctxSnap["fleet_scope"] = "a"
	writeLaneScopeFixture(t, cr.cs.WorkspacePath, []string{"a"}, "g")
	blocker := blockLineage(t, root)

	_, act, err := cr.dispatch(PhaseScout)
	if !errors.Is(err, ErrLaneRecordUnwritable) || act != loopAbort {
		t.Fatalf("dispatch = %v, %v, want loopAbort with ErrLaneRecordUnwritable", act, err)
	}
	if ran != 0 {
		t.Fatalf("the runner ran %d times, want no cell", ran)
	}

	if err := os.Remove(blocker); err != nil {
		t.Fatal(err)
	}
	writeLaneScopeFixture(t, cr.cs.WorkspacePath, []string{"b", "c"}, "forged")
	rec, err := cycleRecord(indexTarget{projectRoot: root, workspace: cr.cs.WorkspacePath, runID: "run-resume", goalHash: "g", cycle: 5}, []string{"a"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(rec.ItemIDs, ",") != "a" || rec.GoalHash != "g" {
		t.Fatalf("the resumed record = %+v, want ids [a] and goal g", rec)
	}
	if got := committedset.LanePin(cr.cs.WorkspacePath); strings.Join(got, ",") != "a" {
		t.Fatalf("LanePin = %v, want [a]", got)
	}
}

func TestWriteInputIndex_ARecordFailureWithoutAFleetScopeStaysAWarning(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	blockLineage(t, root)
	var warn strings.Builder
	err := writeInputIndex(indexTarget{projectRoot: root, workspace: filepath.Join(root, "ws"), cycle: 3}, PhaseBuild, nil, &warn)
	if err != nil {
		t.Fatalf("writeInputIndex = %v, want nil without a fleet scope", err)
	}
	if !strings.Contains(warn.String(), "WARN input index") {
		t.Fatalf("warn = %q", warn.String())
	}
}

func TestWriteInputIndex_ARecordFailureWithAFleetScopeIsReturned(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	blockLineage(t, root)
	var warn strings.Builder
	err := writeInputIndex(indexTarget{projectRoot: root, workspace: filepath.Join(root, "ws"), cycle: 3}, PhaseBuild, []string{"a"}, &warn)
	if !errors.Is(err, ErrLaneRecordUnwritable) {
		t.Fatalf("writeInputIndex = %v, want ErrLaneRecordUnwritable", err)
	}
}

func TestWriteInputIndex_AFailedIndexWithAFleetScopeStaysAWarning(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ws := filepath.Join(root, "ws")
	if err := os.WriteFile(ws, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var warn strings.Builder
	err := writeInputIndex(indexTarget{projectRoot: root, workspace: ws, cycle: 3}, PhaseBuild, []string{"a"}, &warn)
	if err != nil {
		t.Fatalf("writeInputIndex = %v, want nil: the record was written", err)
	}
	if !strings.Contains(warn.String(), "WARN input index") {
		t.Fatalf("warn = %q, want the index warning", warn.String())
	}
}
