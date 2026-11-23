//go:build !notmux

package tmuxtest

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
)

func TestMain_RunsTheTestsOnAFreshServerOwnedByThisProcess(t *testing.T) {
	socketDir := privateSocketDir(t)
	own := bridge.DeriveTestSocket(os.Getpid())
	stale := exec.Command("tmux", "-L", own, "-f", os.DevNull, "new-session", "-d", "-s", "left-by-a-dead-process")
	if out, err := stale.CombinedOutput(); err != nil {
		t.Fatalf("seed a stale server on %s: %v: %s", own, err, out)
	}
	seen := map[string]string{}

	code := Main(runFunc(func() int {
		seen["socket"] = os.Getenv(bridge.TmuxSocketEnv)
		seen["default-command"], _ = tmux("show-options", "-gv", "default-command")
		seen["exit-empty"], _ = tmux("show-options", "-sv", "exit-empty")
		_, staleErr := tmux("has-session", "-t", bridge.ExactSessionTarget("left-by-a-dead-process"))
		seen["stale-session-gone"] = strconv.FormatBool(staleErr != nil)
		return 3
	}))

	want := map[string]string{
		"socket":             own,
		"default-command":    "exec " + paneShell,
		"exit-empty":         "off",
		"stale-session-gone": "true",
	}
	for k, v := range want {
		if seen[k] != v {
			t.Errorf("during the run %s = %q, want %q", k, seen[k], v)
		}
	}
	if code != 3 {
		t.Errorf("Main returned %d, want the runner's exit code 3", code)
	}
	if _, err := os.Stat(filepath.Join(socketDir, own)); !os.IsNotExist(err) {
		t.Errorf("socket %s still present after Main (stat err %v); the server must be stopped and its socket removed", own, err)
	}
}

func TestMain_WithoutTmuxStillRunsTheTestsOnTheirOwnSocketName(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv(bridge.TmuxSocketEnv, bridge.DeriveRunSocket(1))
	var socket string

	code := Main(runFunc(func() int { socket = os.Getenv(bridge.TmuxSocketEnv); return 0 }))

	if code != 0 || socket != bridge.DeriveTestSocket(os.Getpid()) {
		t.Fatalf("Main = %d with socket %q, want 0 on %q", code, socket, bridge.DeriveTestSocket(os.Getpid()))
	}
}

func TestMain_AServerThatCannotStartRunsNothingAndFails(t *testing.T) {
	bin := t.TempDir()
	fakeclitest.Install(t, filepath.Join(bin, "tmux"), "#!/bin/sh\nexit 1\n")
	t.Setenv("PATH", bin)
	t.Setenv("TMUX_TMPDIR", t.TempDir())
	ran := false

	code := Main(runFunc(func() int { ran = true; return 0 }))

	if code == 0 || ran {
		t.Fatalf("Main = %d, ran = %v; a test binary whose tmux server cannot start must fail without running", code, ran)
	}
}

func TestMain_AGuardThatCannotStartStopsTheServerAndRunsNothing(t *testing.T) {
	bin := t.TempDir()
	log := filepath.Join(t.TempDir(), "calls")
	fakeclitest.Install(t, filepath.Join(bin, "tmux"), "#!/bin/sh\necho \"$*\" >> "+log+"\ncase \"$*\" in *new-session*) exit 1 ;; esac\nexit 0\n")
	t.Setenv("PATH", bin)
	t.Setenv("TMUX_TMPDIR", t.TempDir())
	ran := false

	code := Main(runFunc(func() int { ran = true; return 0 }))

	calls, _ := os.ReadFile(log)
	if code != 1 || ran || !strings.HasSuffix(strings.TrimSpace(string(calls)), "kill-server") {
		t.Fatalf("Main = %d, ran = %v, tmux calls:\n%s\nwant 1, no run, and a stopped server", code, ran, calls)
	}
}

func TestMain_AServerThatCannotBeStoppedFailsEvenAGreenRun(t *testing.T) {
	bin := t.TempDir()
	fakeclitest.Install(t, filepath.Join(bin, "tmux"), "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", bin)
	t.Setenv("TMUX_TMPDIR", t.TempDir())
	t.Setenv(bridge.TmuxSocketEnv, bridge.DeriveRunSocket(1))
	cases := []struct {
		name    string
		runCode int
		want    int
	}{
		{"green run becomes red", 0, 1},
		{"red run keeps its code", 4, 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stuck := filepath.Join(bridge.TmuxSocketDir(), bridge.DeriveTestSocket(os.Getpid()))
			t.Cleanup(func() { _ = os.RemoveAll(stuck) })

			code := Main(runFunc(func() int {
				if err := os.MkdirAll(filepath.Join(stuck, "occupied"), 0o755); err != nil {
					t.Fatal(err)
				}
				return tc.runCode
			}))

			if code != tc.want {
				t.Errorf("Main = %d, want %d when the socket cannot be removed after the run", code, tc.want)
			}
		})
	}
}

func TestMain_ReapsDeadTestServersBeforeRunningWithFakeProcessTable(t *testing.T) {
	bin := t.TempDir()
	fakeclitest.Install(t, filepath.Join(bin, "tmux"), "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", bin)
	t.Setenv("TMUX_TMPDIR", t.TempDir())
	t.Setenv(bridge.TmuxSocketEnv, bridge.DeriveRunSocket(1))
	const deadPID, livePID = 333, 444
	prevList, prevAlive, prevKill := testSocketLister, pidAlive, serverKiller
	t.Cleanup(func() { testSocketLister, pidAlive, serverKiller = prevList, prevAlive, prevKill })
	testSocketLister = func() ([]string, error) {
		return []string{
			bridge.DeriveTestSocket(deadPID),
			bridge.DeriveTestSocket(deadPID) + "-hermetic",
			bridge.DeriveTestSocket(livePID),
			bridge.DeriveTestSocket(livePID) + "-hermetic",
		}, nil
	}
	pidAlive = func(pid int) bool { return pid == livePID || pid == os.Getpid() }
	var killedDuringStart []string
	serverKiller = func(_ context.Context, socket string) error {
		killedDuringStart = append(killedDuringStart, socket)
		return nil
	}
	var seen []string

	code := Main(runFunc(func() int {
		seen = slices.Clone(killedDuringStart)
		return 0
	}))

	if code != 0 {
		t.Fatalf("Main = %d, want 0", code)
	}
	slices.Sort(seen)
	want := []string{
		bridge.DeriveTestSocket(deadPID),
		bridge.DeriveTestSocket(deadPID) + "-hermetic",
	}
	if !slices.Equal(seen, want) {
		t.Fatalf("killed before test run = %v, want %v", seen, want)
	}
}

func TestMain_ContinuesWhenReapDeadTestServersFails(t *testing.T) {
	bin := t.TempDir()
	fakeclitest.Install(t, filepath.Join(bin, "tmux"), "#!/bin/sh\nexit 0\n")
	t.Setenv("PATH", bin)
	t.Setenv("TMUX_TMPDIR", t.TempDir())
	t.Setenv(bridge.TmuxSocketEnv, bridge.DeriveRunSocket(1))
	prevList := testSocketLister
	t.Cleanup(func() { testSocketLister = prevList })
	testSocketLister = func() ([]string, error) {
		return nil, errors.New("socket scan failed")
	}
	ran := false

	code := Main(runFunc(func() int {
		ran = true
		return 0
	}))

	if code != 0 || !ran {
		t.Fatalf("Main = %d, ran = %v; want 0 and runner executed even when reapDeadTestServers fails", code, ran)
	}
}
