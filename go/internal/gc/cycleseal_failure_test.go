package gc

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func TestSealedCycleReport_SummaryNamesTheHoldTheCountsAndTheErrors(t *testing.T) {
	cases := []struct {
		name string
		rep  SealedCycleReport
		want []string
	}{
		{"held", SealedCycleReport{Held: "paused"}, []string{"held: paused"}},
		{"clean", SealedCycleReport{Reaped: []Process{{Pid: 4}}, Items: []WorktreeItem{{}, {}}}, []string{"stopped 1 process(es)", "released 2 item(s)"}},
		{"errors", SealedCycleReport{Errors: []string{"a", "b"}}, []string{"2 error(s): a; b"}},
	}
	for _, c := range cases {
		got := c.rep.Summary()
		for _, w := range c.want {
			if !strings.Contains(got, w) {
				t.Errorf("%s: Summary() = %q, want it to contain %q", c.name, got, w)
			}
		}
	}
}

func TestSweepSealedCycle_ACorruptContinuationRegistryStopsTheSweepAndIsReported(t *testing.T) {
	e := newSealEnv(t)
	sealed := e.lane(800, true)
	writeFile(t, continuation.RegistryPath(e.repo.Dir), "{not json")

	rep := e.sweep(800)

	if len(rep.Errors) != 1 || !strings.Contains(rep.Errors[0], "continuation registry") {
		t.Errorf("Errors = %v, want the unreadable registry", rep.Errors)
	}
	e.assertLaneKept(800, "f.txt", "base\nuncommitted edit\n")
	if _, err := os.Stat(sealed); err != nil {
		t.Errorf("lane must stay: %v", err)
	}
}

func TestSweepSealedCycle_AShipLockThatCannotBeTakenKeepsTheLane(t *testing.T) {
	e := newSealEnv(t)
	e.lane(800, true)
	if err := os.MkdirAll(filepath.Join(e.evolveDir, "ship.lock"), 0o755); err != nil {
		t.Fatal(err)
	}

	rep := e.sweep(800)

	if len(rep.Errors) != 1 || !strings.Contains(rep.Errors[0], "ship.lock") {
		t.Errorf("Errors = %v, want the lock failure", rep.Errors)
	}
	e.assertLaneKept(800, "f.txt", "base\nuncommitted edit\n")
}

func TestSweepSealedCycle_AGitFailureWhileListingTheLanesKeepsEverything(t *testing.T) {
	faults := []gittest.GitFault{
		{Verb: "worktree"},
		{Verb: "branch"},
		{Verb: "branch", Skip: 1},
	}
	for _, f := range faults {
		t.Run(f.Verb+string(rune('0'+f.Skip)), func(t *testing.T) {
			e := newSealEnv(t)
			e.lane(800, true)
			f.Code = 2
			gittest.InstallFault(t, f)

			rep := e.sweep(800)

			if len(rep.Errors) == 0 || len(rep.Items) != 0 {
				t.Errorf("a failed listing must be reported and release nothing: %+v", rep)
			}
			e.assertLaneKept(800, "f.txt", "base\nuncommitted edit\n")
		})
	}
}

func TestSweepSealedCycle_AFailedStepOfTheReleaseKeepsWhatItCouldNotSave(t *testing.T) {
	cases := []struct {
		name  string
		fault gittest.GitFault
		dirty bool
		want  string
	}{
		{"status", gittest.GitFault{Verb: "status"}, true, "status"},
		{"salvage", gittest.GitFault{Verb: "ls-files"}, true, "ls-files"},
		{"remove", gittest.GitFault{Verb: "worktree", Skip: 1}, false, "worktree remove"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newSealEnv(t)
			e.lane(800, c.dirty)
			c.fault.Code = 2
			gittest.InstallFault(t, c.fault)

			rep := e.sweep(800)

			if len(rep.Errors) != 1 || !strings.Contains(rep.Errors[0], c.want) {
				t.Errorf("Errors = %v, want one error naming %q", rep.Errors, c.want)
			}
			if len(rep.Items) != 0 {
				t.Errorf("Items = %+v, want none: nothing was released", rep.Items)
			}
			if !gittest.BranchExists(t, e.repo.Dir, e.leaf(800)) {
				t.Errorf("the branch must stay when the lane did not go")
			}
		})
	}
}

