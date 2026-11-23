//go:build acs

// Package cycle2625 encodes the acceptance of the recycle-x8-test-tmux-socket-reap inbox item: the orphan socket reaper sees the socket dirs of test binaries, stops the server of a dead owner, and spares the server of a live owner.
package cycle2625

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/swarm"
	"github.com/mickeyyaya/evolve-loop/go/internal/tmuxtest"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

type runFunc func() int

func (f runFunc) Run() int { return f() }

func privateDir(t *testing.T, prefix string) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", prefix)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func socketDirOf(dir string) string {
	return filepath.Join(dir, "tmux-"+strconv.Itoa(os.Getuid()))
}

func isolatedReaperDir(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Fatalf("tmux is required to prove a real server is stopped: %v", err)
	}
	t.Setenv("TMPDIR", privateDir(t, "x8r"))
	t.Setenv(bridge.TmuxSocketEnv, "")
	return privateDir(t, "x8q")
}

func registerThroughMain(t *testing.T, dir string, populate func(socketDir string)) {
	t.Helper()
	t.Setenv("TMUX_TMPDIR", dir)
	code := tmuxtest.Main(runFunc(func() int {
		populate(socketDirOf(dir))
		return 0
	}))
	if code != 0 {
		t.Fatalf("tmuxtest.Main exited %d", code)
	}
}

func tmuxIn(dir string, args ...string) *exec.Cmd {
	cmd := exec.Command("tmux", args...)
	cmd.Env = append(slices.DeleteFunc(os.Environ(), func(kv string) bool {
		return strings.HasPrefix(kv, "TMUX_TMPDIR=") || strings.HasPrefix(kv, "TMUX=")
	}), "TMUX_TMPDIR="+dir)
	return cmd
}

func plantServer(t *testing.T, dir, socket string) {
	t.Helper()
	if out, err := tmuxIn(dir, "-L", socket, "-f", os.DevNull, "new-session", "-d", "-s", "victim", "sleep 600").CombinedOutput(); err != nil {
		t.Fatalf("start the server on %s: %v: %s", socket, err, out)
	}
	t.Cleanup(func() { _ = tmuxIn(dir, "-L", socket, "kill-server").Run() })
}

func serverUp(dir, socket string) bool {
	return tmuxIn(dir, "-L", socket, "has-session", "-t", bridge.ExactSessionTarget("victim")).Run() == nil
}

func runningProcess(t *testing.T) *os.Process {
	t.Helper()
	cmd := exec.Command("sleep", "120")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _, _ = cmd.Process.Wait() })
	return cmd.Process
}

func TestC2625_001_DeadOwnerServerOnARegisteredTestDirIsStopped(t *testing.T) {
	reaperDir := isolatedReaperDir(t)
	dir := privateDir(t, "x8d")
	dead := bridge.DeriveTestSocket(acsassert.ExitedPID(t))
	registerThroughMain(t, dir, func(string) { plantServer(t, dir, dead) })
	if !serverUp(dir, dead) {
		t.Fatalf("fixture: the dead-owner server %s is not running before the reap", dead)
	}

	t.Setenv("TMUX_TMPDIR", reaperDir)
	rep := swarm.ExecReapOrphanSockets(context.Background())

	if serverUp(dir, dead) {
		t.Errorf("the server %s of a dead owner still runs after the reap: the reaper never saw the test socket dir", dead)
	}
	if acsassert.PathExists(filepath.Join(socketDirOf(dir), dead)) {
		t.Errorf("the socket file of %s is still in the test socket dir after the reap", dead)
	}
	if !slices.ContainsFunc(rep.Killed, func(k string) bool { return strings.HasSuffix(k, dead) }) {
		t.Errorf("report.Killed = %v; want it to name %s", rep.Killed, dead)
	}
}

