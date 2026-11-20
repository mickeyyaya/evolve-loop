package core

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/recoveryretro"
)

const (
	retireJudgmentID  = "cycle-5-judgment-plan-review"
	retireOtherID     = "cycle-6-judgment-audit"
	retireNonJudgment = "cycle-5-failed-build"
)

func retireTodos() []CarryoverTodo {
	return []CarryoverTodo{
		{ID: retireJudgmentID, Action: "the plan widens the lane scope"},
		{ID: retireOtherID, Action: "the audit trusts a stale report"},
		{ID: retireNonJudgment, Action: "cycle 5 failed during build: a compile error"},
	}
}

func publishedDocument(t *testing.T, root, id string) {
	t.Helper()
	dir := filepath.Join(root, filepath.FromSlash(recoveryretro.DocsHome))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".md"), []byte("# "+id+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func closeoutOn(t *testing.T, root string, storage *fakeStorage) {
	t.Helper()
	o := NewOrchestrator(storage, &fakeLedger{}, buildRunners(nil),
		WithWorktreeProvisioner(&fakeWorktree{path: t.TempDir()}), WithDossierDestination(DossierFilesOnly), WithStderr(&bytes.Buffer{}))
	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "retro-retire"}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
}

func todoIDs(todos []CarryoverTodo) []string {
	ids := make([]string, 0, len(todos))
	for _, td := range todos {
		ids = append(ids, td.ID)
	}
	return ids
}

func TestRunCycle_AtEnforceAJudgmentTodoWithNoDocumentOnMainStays(t *testing.T) {
	t.Parallel()
	root := enforceRoot(t, "enforce")
	storage := &fakeStorage{state: State{CarryoverTodos: retireTodos()}}

	closeoutOn(t, root, storage)

	if got := todoIDs(storage.state.CarryoverTodos); !reflect.DeepEqual(got, []string{retireJudgmentID, retireOtherID, retireNonJudgment}) {
		t.Fatalf("no document is on main, so every todo stays: %v", got)
	}
	if _, err := os.Stat(filepath.Join(recoveryretro.PendingDir(root), retireJudgmentID+".md")); err != nil {
		t.Errorf("a pending document is not a document on main, and the closeout must still write it: %v", err)
	}
}

func TestRunCycle_AtEnforceAJudgmentTodoWithItsDocumentOnMainLeavesAndTheRestStay(t *testing.T) {
	t.Parallel()
	root := enforceRoot(t, "enforce")
	publishedDocument(t, root, retireJudgmentID)
	publishedDocument(t, root, retireNonJudgment)
	storage := &fakeStorage{state: State{CarryoverTodos: retireTodos()}}

	closeoutOn(t, root, storage)

	if got := todoIDs(storage.state.CarryoverTodos); !reflect.DeepEqual(got, []string{retireOtherID, retireNonJudgment}) {
		t.Fatalf("the documented judgment todo leaves; the undocumented one and the non-judgment todo stay: %v", got)
	}
}

func TestRunCycle_ASecondEnforceCloseoutChangesNothingInTheCarryoverLog(t *testing.T) {
	t.Parallel()
	root := enforceRoot(t, "enforce")
	publishedDocument(t, root, retireJudgmentID)
	storage := &fakeStorage{state: State{CarryoverTodos: retireTodos()}}

	closeoutOn(t, root, storage)
	afterFirst := append([]CarryoverTodo(nil), storage.state.CarryoverTodos...)
	closeoutOn(t, root, storage)

	if !reflect.DeepEqual(storage.state.CarryoverTodos, afterFirst) {
		t.Fatalf("the second closeout changed the log: before %+v, after %+v", afterFirst, storage.state.CarryoverTodos)
	}
	if len(afterFirst) != 2 {
		t.Fatalf("the first closeout must retire exactly the documented todo: %v", todoIDs(afterFirst))
	}
}

func TestRunCycle_AtEnforceTheJudgmentTodoLeavesOnlyAfterTheBoundaryPublishedItsDocument(t *testing.T) {
	t.Parallel()
	repo := gittest.Fixture(t)
	repo.Write("README.md", "plane\n")
	repo.Git("add", "README.md")
	repo.Git("commit", "-q", "-m", "base")
	repo.Write(".evolve/policy.json", `{"retrospectives":{"stage":"enforce"}}`)
	storage := &fakeStorage{state: State{CarryoverTodos: []CarryoverTodo{{ID: retireJudgmentID, Action: "the plan widens the lane scope"}}}}

	closeoutOn(t, repo.Dir, storage)
	if len(storage.state.CarryoverTodos) != 1 {
		t.Fatalf("before the publish the todo stays, or a failed publish drops the lesson: %v", todoIDs(storage.state.CarryoverTodos))
	}
	published, err := recoveryretro.PublishPending(repo.Dir, &bytes.Buffer{})
	if err != nil || len(published) != 1 {
		t.Fatalf("PublishPending = %v, %v; want the one pair", published, err)
	}
	closeoutOn(t, repo.Dir, storage)

	if len(storage.state.CarryoverTodos) != 0 {
		t.Fatalf("after the boundary published its document, the next closeout retires the todo: %v", todoIDs(storage.state.CarryoverTodos))
	}
	if _, err := os.Stat(filepath.Join(repo.Dir, filepath.FromSlash(recoveryretro.DocsHome), retireJudgmentID+".md")); err != nil {
		t.Errorf("the document stays on main after the todo left: %v", err)
	}
}

func TestRunCycle_BelowEnforceAJudgmentTodoWithItsDocumentOnMainStays(t *testing.T) {
	t.Parallel()
	for _, stage := range []string{"shadow", "off"} {
		t.Run(stage, func(t *testing.T) {
			t.Parallel()
			root := enforceRoot(t, stage)
			publishedDocument(t, root, retireJudgmentID)
			storage := &fakeStorage{state: State{CarryoverTodos: retireTodos()}}

			closeoutOn(t, root, storage)

			if got := todoIDs(storage.state.CarryoverTodos); !reflect.DeepEqual(got, []string{retireJudgmentID, retireOtherID, retireNonJudgment}) {
				t.Fatalf("at stage %s the todo retirement is not active: %v", stage, got)
			}
		})
	}
}