func TestSweepSealedCycle_ALandedBranchGoesWithoutABundle(t *testing.T) {
	e := newSealEnv(t)
	landed := filepath.Join(e.base, e.leaf(800))
	e.repo.Git("worktree", "add", "-q", landed, "-b", e.leaf(800))
	e.repo.Git("branch", e.leaf(801))

	rep := e.sweep(800)
	rep801 := e.sweep(801)

	if len(rep.Errors) != 0 || len(rep801.Errors) != 0 {
		t.Fatalf("errors: %v %v", rep.Errors, rep801.Errors)
	}
	for _, n := range []int{800, 801} {
		if gittest.BranchExists(t, e.repo.Dir, e.leaf(n)) {
			t.Errorf("landed branch %s must be released", e.leaf(n))
		}
	}
	if _, err := os.Stat(landed); !os.IsNotExist(err) {
		t.Errorf("the landed lane must be released: %v", err)
	}
	if got := e.bundleHolding(strings.TrimSpace(e.repo.Git("rev-parse", "main")), e.leaf(800)); got != "" {
		t.Errorf("a landed branch needs no bundle, found %s", got)
	}
}

func TestSweepSealedCycle_ABranchThatAnotherLaneHoldsIsBundledButNotDeleted(t *testing.T) {
	e := newSealEnv(t)
	e.repo.Git("checkout", "-q", "-b", e.leaf(800))
	e.repo.Write("n.txt", "n\n")
	e.repo.Git("add", "n.txt")
	e.repo.Git("commit", "-q", "-m", "unmerged")
	e.repo.Git("checkout", "-q", "main")
	elsewhere := filepath.Join(t.TempDir(), "elsewhere")
	e.repo.Git("worktree", "add", "-q", elsewhere, e.leaf(800))

	rep := e.sweep(800)

	if len(rep.Errors) != 1 || !strings.Contains(rep.Errors[0], "checked out") {
		t.Errorf("Errors = %v, want the checked-out refusal", rep.Errors)
	}
	if !gittest.BranchExists(t, e.repo.Dir, e.leaf(800)) {
		t.Errorf("a branch held by a lane outside the base must stay")
	}
}

func TestSweepSealedCycle_ABranchWhoseRefCannotBeDeletedIsReported(t *testing.T) {
	e := newSealEnv(t)
	e.repo.Git("checkout", "-q", "-b", e.leaf(800))
	e.repo.Write("n.txt", "n\n")
	e.repo.Git("add", "n.txt")
	e.repo.Git("commit", "-q", "-m", "unmerged")
	e.repo.Git("checkout", "-q", "main")
	gittest.InstallFault(t, gittest.GitFault{Verb: "update-ref", Code: 2})

	rep := e.sweep(800)

	if len(rep.Errors) != 1 || len(rep.Items) != 0 {
		t.Errorf("want one error and no item, got %+v", rep)
	}
	if !gittest.BranchExists(t, e.repo.Dir, e.leaf(800)) {
		t.Errorf("the branch must stay")
	}
}

func TestSweepSealedCycle_ASquashCheckThatFailsKeepsTheBranch(t *testing.T) {
	e := newSealEnv(t)
	e.repo.Git("checkout", "-q", "-b", e.leaf(800))
	e.repo.Write("n.txt", "n\n")
	e.repo.Git("add", "n.txt")
	e.repo.Git("commit", "-q", "-m", "unmerged")
	e.repo.Git("checkout", "-q", "main")
	gittest.InstallFault(t, gittest.GitFault{Verb: "diff", Code: 2})

	rep := e.sweep(800)

	if len(rep.Errors) != 1 {
		t.Errorf("Errors = %v, want the failed landing check", rep.Errors)
	}
	if !gittest.BranchExists(t, e.repo.Dir, e.leaf(800)) {
		t.Errorf("the branch must stay when its landing cannot be judged")
	}
}

func TestSweepSealedCycle_AKillThatFailsIsReportedAndTheLaneIsStillReleased(t *testing.T) {
	e := newSealEnv(t)
	sealed := e.lane(800, false)
	e.orphan(20, sealed)

	rep := SweepSealedCycle(context.Background(), e.opts(), 800, func(int) error { return errors.New("denied") })

	if len(rep.Errors) != 1 || !strings.Contains(rep.Errors[0], "denied") || len(rep.Reaped) != 0 {
		t.Errorf("rep = %+v, want one signal error and nothing reaped", rep)
	}
	if _, err := os.Stat(sealed); !os.IsNotExist(err) {
		t.Errorf("the lane must still be released: %v", err)
	}
}
