package core

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestRebaseRecordingConflicts_APeerThatLandedTheSameFileIsACorrectRebase(t *testing.T) {
	t.Parallel()

	l := newInboxLane(t,
		func(l *inboxLane) {
			writeInboxFile(t, filepath.Join(l.worktree, ownItemPath), laneItem)
			writeInboxFile(t, filepath.Join(l.worktree, "go/same.go"), "package same\n")
		},
		func(l *inboxLane) {
			writeInboxFile(t, filepath.Join(l.repo.Dir, ownItemPath), stampItem)
			writeInboxFile(t, filepath.Join(l.repo.Dir, "go/same.go"), "package same\n")
		})

	ok, conflict, stamped, _ := l.rebase(t)

	if !ok || conflict || !reflect.DeepEqual(stamped, []string{ownItemPath}) {
		t.Fatalf("rebase = (%v, %v, %v), want the lane to land: a peer absorbed one of its files", ok, conflict, stamped)
	}
}

func TestRebaseRecordingConflicts_APeerEditNearTheLanesLinesIsACorrectRebase(t *testing.T) {
	t.Parallel()

	l := newInboxLane(t,
		func(l *inboxLane) {
			writeInboxFile(t, filepath.Join(l.worktree, ownItemPath), laneItem)
			writeInboxFile(t, filepath.Join(l.worktree, "go/a.go"), "package a\n\nvar lane = 1\n")
		},
		func(l *inboxLane) {
			writeInboxFile(t, filepath.Join(l.repo.Dir, ownItemPath), stampItem)
			writeInboxFile(t, filepath.Join(l.repo.Dir, "go/a.go"), "// peer\npackage a\n")
		})

	ok, conflict, stamped, _ := l.rebase(t)

	if !ok || conflict || len(stamped) != 1 {
		t.Fatalf("rebase = (%v, %v, %v), want the lane to land beside a peer edit of the same file", ok, conflict, stamped)
	}
}

func TestRebaseRecordingConflicts_AnItemAddedByTheLaneAndByMainMergesWithNoBaseStage(t *testing.T) {
	t.Parallel()

	const addedID = "both-sides-added"
	const addedPath = ".evolve/inbox/2026-11-04T01-00-00Z-" + addedID + ".json"
	l := newInboxLane(t,
		func(l *inboxLane) {
			writeInboxFile(t, filepath.Join(l.worktree, addedPath), `{"id":"`+addedID+`","notes":"lane"}`)
		},
		func(l *inboxLane) {
			writeInboxFile(t, filepath.Join(l.repo.Dir, addedPath), `{"id":"`+addedID+`","failure_count":1}`)
		})
	materializeLaneScope(l.workspace, addedID, "")

	ok, conflict, stamped, _ := l.rebase(t)

	if !ok || conflict || !reflect.DeepEqual(stamped, []string{addedPath}) {
		t.Fatalf("rebase = (%v, %v, %v), want the item merged although git has no base stage for it", ok, conflict, stamped)
	}
	if got := l.repo.Git("-C", l.worktree, "show", "HEAD:"+addedPath); got != `{"failure_count":1,"id":"`+addedID+`","notes":"lane"}` {
		t.Errorf("merged item = %s, want the lane's notes with main's failure_count", got)
	}
}

func TestRecoveryReasonCode(t *testing.T) {
	t.Parallel()

	needed, conflict := string(CodeGitFleetRebaseNeeded), string(CodeGitFleetRebaseConflict)
	cases := []struct {
		name      string
		code      string
		conflicts []string
		want      string
	}{
		{"a needed rebase with conflicts is a conflict", needed, []string{"go/a.go"}, conflict},
		{"a needed rebase with no conflicts stays needed", needed, nil, needed},
		{"another code is never rewritten", "GIT_FF_MERGE_DIVERGED", []string{"go/a.go"}, "GIT_FF_MERGE_DIVERGED"},
	}
	for _, tc := range cases {
		if got := recoveryReasonCode(tc.code, tc.conflicts); got != tc.want {
			t.Errorf("%s: recoveryReasonCode = %q, want %q", tc.name, got, tc.want)
		}
	}
}
