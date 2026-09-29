package gc

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
)

func TestParseCwdListing_ReadsEachProcessPidParentAndCwd(t *testing.T) {
	t.Parallel()
	listing := "p331\nR99913\nfcwd\nn/hub/runtime\np348\nR1\nfcwd\nn/hub/runtime/.evolve/worktrees/cycle-a-5/go\np400\nR1\nfcwd\n"

	got := parseCwdListing(listing)

	want := []Process{
		{Pid: 331, Ppid: 99913, Cwd: "/hub/runtime"},
		{Pid: 348, Ppid: 1, Cwd: "/hub/runtime/.evolve/worktrees/cycle-a-5/go"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseCwdListing = %+v, want %+v (a record without a cwd is dropped, never guessed)", got, want)
	}
}

func TestFinishedCycleOrphans_TakesOnlyOrphansWhoseCwdIsInsideAFinishedCycleTree(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	base := filepath.Join(root, ".evolve", "worktrees")
	closed := filepath.Join(base, "cycle-cd3ae73e-1762")
	live := filepath.Join(base, "cycle-cd3ae73e-1763")
	for _, d := range []string{filepath.Join(closed, "go"), live, filepath.Join(base, "lane-notes")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(dossier.CyclesDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dossier.CyclesDir(root), "cycle-1762.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	o := WorktreeOptions{ProjectRoot: root, WorktreeBase: base}
	cases := []struct {
		name string
		p    Process
		reap bool
	}{
		{"orphan busy loop inside a closed-out cycle's tree", Process{Pid: 10, Ppid: 1, Cwd: closed}, true},
		{"orphan in a subdirectory of a closed-out tree", Process{Pid: 11, Ppid: 1, Cwd: filepath.Join(closed, "go")}, true},
		{"orphan whose cycle tree was already removed", Process{Pid: 12, Ppid: 1, Cwd: filepath.Join(base, "cycle-cd3ae73e-1700")}, true},
		{"orphan inside a live cycle's tree", Process{Pid: 13, Ppid: 1, Cwd: live}, false},
		{"attached process inside a closed-out tree", Process{Pid: 14, Ppid: 4242, Cwd: closed}, false},
		{"orphan in the operator's console", Process{Pid: 15, Ppid: 1, Cwd: filepath.Join(root, "console")}, false},
		{"orphan at the worktree base itself", Process{Pid: 16, Ppid: 1, Cwd: base}, false},
		{"orphan in a non-cycle directory under the base", Process{Pid: 17, Ppid: 1, Cwd: filepath.Join(base, "lane-notes")}, false},
		{"orphan in a sibling whose name only shares the base prefix", Process{Pid: 18, Ppid: 1, Cwd: base + "-old/cycle-cd3ae73e-1700"}, false},
	}
	for _, tc := range cases {
		got := o.FinishedCycleOrphans([]Process{tc.p})
		if (len(got) == 1) != tc.reap {
			t.Errorf("%s: reap=%v, want %v", tc.name, len(got) == 1, tc.reap)
		}
	}
}

func TestReapFinishedCycleOrphans_SignalsEachOwnedOrphanAndReportsFailures(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	base := filepath.Join(root, ".evolve", "worktrees")
	gone := filepath.Join(base, "cycle-cd3ae73e-1700")
	listing := "p20\nR1\nfcwd\nn" + gone + "\np21\nR1\nfcwd\nn" + gone + "/go\np22\nR1\nfcwd\nn" + root + "/console\n"
	var lsofArgs []string
	o := WorktreeOptions{ProjectRoot: root, WorktreeBase: base, Exec: func(_ context.Context, name, _ string, args, _ []string, _ io.Reader, stdout, _ io.Writer) (int, error) {
		lsofArgs = append([]string{name}, args...)
		_, _ = io.WriteString(stdout, listing)
		return 0, nil
	}}
	var signaled []int
	kill := func(pid int) error {
		signaled = append(signaled, pid)
		if pid == 21 {
			return errors.New("operation not permitted")
		}
		return nil
	}

	rep := ReapFinishedCycleOrphans(context.Background(), o, kill)

	if !reflect.DeepEqual(signaled, []int{20, 21}) {
		t.Fatalf("signaled %v, want [20 21]: only the orphans inside the finished tree, never the console process", signaled)
	}
	if len(rep.Reaped) != 1 || rep.Reaped[0].Pid != 20 {
		t.Errorf("Reaped = %+v, want only pid 20 (pid 21's signal failed)", rep.Reaped)
	}
	if len(rep.Errors) != 1 || !strings.Contains(rep.Errors[0], "21") {
		t.Errorf("Errors = %v, want the pid 21 failure named", rep.Errors)
	}
	if strings.Join(lsofArgs, " ") != "lsof -a -d cwd -u "+uidArg()+" -R -F pRn" {
		t.Errorf("listing ran %q", strings.Join(lsofArgs, " "))
	}
}

func TestReapFinishedCycleOrphans_AFailedListingSignalsNothingAndIsReported(t *testing.T) {
	t.Parallel()
	o := WorktreeOptions{ProjectRoot: t.TempDir(), WorktreeBase: "/x", Exec: func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
		return 0, errors.New("lsof: not found")
	}}
	kill := func(pid int) error {
		t.Errorf("signaled pid %d without a process listing", pid)
		return nil
	}

	rep := ReapFinishedCycleOrphans(context.Background(), o, kill)

	if len(rep.Errors) != 1 || !strings.Contains(rep.Errors[0], "lsof: not found") {
		t.Errorf("Errors = %v, want the listing failure surfaced", rep.Errors)
	}
}
