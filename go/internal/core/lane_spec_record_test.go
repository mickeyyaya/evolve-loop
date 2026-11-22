package core

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
)

func TestCycleRecord_ANewRunWithNoSpecKeepsTheSpecOfTheRecord(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	spec := committedset.LaneSpec{DependsOn: []string{"d"}, Priority: 2, OutputContract: "docs"}
	if _, err := cycleRecord(indexTarget{projectRoot: root, runID: "run-1", goalHash: "g", cycle: 6, spec: spec}, []string{"a"}); err != nil {
		t.Fatal(err)
	}

	second, err := cycleRecord(indexTarget{projectRoot: root, runID: "run-2", goalHash: "g", cycle: 6}, []string{"a"})

	if err != nil || !reflect.DeepEqual(second.LaneSpec, spec) {
		t.Fatalf("second record = %+v, %v, want the spec %+v of the first run", second, err, spec)
	}
}

func TestPausedLaneScopes_CarriesTheLaneSpecOfTheLineageRecord(t *testing.T) {
	t.Parallel()
	evolveDir, worktree, root := t.TempDir(), t.TempDir(), t.TempDir()
	path := seedLaneRun(t, evolveDir, quotaPausedRun(7, worktree, "item-a"))
	spec := committedset.LaneSpec{DependsOn: []string{"dep"}, Priority: 3, OutputContract: "pr"}
	if err := committedset.WriteRecord(root, committedset.Record{Cycle: 7, ItemIDs: []string{"item-a"}, LaneSpec: spec}); err != nil {
		t.Fatal(err)
	}

	got, err := PausedLaneScopes(root, evolveDir, ResumeOptions{})

	want := []PausedScope{{Cycle: 7, Scope: []string{"item-a"}, Phase: "build", StatePath: path, AutoMaxAttempts: 3, Reason: laneQuotaReason, Spec: spec}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("PausedLaneScopes = %+v, %v, want %+v", got, err, want)
	}
}
