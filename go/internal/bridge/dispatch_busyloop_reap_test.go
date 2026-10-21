package bridge

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/proctree"
)

func orphanBusyLoop(t *testing.T, dir string) int {
	t.Helper()
	out, err := exec.Command("sh", "-c", "cd \"$1\" || exit 1; (while :; do :; done) >/dev/null 2>&1 & echo $!", "sh", dir).Output()
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
	return pid
}

func gone(pid int, within time.Duration) bool {
	for deadline := time.Now().Add(within); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
			return true
		}
	}
	return false
}

func TestDispatchSweep_AnUntaggedBusyLoopOrphanedInTheWorktreeStopsWhenTheDispatchEnds(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("lsof"); err != nil {
		t.Skip("lsof is not installed")
	}
	worktree := t.TempDir()
	loop := orphanBusyLoop(t, worktree)
	onlyTheLoop := func(pid int, sig syscall.Signal) error {
		if pid != loop {
			t.Errorf("the sweep signalled pid %d, which this test did not start", pid)
			return nil
		}
		return syscall.Kill(pid, sig)
	}
	var stderr bytes.Buffer
	deps := Deps{Stderr: &stderr, ListProcesses: proctree.ExecLister(), ListCwds: proctree.ExecCwdLister(), SignalProcess: onlyTheLoop, Sleep: time.Sleep}
	sweep := newDispatchSweep(mintDispatchID(&Config{RunID: "busyloop", Cycle: 1762, Agent: "audit"}, 1), "", worktree)

	sweep.finish(context.Background(), deps)

	if !gone(loop, 10*time.Second) {
		t.Fatalf("the busy loop orphaned in the worktree outlived the dispatch (cycle 1762); stderr: %s", stderr.String())
	}
}

func TestDispatchSweep_TheProjectRootIsNeverSweptAsAWorktree(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if s := newDispatchSweep("d", root, root); s.cwdProofDir != "" {
		t.Fatalf("cwdProofDir = %q; an in-place run shares the operator's tree, so its orphans are not the dispatch's", s.cwdProofDir)
	}
	if s := newDispatchSweep("d", root, root+"/.evolve/worktrees/cycle-a-1"); s.cwdProofDir == "" {
		t.Fatal("a lane worktree under the project root is swept")
	}
}

func TestDispatchSweep_AnUnreadableCwdListingKeepsTheTagSweepAndWarns(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	var signalled []int
	orphan := proctree.Process{Pid: 4242, Ppid: 1, Comm: "sh"}
	deps := Deps{Stderr: &stderr, Sleep: func(time.Duration) {},
		ListProcesses: func(context.Context) ([]proctree.Process, error) { return withSelf([]proctree.Process{orphan}), nil },
		ListCwds:      func(context.Context) ([]proctree.Cwd, error) { return nil, errors.New("no lsof") },
		SignalProcess: func(pid int, _ syscall.Signal) error { signalled = append(signalled, pid); return nil }}

	newDispatchSweep("d", "", "/wt").finish(context.Background(), deps)

	if len(signalled) != 0 || !strings.Contains(stderr.String(), "cannot list the process cwds") {
		t.Fatalf("signalled %v, stderr %q; an unproven orphan is spared and the gap is named", signalled, stderr.String())
	}
}
