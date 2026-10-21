package bridge

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/proctree"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
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

type sweepHost struct {
	worktree, root, home, dev string
	started                   time.Time
}

func newSweepHost(t *testing.T) sweepHost {
	t.Helper()
	root := gitCheckout(t, t.TempDir())
	worktree := gitCheckout(t, filepath.Join(root, ".evolve", "worktrees", "cycle-a-1"))
	dev := filepath.Join(root, "dev")
	gitCheckout(t, filepath.Join(dev, "cl-a"))
	return sweepHost{worktree: worktree, root: root, home: t.TempDir(), dev: dev, started: reapT0}
}

func gitCheckout(t *testing.T, dir string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: /nowhere\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func (h sweepHost) sweep(root, worktree string) *dispatchSweep {
	return newDispatchSweep(thisDispatch, root, worktree, h.home, h.started)
}

type cwdHost struct {
	procs     []proctree.Process
	cwds      []proctree.Cwd
	signalled []int
}

func (c *cwdHost) orphan(pid int, cwd string, started time.Time) *cwdHost {
	c.procs = append(c.procs, proctree.Process{Pid: pid, Ppid: 1, Started: started, Comm: "sh"})
	if real, err := filepath.EvalSymlinks(cwd); err == nil {
		cwd = real
	}
	c.cwds = append(c.cwds, proctree.Cwd{Pid: pid, Ppid: 1, Cwd: cwd})
	return c
}

func (c *cwdHost) deps(stderr *bytes.Buffer, center *signalcenter.Center) Deps {
	return Deps{Stderr: stderr, Signals: center, Sleep: func(time.Duration) {},
		ListProcesses: func(context.Context) ([]proctree.Process, error) { return withSelf(c.procs), nil },
		ListCwds:      func(context.Context) ([]proctree.Cwd, error) { return c.cwds, nil },
		SignalProcess: func(pid int, _ syscall.Signal) error { c.signalled = append(c.signalled, pid); return nil }}
}

func TestDispatchSweep_AnUntaggedBusyLoopOrphanedInTheWorktreeStopsWhenTheDispatchEnds(t *testing.T) {
	t.Parallel()
	if _, err := exec.LookPath("lsof"); err != nil {
		t.Skip("lsof is not installed")
	}
	worktree := gitCheckout(t, t.TempDir())
	started := time.Now()
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
	sweep := newDispatchSweep(mintDispatchID(&Config{RunID: "busyloop", Cycle: 1762, Agent: "audit"}, 1), t.TempDir(), worktree, t.TempDir(), started)

	sweep.finish(context.Background(), deps)

	if !gone(loop, 10*time.Second) {
		t.Fatalf("the busy loop orphaned in the worktree outlived the dispatch (cycle 1762); stderr: %s", stderr.String())
	}
}

func TestDispatchSweep_AnUnsafeBaseSignalsNothingAndWarns(t *testing.T) {
	t.Parallel()
	h := newSweepHost(t)
	for _, tc := range []struct {
		name, root, worktree, home string
	}{
		{"an empty project root", "", "/", h.home},
		{"an empty project root under a lane worktree", "", h.worktree, h.home},
		{"the filesystem root", h.root, "/", h.home},
		{"the project root itself", h.root, h.root, h.home},
		{"an ancestor of the project root", h.root, filepath.Dir(h.root), h.home},
		{"the home directory", h.root, h.home, h.home},
		{"an ancestor of the home directory", h.root, filepath.Dir(h.home), h.home},
		{"a git checkout that holds the home directory", h.root, h.worktree, filepath.Join(h.worktree, "home")},
		{"a relative worktree", h.root, "wt", h.home},
		{"a relative project root", "plane", h.worktree, h.home},
		{"an unknown home directory", h.root, h.worktree, ""},
		{"a worktree that does not exist", h.root, filepath.Join(h.root, "gone"), h.home},
		{"a directory that is not the top of a git checkout", h.root, filepath.Dir(h.worktree), h.home},
		{"the .evolve directory", h.root, filepath.Join(h.root, ".evolve"), h.home},
		{"the dev directory of the hub", h.root, h.dev, h.home},
		{"/private/var", h.root, "/private/var", h.home},
	} {
		host := (&cwdHost{}).orphan(700, "/", reapT0).orphan(701, h.worktree, reapT0).orphan(702, h.home, reapT0).orphan(703, h.root, reapT0)
		center := signalcenter.New()
		var stderr bytes.Buffer

		newDispatchSweep(thisDispatch, tc.root, tc.worktree, tc.home, reapT0).finish(context.Background(), host.deps(&stderr, center))
		center.Flush()

		if len(host.signalled) != 0 {
			t.Errorf("%s: signalled %v; an unsafe base is refused, never swept", tc.name, host.signalled)
		}
		if !slices.ContainsFunc(center.Recent(), func(e signalcenter.Event) bool {
			return e.Code == CodeDispatchSweepRefused && e.Severity == signalcenter.SeverityWarn && e.Fields["reason"] != ""
		}) || !strings.Contains(stderr.String(), "no orphan sweep by cwd") {
			t.Errorf("%s: events %+v, stderr %q; want a WARN %s that names the reason", tc.name, center.Recent(), stderr.String(), CodeDispatchSweepRefused)
		}
	}
}

func TestDispatchSweep_NoWorktreeIsNoCwdSweepAndNoWarning(t *testing.T) {
	t.Parallel()
	h := newSweepHost(t)
	host := (&cwdHost{}).orphan(750, "/", reapT0).orphan(751, h.root, reapT0)
	center := signalcenter.New()
	var stderr bytes.Buffer

	h.sweep("", "").finish(context.Background(), host.deps(&stderr, center))
	center.Flush()

	if len(host.signalled) != 0 || len(center.Recent()) != 0 || stderr.Len() != 0 {
		t.Fatalf("signalled %v, events %+v, stderr %q; a launch without a worktree asks for no cwd sweep", host.signalled, center.Recent(), stderr.String())
	}
}

func TestDispatchSweep_ALaneWorktreeSweepsOnlyItsOwnNewOrphans(t *testing.T) {
	t.Parallel()
	h := newSweepHost(t)
	base, err := filepath.EvalSymlinks(h.worktree)
	if err != nil {
		t.Fatal(err)
	}
	host := (&cwdHost{}).orphan(800, filepath.Join(base, "go"), reapT0.Add(time.Minute)).
		orphan(801, filepath.Join(base, "go"), reapT0.Add(-psStartSlack-time.Second)).
		orphan(802, h.root, reapT0.Add(time.Minute))
	var stderr bytes.Buffer

	h.sweep(h.root, h.worktree).finish(context.Background(), host.deps(&stderr, signalcenter.New()))

	if !slices.Equal(host.signalled, []int{800, 800}) {
		t.Fatalf("signalled %v; want only pid 800 (SIGTERM, then SIGKILL): 801 started before the dispatch and 802 is outside the worktree", host.signalled)
	}
}

func TestDispatchSweep_TheWindowAllowsForTheResolutionOfThePSStartTime(t *testing.T) {
	t.Parallel()
	h := newSweepHost(t)
	h.started = reapT0.Add(900 * time.Millisecond)
	base, err := filepath.EvalSymlinks(h.worktree)
	if err != nil {
		t.Fatal(err)
	}
	host := (&cwdHost{}).orphan(810, base, reapT0.Add(-time.Second)).
		orphan(812, base, reapT0.Add(-2*time.Second)).
		orphan(811, base, reapT0.Add(-3*time.Second))

	h.sweep(h.root, h.worktree).finish(context.Background(), host.deps(&bytes.Buffer{}, signalcenter.New()))

	if !slices.Equal(host.signalled, []int{810, 812, 810, 812}) {
		t.Fatalf("signalled %v; want 810 and 812 (SIGTERM, then SIGKILL): Linux ps can date a new orphan 1 s before the dispatch second, and the window takes 2 s; 811, 3 s older, is spared", host.signalled)
	}
}

func TestDispatchSweep_ASymlinkSwapAfterTheStartDoesNotWidenTheSweep(t *testing.T) {
	t.Parallel()
	h := newSweepHost(t)
	victim := t.TempDir()
	sweep := h.sweep(h.root, h.worktree)
	if err := os.Rename(h.worktree, h.worktree+".moved"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, h.worktree); err != nil {
		t.Fatal(err)
	}
	realVictim, err := filepath.EvalSymlinks(victim)
	if err != nil {
		t.Fatal(err)
	}
	host := (&cwdHost{}).orphan(900, realVictim, reapT0.Add(time.Minute))

	sweep.finish(context.Background(), host.deps(&bytes.Buffer{}, signalcenter.New()))

	if len(host.signalled) != 0 {
		t.Fatalf("signalled %v; the worktree became a link to %s after the dispatch start, and the sweep followed it", host.signalled, victim)
	}
}

func TestDispatchSweep_AnUnreadableCwdListingKeepsTheTagSweepAndWarns(t *testing.T) {
	t.Parallel()
	h := newSweepHost(t)
	var stderr bytes.Buffer
	var signalled []int
	orphan := proctree.Process{Pid: 4242, Ppid: 1, Started: reapT0, Comm: "sh"}
	deps := Deps{Stderr: &stderr, Sleep: func(time.Duration) {},
		ListProcesses: func(context.Context) ([]proctree.Process, error) { return withSelf([]proctree.Process{orphan}), nil },
		ListCwds:      func(context.Context) ([]proctree.Cwd, error) { return nil, errors.New("no lsof") },
		SignalProcess: func(pid int, _ syscall.Signal) error { signalled = append(signalled, pid); return nil }}

	h.sweep(h.root, h.worktree).finish(context.Background(), deps)

	if len(signalled) != 0 || !strings.Contains(stderr.String(), "cannot list the process cwds") {
		t.Fatalf("signalled %v, stderr %q; an unproven orphan is spared and the gap is named", signalled, stderr.String())
	}
}
