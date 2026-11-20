package tmuxtest

import (
	"os"
	"os/exec"
	"path/filepath"
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
