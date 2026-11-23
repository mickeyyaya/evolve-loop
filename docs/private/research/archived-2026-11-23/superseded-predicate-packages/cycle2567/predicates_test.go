//go:build acs

package cycle2567

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
)

type runFunc func() int

func (f runFunc) Run() int { return f() }

type fixture struct {
	reaperDir string
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Fatalf("tmux is required: %v", err)
	}
	reaperDir := privateDir(t, "x8q")
	t.Setenv("TMPDIR", privateDir(t, "x8r"))
	t.Setenv(bridge.TmuxSocketEnv, "")
	return fixture{reaperDir: reaperDir}
}

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

func (f fixture) registerThroughMain(t *testing.T, dir string, populate func(socketDir string)) {
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

func (f fixture) reap(t *testing.T) swarm.OrphanSocketReport {
	t.Helper()
	t.Setenv("TMUX_TMPDIR", f.reaperDir)
	return swarm.ExecReapOrphanSockets(context.Background())
}

func tmuxEnv(dir string) []string {
	env := []string{"TMUX_TMPDIR=" + dir}
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "TMUX_TMPDIR=") && !strings.HasPrefix(kv, "TMUX=") {
			env = append(env, kv)
		}
	}
	return env
}

func startServer(t *testing.T, dir, socket string) {
	t.Helper()
	cmd := exec.Command("tmux", "-L", socket, "-f", os.DevNull, "new-session", "-d", "-s", "victim", "sleep 600")
	cmd.Env = tmuxEnv(dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("start the server on %s: %v: %s", socket, err, out)
	}
	t.Cleanup(func() {
		stop := exec.Command("tmux", "-L", socket, "kill-server")
		stop.Env = tmuxEnv(dir)
		_ = stop.Run()
	})
}

func serverUp(dir, socket string) bool {
	cmd := exec.Command("tmux", "-L", socket, "has-session", "-t", bridge.ExactSessionTarget("victim"))
	cmd.Env = tmuxEnv(dir)
	return cmd.Run() == nil
}

func finishedPID(t *testing.T) int {
	t.Helper()
	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	return cmd.ProcessState.Pid()
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

func processUp(p *os.Process) bool { return p.Signal(syscall.Signal(0)) == nil }

func killedHas(rep swarm.OrphanSocketReport, socket string) bool {
	return slices.ContainsFunc(rep.Killed, func(k string) bool { return strings.HasSuffix(k, socket) })
}

func fileExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func TestC2567_001_DeadOwnerServerOnARegisteredTestDirIsStopped(t *testing.T) {
	f := newFixture(t)
	dir := privateDir(t, "x8d")
	dead := bridge.DeriveTestSocket(finishedPID(t))
	f.registerThroughMain(t, dir, func(string) { startServer(t, dir, dead) })
	if !serverUp(dir, dead) {
		t.Fatalf("fixture: the dead-owner server %s is not running before the reap", dead)
	}

	rep := f.reap(t)

	if serverUp(dir, dead) {
		t.Errorf("the server %s of a dead owner still runs after the reap: the reaper never saw the test socket dir", dead)
	}
	if fileExists(filepath.Join(socketDirOf(dir), dead)) {
		t.Errorf("the socket file of %s is still in the test socket dir after the reap", dead)
	}
	if !killedHas(rep, dead) {
		t.Errorf("report.Killed = %v; want it to name %s", rep.Killed, dead)
	}
}

func TestC2567_002_LiveOwnerServerOnARegisteredTestDirStays(t *testing.T) {
	f := newFixture(t)
	dir := privateDir(t, "x8d")
	owner := runningProcess(t)
	live := bridge.DeriveTestSocket(owner.Pid)
	f.registerThroughMain(t, dir, func(string) { startServer(t, dir, live) })

	rep := f.reap(t)

	if rep.SkippedLive < 1 {
		t.Errorf("report.SkippedLive = %d; want the reaper to see the live-owner socket %s and spare it on purpose", rep.SkippedLive, live)
	}
	if killedHas(rep, live) {
		t.Errorf("report.Killed = %v; the live-owner socket %s must not be reaped", rep.Killed, live)
	}
	if !serverUp(dir, live) {
		t.Errorf("the server %s of a live owner was stopped", live)
	}
	if !processUp(owner) {
		t.Errorf("the live owner process %d was signalled", owner.Pid)
	}
}

func TestC2567_003_TheReapStopsNothingButDeadOwnerSocketsAndSignalsNoProcess(t *testing.T) {
	f := newFixture(t)
	vanished := privateDir(t, "x8v")
	f.registerThroughMain(t, vanished, func(string) {})
	if err := os.RemoveAll(vanished); err != nil {
		t.Fatal(err)
	}
	dir := privateDir(t, "x8d")
	owner, bystander := runningProcess(t), runningProcess(t)
	deadName := bridge.DeriveTestSocket(finishedPID(t))
	keep := []string{bridge.DeriveTestSocket(owner.Pid), bridge.TmuxSocket + "-t12ab", "default"}
	f.registerThroughMain(t, dir, func(socketDir string) {
		for _, name := range append([]string{deadName}, keep...) {
			if err := os.WriteFile(filepath.Join(socketDir, name), nil, 0o600); err != nil {
				t.Fatal(err)
			}
		}
	})

	rep := f.reap(t)

	if fileExists(filepath.Join(socketDirOf(dir), deadName)) {
		t.Errorf("the dead-owner socket %s was not reaped from the registered dir", deadName)
	}
	for _, name := range keep {
		if !fileExists(filepath.Join(socketDirOf(dir), name)) {
			t.Errorf("the reaper removed %s, a socket whose owner lives or whose name has no owner pid", name)
		}
	}
	for _, p := range []*os.Process{owner, bystander} {
		if !processUp(p) {
			t.Errorf("process %d was signalled by the reap", p.Pid)
		}
	}
	if len(rep.Errors) != 0 {
		t.Errorf("report.Errors = %v; a registered dir that no longer exists must not raise an error", rep.Errors)
	}
}

func TestC2567_004_OnlyRegisteredTestDirsAreSwept(t *testing.T) {
	f := newFixture(t)
	registered, unregistered := privateDir(t, "x8d"), privateDir(t, "x8u")
	deadName := bridge.DeriveTestSocket(finishedPID(t))
	if err := os.MkdirAll(socketDirOf(unregistered), 0o700); err != nil {
		t.Fatal(err)
	}
	strangerSocket := filepath.Join(socketDirOf(unregistered), deadName)
	if err := os.WriteFile(strangerSocket, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	f.registerThroughMain(t, registered, func(socketDir string) {
		if err := os.WriteFile(filepath.Join(socketDir, deadName), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	})

	f.reap(t)

	if fileExists(filepath.Join(socketDirOf(registered), deadName)) {
		t.Errorf("the dead-owner socket in the registered dir was not reaped")
	}
	if !fileExists(strangerSocket) {
		t.Errorf("the reaper removed %s from a dir no test binary registered", strangerSocket)
	}
}
