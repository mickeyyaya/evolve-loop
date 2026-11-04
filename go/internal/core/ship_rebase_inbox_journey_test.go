package core

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func stampOnMain(t *testing.T) func(*inboxLane) {
	return func(l *inboxLane) { writeInboxFile(t, filepath.Join(l.repo.Dir, ownItemPath), stampItem) }
}

func assertLaneUntouched(t *testing.T, l *inboxLane, head string, ok bool, stamped []string) {
	t.Helper()
	if ok || len(stamped) != 0 {
		t.Fatalf("rebase ok=%v stamped=%v, want a refused landing with no stamped path", ok, stamped)
	}
	if got := l.repo.Git("-C", l.worktree, "rev-parse", "HEAD"); got != head {
		t.Errorf("lane HEAD = %s, want the audited %s restored", got, head)
	}
	if _, err := os.Stat(filepath.Join(l.worktree, "go/lane.go")); err != nil {
		t.Errorf("the audited code is gone from the lane: %v", err)
	}
	if status := l.repo.Git("-C", l.worktree, "status", "--porcelain"); status != "" {
		t.Errorf("lane not clean:\n%s", status)
	}
}

func TestShipJourney_AFailedContinueAfterAResolveKeepsTheAuditedCommit(t *testing.T) {
	l := newInboxLane(t, func(l *inboxLane) {
		writeInboxFile(t, filepath.Join(l.worktree, ownItemPath), laneItem)
	}, stampOnMain(t))
	head := l.repo.Git("-C", l.worktree, "rev-parse", "HEAD")
	gittest.InstallFault(t, gittest.GitFault{Verb: "rebase", Args: []string{"--continue"}, Code: 1})

	ok, _, stamped, _ := l.rebase(t)

	assertLaneUntouched(t, l, head, ok, stamped)
}

func TestShipJourney_AFailedStageReadIsNotTheLaneDeletingTheItem(t *testing.T) {
	l := newInboxLane(t, func(l *inboxLane) {
		writeInboxFile(t, filepath.Join(l.worktree, ownItemPath), laneItem)
	}, stampOnMain(t))
	head := l.repo.Git("-C", l.worktree, "rev-parse", "HEAD")
	gittest.InstallFault(t, gittest.GitFault{Verb: "show", Args: []string{":3:" + ownItemPath}, Code: 128})

	ok, _, stamped, _ := l.rebase(t)

	assertLaneUntouched(t, l, head, ok, stamped)
}

func TestShipJourney_ADeletionWithNoConsumedCopyStaysAConflict(t *testing.T) {
	t.Parallel()

	l := newInboxLane(t, func(l *inboxLane) {
		l.repo.Git("-C", l.worktree, "rm", "-q", "--", ownItemPath)
	}, stampOnMain(t))
	head := l.repo.Git("-C", l.worktree, "rev-parse", "HEAD")

	ok, conflict, stamped, cs := l.rebase(t)

	assertLaneUntouched(t, l, head, ok, stamped)
	if !conflict || !reflect.DeepEqual(cs.ShipRecoveryConflicts, []string{ownItemPath}) {
		t.Errorf("conflicts = %v, want the item named for the debugger", cs.ShipRecoveryConflicts)
	}
}

func TestShipJourney_AMergedItemThatLosesItsIdentityStaysAConflict(t *testing.T) {
	t.Parallel()

	for name, content := range map[string]string{
		"null":         `null`,
		"empty object": `{}`,
		"peer id":      `{"id":"` + peerItemID + `","notes":"original","title":"O6","weight":0.6}`,
	} {
		content := content
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			l := newInboxLane(t, func(l *inboxLane) {
				writeInboxFile(t, filepath.Join(l.worktree, ownItemPath), content)
			}, stampOnMain(t))
			head := l.repo.Git("-C", l.worktree, "rev-parse", "HEAD")

			ok, conflict, stamped, _ := l.rebase(t)

			assertLaneUntouched(t, l, head, ok, stamped)
			if !conflict {
				t.Error("want the item reported as a conflict")
			}
		})
	}
}

func TestShipJourney_AnItemEditedByTwoLaneCommitsIsStampedOnce(t *testing.T) {
	t.Parallel()

	second := `{"id":"` + ownItemID + `","notes":"lane second","title":"O6","weight":0.6}`
	l := newInboxLane(t, func(l *inboxLane) {
		writeInboxFile(t, filepath.Join(l.worktree, ownItemPath), laneItem)
		l.repo.Git("-C", l.worktree, "add", "-A")
		l.repo.Git("-C", l.worktree, "commit", "-q", "-m", "lane first")
		writeInboxFile(t, filepath.Join(l.worktree, ownItemPath), second)
	}, stampOnMain(t))

	ok, _, stamped, _ := l.rebase(t)

	if !ok || !reflect.DeepEqual(stamped, []string{ownItemPath}) {
		t.Fatalf("rebase ok=%v stamped=%v, want the path once", ok, stamped)
	}
}
