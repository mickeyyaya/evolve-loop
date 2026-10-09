package tmuxtest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/fakeclitest"
)

type runFunc func() int

func (f runFunc) Run() int { return f() }

func privateSocketDir(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}
	dir, err := os.MkdirTemp("/tmp", "tmt")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	t.Setenv("TMUX_TMPDIR", dir)
	t.Setenv(bridge.TmuxSocketEnv, bridge.DeriveRunSocket(1))
	return filepath.Join(dir, "tmux-"+strconv.Itoa(os.Getuid()))
}

func tmux(args ...string) (string, error) {
	out, err := exec.Command("tmux", bridge.TmuxSocketArgs(args...)...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

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

func TestStartServer_RefusesAnEmptySocketBeforeItStartsAnything(t *testing.T) {
	err := StartServer("")

	if err == nil || !strings.Contains(err.Error(), "empty socket name") {
		t.Fatalf("StartServer(\"\") err=%v, want the empty-socket refusal: an empty name would select the default server", err)
	}
}

func TestStopServer_RefusesAnEmptySocketSoItNeverKillsTheDefaultServer(t *testing.T) {
	err := StopServer("")

	if err == nil || !strings.Contains(err.Error(), "empty socket name") {
		t.Fatalf("StopServer(\"\") err=%v, want the empty-socket refusal", err)
	}
}
