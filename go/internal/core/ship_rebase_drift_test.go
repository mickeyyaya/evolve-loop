package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func laneRebaser(l *inboxLane) *fleetRebase {
	return &fleetRebase{worktree: l.worktree, git: gitCapture, inbox: newInboxStamps([]string{ownItemID})}
}

func TestFleetRebase_DriftNamesTheCodeThatTheRebaseLost(t *testing.T) {
	t.Parallel()

	l := newInboxLane(t,
		func(l *inboxLane) { writeInboxFile(t, filepath.Join(l.worktree, ownItemPath), laneItem) },
		stampOnMainFile(t))
	f := laneRebaser(l)
	head, err := f.anchor(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := f.rebaseAll(context.Background()); !ok {
		t.Fatal("the fixture rebase must land")
	}
	if drift, err := f.auditedDrift(context.Background(), head); err != nil || len(drift) != 0 {
		t.Fatalf("drift = %v, %v, want none for a correct rebase", drift, err)
	}
	if err := os.Remove(filepath.Join(l.worktree, "go/lane.go")); err != nil {
		t.Fatal(err)
	}
	l.repo.Git("-C", l.worktree, "add", "-A")
	l.repo.Git("-C", l.worktree, "commit", "-q", "-m", "lose the lane code")

	drift, err := f.auditedDrift(context.Background(), head)

	if err != nil || !reflect.DeepEqual(drift, []string{"go/lane.go"}) {
		t.Fatalf("drift = %v, %v, want [go/lane.go]", drift, err)
	}
}

func stampOnMainFile(t *testing.T) func(*inboxLane) {
	return func(l *inboxLane) { writeInboxFile(t, filepath.Join(l.repo.Dir, ownItemPath), stampItem) }
}

func TestFleetRebase_AMergeTreeThatCannotRunIsAnErrorNotAProof(t *testing.T) {
	t.Parallel()

	g := &scriptedGit{respond: func([]string) (string, int, error) { return "", 0, errors.New("spawn failed") }}
	f := &fleetRebase{worktree: t.TempDir(), git: g.capture}

	drift, err := f.auditedDrift(context.Background(), "orig")

	if err == nil || len(drift) != 0 {
		t.Fatalf("drift = %v, %v, want an error and no verdict", drift, err)
	}
}
