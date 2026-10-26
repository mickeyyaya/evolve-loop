//go:build integration

package bridge

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const ownServerReadyTimeout = 10 * time.Second

func ownTmuxServer(t *testing.T, suffix string, env []string) string {
	t.Helper()
	requireTmux(t)
	socket := DeriveTestSocket(os.Getpid()) + suffix
	t.Setenv(TmuxSocketEnv, socket)
	t.Cleanup(func() { stopOwnTmuxServer(t, socket) })
	startOwnTmuxServer(t, socket, env)
	return socket
}

func startOwnTmuxServer(t *testing.T, socket string, env []string) {
	t.Helper()
	start := exec.Command("tmux", "-L", socket, "-f", os.DevNull, "start-server", ";", "set-option", "-s", "exit-empty", "off")
	start.Env = env
	if out, err := start.CombinedOutput(); err != nil {
		t.Fatalf("start the tmux server on socket %s: %v: %s", socket, err, out)
	}
	var out []byte
	var err error
	deadline := time.Now().Add(ownServerReadyTimeout)
	for time.Now().Before(deadline) {
		out, err = exec.Command("tmux", "-L", socket, "show-options", "-sv", "exit-empty").CombinedOutput()
		if err == nil && strings.TrimSpace(string(out)) == "off" {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("the tmux server on socket %s was not ready with exit-empty off within %s: %v: %s", socket, ownServerReadyTimeout, err, out)
}

func stopOwnTmuxServer(t *testing.T, socket string) {
	t.Helper()
	_ = exec.Command("tmux", "-L", socket, "kill-server").Run()
	if err := os.Remove(filepath.Join(TmuxSocketDir(), socket)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("remove the socket of the stopped tmux server %s: %v", socket, err)
	}
}
