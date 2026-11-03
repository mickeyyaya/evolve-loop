package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

func corruptLease(t *testing.T, ws string, age time.Duration) {
	t.Helper()
	path := runlease.PathIn(ws)
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	at := time.Now().Add(-age)
	if err := os.Chtimes(path, at, at); err != nil {
		t.Fatal(err)
	}
}

func TestResumeHolder_ACorruptLeaseHoldsThePausedCycleForFindAndTakeUntilItsFileAgesPastTheTTL(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		age  time.Duration
		held bool
	}{
		"a fresh corrupt lease holds": {age: time.Minute, held: true},
		"an aged corrupt lease frees": {age: 2 * runlease.DefaultTTL, held: false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			evolveDir, worktree := t.TempDir(), t.TempDir()
			path := seedLaneRun(t, evolveDir, quotaPausedRun(5, worktree, "item-a"))
			ws := filepath.Dir(path)
			corruptLease(t, ws, tc.age)

			_, found := FindLaneResume(t.TempDir(), evolveDir, []string{"item-a"}, ResumeOptions{})
			err := TakeResumeLease(ws, time.Now())

			if found == tc.held {
				t.Errorf("FindLaneResume found = %v, want %v: Find and Take must agree on the corrupt lease", found, !tc.held)
			}
			var held *ResumeHeldError
			if tc.held && (!errors.As(err, &held) || held.Cause == nil || !strings.Contains(err.Error(), "unreadable")) {
				t.Errorf("TakeResumeLease = %v, want a *ResumeHeldError that names the unreadable lease", err)
			}
			if !tc.held {
				if got, ok, rerr := runlease.Read(ws); err != nil || rerr != nil || !ok || got.OwnerPID != os.Getpid() {
					t.Errorf("TakeResumeLease = %v, lease = (%+v, %v, %v); want the aged corrupt lease reclaimed by pid %d", err, got, ok, rerr, os.Getpid())
				}
			}
		})
	}
}

func TestReleaseResumeLease_RemovesOnlyTheAdmissionLeaseOfThisProcess(t *testing.T) {
	t.Parallel()
	now := time.Now()
	for name, tc := range map[string]struct {
		seed    func(t *testing.T, ws string)
		removed bool
	}{
		"the admission lease of this process": {func(t *testing.T, ws string) {
			if err := TakeResumeLease(ws, now); err != nil {
				t.Fatal(err)
			}
		}, true},
		"the heartbeat lease of the resumed run": {func(t *testing.T, ws string) {
			if err := runlease.Write(ws, runlease.Lease{RunID: "paused-run", OwnerPID: os.Getpid()}, now); err != nil {
				t.Fatal(err)
			}
		}, false},
		"the admission lease of another lane": {func(t *testing.T, ws string) {
			if err := runlease.Write(ws, runlease.Lease{RunID: resumeAdmissionRunID, OwnerPID: os.Getpid() + 1}, now); err != nil {
				t.Fatal(err)
			}
		}, false},
		"no lease": {func(*testing.T, string) {}, true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ws := t.TempDir()
			tc.seed(t, ws)

			if err := ReleaseResumeLease(ws); err != nil {
				t.Fatalf("ReleaseResumeLease: %v", err)
			}
			if _, ok, err := runlease.Read(ws); err != nil || ok == tc.removed {
				t.Errorf("lease present after release = %v (err %v), want present %v", ok, err, !tc.removed)
			}
		})
	}
}

func TestReleaseResumeLease_ACorruptLeaseIsLeftAndReported(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	corruptLease(t, ws, 0)

	if err := ReleaseResumeLease(ws); err == nil {
		t.Fatal("ReleaseResumeLease over a corrupt lease must report it, not guess its owner")
	}
	if _, err := os.Stat(runlease.PathIn(ws)); err != nil {
		t.Errorf("a corrupt lease was removed: %v", err)
	}
}

func TestResumeHolder_ALeaseThatCannotBeStattedFreesThePausedCycleForFindAndTake(t *testing.T) {
	t.Parallel()
	evolveDir, worktree := t.TempDir(), t.TempDir()
	ws := filepath.Dir(seedLaneRun(t, evolveDir, quotaPausedRun(5, worktree, "item-a")))
	if err := os.Symlink(runlease.FileName, runlease.PathIn(ws)); err != nil {
		t.Fatal(err)
	}

	_, found := FindLaneResume(t.TempDir(), evolveDir, []string{"item-a"}, ResumeOptions{})
	err := TakeResumeLease(ws, time.Now())

	if !found {
		t.Error("FindLaneResume skipped a cycle whose lease has no age: the cycle is stranded")
	}
	if got, ok, rerr := runlease.Read(ws); err != nil || rerr != nil || !ok || got.OwnerPID != os.Getpid() {
		t.Errorf("TakeResumeLease = %v, lease = (%+v, %v, %v); want the cycle reclaimed by pid %d", err, got, ok, rerr, os.Getpid())
	}
}