func TestC2625_002_LiveOwnerServerOnARegisteredTestDirStays(t *testing.T) {
	reaperDir := isolatedReaperDir(t)
	dir := privateDir(t, "x8d")
	owner := runningProcess(t)
	live := bridge.DeriveTestSocket(owner.Pid)
	registerThroughMain(t, dir, func(string) { plantServer(t, dir, live) })

	t.Setenv("TMUX_TMPDIR", reaperDir)
	rep := swarm.ExecReapOrphanSockets(context.Background())

	if rep.SkippedLive < 1 {
		t.Errorf("report.SkippedLive = %d; want the reaper to see the live-owner socket %s and spare it on purpose", rep.SkippedLive, live)
	}
	if slices.ContainsFunc(rep.Killed, func(k string) bool { return strings.HasSuffix(k, live) }) {
		t.Errorf("report.Killed = %v; the live-owner socket %s must not be reaped", rep.Killed, live)
	}
	if !serverUp(dir, live) {
		t.Errorf("the server %s of a live owner was stopped", live)
	}
	if owner.Signal(syscall.Signal(0)) != nil {
		t.Errorf("the live owner process %d was signalled", owner.Pid)
	}
}

func TestC2625_003_TheReapStopsNothingButDeadOwnerSocketsAndSignalsNoProcess(t *testing.T) {
	reaperDir := isolatedReaperDir(t)
	vanished := privateDir(t, "x8v")
	registerThroughMain(t, vanished, func(string) {})
	if err := os.RemoveAll(vanished); err != nil {
		t.Fatal(err)
	}
	dir := privateDir(t, "x8d")
	owner, bystander := runningProcess(t), runningProcess(t)
	deadName := bridge.DeriveTestSocket(acsassert.ExitedPID(t))
	keep := []string{bridge.DeriveTestSocket(owner.Pid), bridge.TmuxSocket + "-t12ab", "default"}
	registerThroughMain(t, dir, func(socketDir string) {
		for _, name := range append([]string{deadName}, keep...) {
			if err := os.WriteFile(filepath.Join(socketDir, name), nil, 0o600); err != nil {
				t.Fatal(err)
			}
		}
	})

	t.Setenv("TMUX_TMPDIR", reaperDir)
	rep := swarm.ExecReapOrphanSockets(context.Background())

	if acsassert.PathExists(filepath.Join(socketDirOf(dir), deadName)) {
		t.Errorf("the dead-owner socket %s was not reaped from the registered dir", deadName)
	}
	for _, name := range keep {
		if !acsassert.PathExists(filepath.Join(socketDirOf(dir), name)) {
			t.Errorf("the reaper removed %s, a socket whose owner lives or whose name has no owner pid", name)
		}
	}
	for _, p := range []*os.Process{owner, bystander} {
		if p.Signal(syscall.Signal(0)) != nil {
			t.Errorf("process %d was signalled by the reap", p.Pid)
		}
	}
	if len(rep.Errors) != 0 {
		t.Errorf("report.Errors = %v; a registered dir that no longer exists must not raise an error", rep.Errors)
	}
}

func TestC2625_004_TheInjectedKillerSeesTheRegisteredDirsAndOnlyThem(t *testing.T) {
	reaperDir := isolatedReaperDir(t)
	registered, unregistered := privateDir(t, "x8d"), privateDir(t, "x8u")
	deadName := bridge.DeriveTestSocket(acsassert.ExitedPID(t))
	if err := os.MkdirAll(socketDirOf(unregistered), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(socketDirOf(unregistered), deadName), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	registerThroughMain(t, registered, func(socketDir string) {
		if err := os.WriteFile(filepath.Join(socketDir, deadName), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	})
	var asked []string
	recordOnly := func(_ context.Context, socket string) error {
		asked = append(asked, socket)
		return nil
	}

	t.Setenv("TMUX_TMPDIR", reaperDir)
	rep := swarm.ReapOrphanSockets(context.Background(), swarm.ExecListBridgeSockets, swarm.ExecPidAlive, recordOnly)

	named := func(socket string) bool { return strings.HasSuffix(socket, deadName) }
	if got := len(slices.DeleteFunc(slices.Clone(asked), func(s string) bool { return !named(s) })); got != 1 {
		t.Fatalf("the signal sender was asked to stop %v; want %s once, for the registered dir only", asked, deadName)
	}
	if got := len(slices.DeleteFunc(slices.Clone(rep.Killed), func(s string) bool { return !named(s) })); got != 1 {
		t.Errorf("report.Killed = %v; want %s once", rep.Killed, deadName)
	}
	if !acsassert.PathExists(filepath.Join(socketDirOf(unregistered), deadName)) {
		t.Errorf("a socket in %s, a dir that no test binary registered, was touched", unregistered)
	}
}
