//go:build integration

package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/tmuxtest"
)

func e2IsolatedTmuxServer(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}
	base, err := os.MkdirTemp("/tmp", "e2gc")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(base) })
	socket := "evolve-bridge-e2gc" + strconv.Itoa(os.Getpid())
	t.Setenv("TMUX", "")
	t.Setenv("TMUX_TMPDIR", base)
	t.Setenv(bridge.TmuxSocketEnv, socket)
	if err := tmuxtest.StartServer(socket); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tmuxtest.StopServer(socket) })
	return socket
}

func e2TmuxOn(t *testing.T, args ...string) string {
	t.Helper()
	out, err := exec.Command("tmux", bridge.TmuxSocketArgs(args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("tmux %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestGCSessions_ADryRunNamesTheOrphanOnARealServerAndKillsNothing(t *testing.T) {
	e2IsolatedTmuxServer(t)
	orphan := "evolve-bridge-e2-pid" + strconv.Itoa(deadE2PID)
	alive := "evolve-bridge-e2-pid" + strconv.Itoa(os.Getpid())
	e2TmuxOn(t, "new-session", "-d", "-s", orphan)
	e2TmuxOn(t, "new-session", "-d", "-s", alive)
	e2TmuxOn(t, "new-session", "-d", "-s", "someone-else")
	var stdout, stderr bytes.Buffer

	failed := newGCRun(context.Background(), true, &stdout, &stderr).sessions()

	want := "evolve gc --dry-run: 1 orphan session(s) would be reaped\n  WOULD-REAP " + orphan + "\nskipped: live=1 foreign=1 no-pid=0; errors=0\n"
	if failed || stdout.String() != want || stderr.Len() != 0 {
		t.Fatalf("failed=%v stdout=%q stderr=%q\nwant %q", failed, stdout.String(), stderr.String(), want)
	}
	if got := e2TmuxOn(t, "list-sessions", "-F", "#{session_name}"); !strings.Contains(got, orphan) {
		t.Fatalf("a dry run killed the orphan; sessions now:\n%s", got)
	}
}
