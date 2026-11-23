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
	"github.com/mickeyyaya/evolve-loop/go/internal/testhost"
)

type runFunc func() int

func (f runFunc) Run() int { return f() }

func privateSocketDir(t *testing.T) string {
	t.Helper()
	testhost.Tool(t, "tmux")
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

func TestReapDeadTestServers_ReapsDeadTestSocketsAndSparesLiveAndLoopSockets(t *testing.T) {
	t.Setenv("TMUX_TMPDIR", t.TempDir())
	dir := bridge.TmuxSocketDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	const deadPID, livePID = 111, 222
	entries := []string{
		bridge.DeriveTestSocket(deadPID),
		bridge.DeriveTestSocket(deadPID) + "-hermetic",
		bridge.DeriveTestSocket(deadPID) + "-earlykill",
		bridge.DeriveTestSocket(livePID),
		bridge.DeriveTestSocket(livePID) + "-hermetic",
		bridge.DeriveRunSocket(deadPID),
		bridge.TmuxSocket,
		bridge.DeriveTestSocket(deadPID) + "x",
	}
	for _, name := range entries {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	prevAlive, prevKill := pidAlive, serverKiller
	t.Cleanup(func() { pidAlive, serverKiller = prevAlive, prevKill })
	pidAlive = func(pid int) bool { return pid == livePID }
	var killed []string
	serverKiller = func(_ context.Context, socket string) error {
		killed = append(killed, socket)
		return nil
	}

	if err := reapDeadTestServers(); err != nil {
		t.Fatalf("reapDeadTestServers: %v", err)
	}

	slices.Sort(killed)
	want := []string{
		bridge.DeriveTestSocket(deadPID),
		bridge.DeriveTestSocket(deadPID) + "-earlykill",
		bridge.DeriveTestSocket(deadPID) + "-hermetic",
	}
	if !slices.Equal(killed, want) {
		t.Fatalf("killed = %v, want %v", killed, want)
	}
}

func TestListTestSockets_ReturnsSocketGlobError(t *testing.T) {
	t.Setenv("TMUX_TMPDIR", "[invalid-glob")

	_, err := listTestSockets()

	if !errors.Is(err, filepath.ErrBadPattern) {
		t.Fatalf("listTestSockets() err = %v, want filepath.ErrBadPattern", err)
	}
}

func TestReapDeadTestServers_ReturnsErrorWhenReapFails(t *testing.T) {
	prevList := testSocketLister
	t.Cleanup(func() { testSocketLister = prevList })
	testSocketLister = func() ([]string, error) {
		return nil, errors.New("socket list unavailable")
	}

	if err := reapDeadTestServers(); err == nil {
		t.Fatal("reapDeadTestServers() err = nil, want non-nil error")
	}
}
