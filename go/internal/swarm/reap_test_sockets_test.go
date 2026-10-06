package swarm

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
)

func TestReapOrphanSockets_ReapsADeadTestProcessSocketAndSparesALiveOne(t *testing.T) {
	list := func() ([]string, error) {
		return []string{bridge.DeriveTestSocket(111), bridge.DeriveTestSocket(222), bridge.DeriveRunSocket(333), bridge.TmuxSocket, bridge.TmuxSocket + "-test", bridge.TmuxSocket + "-tx1"}, nil
	}
	alive := func(pid int) bool { return pid == 222 }
	var killed []string
	kill := func(_ context.Context, socket string) error {
		killed = append(killed, socket)
		return nil
	}

	rep := ReapOrphanSockets(context.Background(), list, alive, kill)

	slices.Sort(killed)
	if !slices.Equal(killed, []string{bridge.DeriveRunSocket(333), bridge.DeriveTestSocket(111)}) || rep.SkippedLive != 1 {
		t.Fatalf("killed %v (skipped live %d); want only the dead test and loop owners reaped and the live test socket spared", killed, rep.SkippedLive)
	}
}

func TestExecListBridgeSockets_ListsTestProcessSockets(t *testing.T) {
	t.Setenv("TMUX_TMPDIR", t.TempDir())
	dir := tmuxSocketDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{bridge.DeriveTestSocket(7), bridge.DeriveRunSocket(8), bridge.TmuxSocket, "default"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	got, err := ExecListBridgeSockets()

	slices.Sort(got)
	if err != nil || !slices.Equal(got, []string{bridge.DeriveRunSocket(8), bridge.DeriveTestSocket(7)}) {
		t.Fatalf("ExecListBridgeSockets() = %v, %v; want the per-run and per-test-process sockets only", got, err)
	}
}
