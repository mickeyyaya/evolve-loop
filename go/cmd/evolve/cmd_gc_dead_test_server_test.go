package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
)

var deadOwnerServerSuffixes = []string{"", "-hermetic", "-secret", "-failstart", "-twofp", "-earlykill", "-notmine"}

func plantOwnerSockets(t *testing.T, dir string, pid int) []string {
	t.Helper()
	var names []string
	for _, suffix := range deadOwnerServerSuffixes {
		name := bridge.DeriveTestSocket(pid) + suffix
		writeE2File(t, filepath.Join(dir, name), "")
		names = append(names, name)
	}
	return names
}

func TestGCSockets_ADryRunNamesEveryDeadOwnerTestServerAndNeverALiveOne(t *testing.T) {
	t.Setenv("TMUX_TMPDIR", t.TempDir())
	dir := bridge.TmuxSocketDir()
	dead := plantOwnerSockets(t, dir, deadE2PID)
	live := plantOwnerSockets(t, dir, os.Getpid())
	var stdout, stderr bytes.Buffer

	failed := newGCRun(context.Background(), true, &stdout, &stderr).sockets()

	if failed || stderr.Len() != 0 {
		t.Fatalf("failed=%v stderr=%q", failed, stderr.String())
	}
	for _, name := range dead {
		if !strings.Contains(stdout.String(), "  socket "+name+"\n") {
			t.Errorf("the dry run does not list %s, the server of a dead test binary:\n%s", name, stdout.String())
		}
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("a dry run touched %s: %v", name, err)
		}
	}
	for _, name := range live {
		if strings.Contains(stdout.String(), name) {
			t.Errorf("the dry run lists %s, the server of a live test binary:\n%s", name, stdout.String())
		}
	}
}

func TestGCSockets_AnApplyStopsEveryDeadOwnerTestServerAndSparesALiveOne(t *testing.T) {
	t.Setenv("TMUX_TMPDIR", t.TempDir())
	dir := bridge.TmuxSocketDir()
	dead := plantOwnerSockets(t, dir, deadE2PID)
	live := plantOwnerSockets(t, dir, os.Getpid())
	var stdout, stderr bytes.Buffer

	newGCRun(context.Background(), false, &stdout, &stderr).sockets()

	for _, name := range dead {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Errorf("gc left %s, the server of a dead test binary, in place (stat err=%v)", name, err)
		}
	}
	for _, name := range live {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("gc stopped %s, the server of a live test binary: %v", name, err)
		}
	}
}
