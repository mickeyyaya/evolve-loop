//go:build acs

package cycle2237

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	pollEvery  = 100 * time.Millisecond
	pollRounds = 100
)

type socketDirState struct {
	name   string
	unlink bool
}

var socketDirStates = []socketDirState{
	{"socket directory intact", false},
	{"socket directory removed by a test cleanup", true},
}

type holder struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    *bufio.Reader
	socket    string
	serverPid int
	env       []string
}

func requireTmux(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}
}

func privateTmuxEnv(t *testing.T) []string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "c2237")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	env := []string{"TMUX_TMPDIR=" + dir}
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "TMUX_TMPDIR=") && !strings.HasPrefix(kv, "TMUX=") {
			env = append(env, kv)
		}
	}
	return env
}

func buildHolder(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	mod, err := acsassert.ModuleDir(cwd)
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(t.TempDir(), "holder")
	build := exec.Command("go", "build", "-tags", "acs", "-o", bin, "./acs/cycle2237/holder")
	build.Dir = mod
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build the test binary that runs a package through tmuxtest.Main: %v\n%s", err, out)
	}
	return bin
}

func tmuxServerPid(env []string, socket string) (int, error) {
	cmd := exec.Command("tmux", "-L", socket, "display-message", "-p", "#{pid}")
	cmd.Env = env
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(out)))
}

func pidAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

func killOwnPid(pid int) {
	if pid > 1 && pidAlive(pid) {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
}

func waitGone(pid int) bool {
	for i := 0; i < pollRounds; i++ {
		if !pidAlive(pid) {
			return true
		}
		time.Sleep(pollEvery)
	}
	return !pidAlive(pid)
}

func startHolder(t *testing.T, env []string, bin string, args ...string) *holder {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = env
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	h := &holder{cmd: cmd, stdin: stdin, stdout: bufio.NewReader(stdout), env: env}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		killOwnPid(h.serverPid)
	})
	line, err := h.stdout.ReadString('\n')
	if err != nil || !strings.HasPrefix(line, "ready ") {
		t.Fatalf("the test binary did not report ready: %q %v", line, err)
	}
	h.socket = strings.TrimSpace(strings.TrimPrefix(line, "ready "))
	h.serverPid, err = tmuxServerPid(env, h.socket)
	if err != nil || h.serverPid <= 1 {
		t.Fatalf("no tmux server on socket %s while the test binary runs: pid %d, %v", h.socket, h.serverPid, err)
	}
	return h
}

func (h *holder) proceed(t *testing.T) {
	t.Helper()
	if _, err := io.WriteString(h.stdin, "go\n"); err != nil {
		t.Fatal(err)
	}
	line, err := h.stdout.ReadString('\n')
	if err != nil || strings.TrimSpace(line) != "proceeded" {
		t.Fatalf("the test binary did not finish its step: %q %v", line, err)
	}
}

func (h *holder) sigkill(t *testing.T) {
	t.Helper()
	if err := h.cmd.Process.Signal(syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	_, _ = h.cmd.Process.Wait()
}

func holderArgs(state socketDirState, extra ...string) []string {
	args := extra
	if state.unlink {
		args = append(args, "unlink")
	}
	return args
}

func TestC2237_001_SigkillOfTheTestBinaryLeavesNoServerOnItsSocket(t *testing.T) {
	requireTmux(t)
	bin := buildHolder(t)
	for _, state := range socketDirStates {
		t.Run(state.name, func(t *testing.T) {
			h := startHolder(t, privateTmuxEnv(t), bin, holderArgs(state, "hold")...)
			h.proceed(t)

			h.sigkill(t)

			if !waitGone(h.serverPid) {
				t.Errorf("tmux server pid %d on socket %s outlived its SIGKILLed test binary by %s", h.serverPid, h.socket, time.Duration(pollRounds)*pollEvery)
			}
		})
	}
}

func TestC2237_002_ANormalRunLeavesNoServerBehind(t *testing.T) {
	requireTmux(t)
	bin := buildHolder(t)
	for _, state := range socketDirStates {
		t.Run(state.name, func(t *testing.T) {
			h := startHolder(t, privateTmuxEnv(t), bin, holderArgs(state)...)
			h.proceed(t)
			_ = h.cmd.Wait()

			if !waitGone(h.serverPid) {
				t.Errorf("tmux server pid %d on socket %s is still alive after the test binary exited normally; it is re-parented to pid 1", h.serverPid, h.socket)
			}
		})
	}
}

type bystanders struct {
	sleeper    *exec.Cmd
	serverPids map[string]int
}

func startBystanders(t *testing.T, env []string, ownerSocket string) bystanders {
	t.Helper()
	b := bystanders{serverPids: map[string]int{}}
	b.sleeper = exec.Command("sleep", "600")
	if err := b.sleeper.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.sleeper.Process.Kill(); _, _ = b.sleeper.Process.Wait() })
	for _, socket := range []string{ownerSocket + "9", "default"} {
		cmd := exec.Command("tmux", "-L", socket, "-f", os.DevNull, "new-session", "-d", "-s", "bystander", "sleep 600")
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("start the bystander server %s: %v: %s", socket, err, out)
		}
		pid, err := tmuxServerPid(env, socket)
		if err != nil {
			t.Fatalf("read the pid of the bystander server %s: %v", socket, err)
		}
		b.serverPids[socket] = pid
		t.Cleanup(func() { killOwnPid(pid) })
	}
	return b
}

func TestC2237_003_TheTeardownKillsNoProcessTheTestDidNotStart(t *testing.T) {
	requireTmux(t)
	bin := buildHolder(t)
	for _, state := range socketDirStates {
		t.Run(state.name, func(t *testing.T) {
			env := privateTmuxEnv(t)
			h := startHolder(t, env, bin, holderArgs(state, "hold")...)
			other := startBystanders(t, env, h.socket)
			h.proceed(t)

			h.sigkill(t)

			if !waitGone(h.serverPid) {
				t.Fatalf("tmux server pid %d of the killed test binary is still alive, so the teardown did not run", h.serverPid)
			}
			if !pidAlive(other.sleeper.Process.Pid) {
				t.Errorf("the teardown killed the bystander process %d that the test binary did not start", other.sleeper.Process.Pid)
			}
			for socket, pid := range other.serverPids {
				if !pidAlive(pid) {
					t.Errorf("the teardown killed the tmux server %s (pid %d) that the test binary did not start", socket, pid)
				}
			}
		})
	}
}
