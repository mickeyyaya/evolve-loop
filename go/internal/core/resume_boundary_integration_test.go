//go:build integration

package core_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/checkpoint"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func TestResumeBoundaryCheckpointer_FailureStopsDispatchAndPreservesPause(t *testing.T) {
	// External package permits the real storage/checkpoint adapters; their
	// shared state file must survive the consumer's normal failure closeout.
	w := fixtures.NewWorkspace(t).WithState(core.State{LastCycleNumber: 6, LastAllocatedCycleNumber: 7}).Build()
	initMaterializationRepo(t, w.Root)
	pendingPath := w.Write("feature.go", "package feature\n")
	ws := w.CycleDir(7)
	statePath := filepath.Join(w.EvolveDir, core.CycleStateFile)
	t.Setenv(ipcenv.CycleStateFileKey, statePath)
	cs := core.CycleState{
		CycleID: 7, RunID: "original-run", Phase: "build", WorkspacePath: ws, ActiveWorktree: w.Root,
		GoalHash: "original-goal", GoalText: "finish the original work", FinalVerdict: core.VerdictPASS,
		CompletedPhases: []string{"scout", "triage", "tdd"},
	}
	store := storage.New(w.EvolveDir)
	ctx := context.Background()
	if err := store.WriteCycleState(ctx, cs); err != nil {
		t.Fatal(err)
	}
	if err := checkpoint.ApplyToStateFile(statePath, checkpoint.Compose(cs, checkpoint.ReasonQuotaLikely, 4.25, "", time.Unix(1000, 0))); err != nil {
		t.Fatal(err)
	}
	readPause := func() []byte {
		t.Helper()
		raw, err := os.ReadFile(statePath)
		if err != nil {
			t.Fatal(err)
		}
		var state map[string]json.RawMessage
		if err := json.Unmarshal(raw, &state); err != nil {
			t.Fatal(err)
		}
		// Storage may reindent the enclosing document. Preserve every checkpoint
		// value while ignoring that insignificant serialization whitespace.
		var compact bytes.Buffer
		if err := json.Compact(&compact, state["checkpoint"]); err != nil {
			t.Fatal(err)
		}
		return compact.Bytes()
	}
	originalPause := readPause()
	rp, err := core.LoadResumeState(ctx, w.Root, w.EvolveDir, core.ResumeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if rp.StatePath != statePath {
		t.Fatalf("resume selected %q, want the persisted pause %q", rp.StatePath, statePath)
	}

	previous := core.ResumeBoundaryCheckpointer
	t.Cleanup(func() { core.ResumeBoundaryCheckpointer = previous })
	writeErr := errors.New("resume checkpoint write denied")
	checkpointCalls := 0
	core.ResumeBoundaryCheckpointer = func(next core.CycleState, projectRoot string, now time.Time) error {
		checkpointCalls++
		if next.Phase != "build" || next.CycleID != cs.CycleID || next.RunID != cs.RunID || projectRoot != w.Root || now.IsZero() {
			t.Errorf("checkpoint received the wrong resume boundary: state=%+v root=%q now=%v", next, projectRoot, now)
		}
		return writeErr
	}
	runners := fixtures.BuildRunners(nil)
	o := core.NewOrchestrator(store, &fixtures.FakeLedger{}, runners)
	result, err := o.RunCycleFromPhase(ctx, core.CycleRequest{ProjectRoot: w.Root, GoalHash: cs.GoalHash}, rp)
	if !errors.Is(err, writeErr) || checkpointCalls != 1 {
		t.Fatalf("resume did not propagate the checkpoint refusal: calls=%d err=%v", checkpointCalls, err)
	}
	for phase, runner := range runners {
		if calls := runner.(*fixtures.FakeRunner).Calls; calls != 0 {
			t.Errorf("%s dispatched %d times after checkpoint persistence failed", phase, calls)
		}
	}
	if got := readPause(); !bytes.Equal(got, originalPause) {
		t.Errorf("failed resume replaced the original pause:\nbefore=%s\nafter=%s", originalPause, got)
	}
	saved, err := store.ReadCycleState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if saved.CycleID != cs.CycleID || saved.RunID != cs.RunID || saved.GoalHash != cs.GoalHash || saved.ActiveWorktree != cs.ActiveWorktree || !slices.Equal(saved.CompletedPhases, cs.CompletedPhases) {
		t.Errorf("checkpoint refusal lost original identity or advanced completion: %+v", saved)
	}
	if raw, err := os.ReadFile(pendingPath); err != nil || string(raw) != "package feature\n" {
		t.Errorf("checkpoint refusal discarded preserved work: content=%q err=%v", raw, err)
	}
	// Failure still receives ordinary terminal diagnostics; it must not invent
	// a successful phase completion or suppress the checkpoint write error.
	raw, err := os.ReadFile(filepath.Join(dossier.CyclesDir(w.Root), "cycle-7.json"))
	if err != nil {
		t.Fatal(err)
	}
	d, err := dossier.ParseJSON(raw)
	if err != nil || d.Cycle != cs.CycleID || d.RunID != cs.RunID || d.FinalVerdict != core.VerdictFAIL || result.FinalVerdict != core.VerdictFAIL || saved.Phase != "aborted" {
		t.Fatalf("checkpoint refusal lost normal failure diagnostics: dossier=%+v result=%+v state=%s err=%v", d, result, saved.Phase, err)
	}
}
