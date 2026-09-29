package swarm

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestExecKillServer_RemovesTheDeadServersSocketFile(t *testing.T) {
	t.Setenv("TMUX_TMPDIR", t.TempDir())
	dir := tmuxSocketDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	socket := "evolve-bridge-p999999"
	path := filepath.Join(dir, socket)
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := ExecKillServer(context.Background(), socket); err != nil {
		t.Fatalf("ExecKillServer: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("stat %s: %v; a reaped dead server leaves no socket file behind, or every gc run reaps it again", path, err)
	}
	if err := ExecKillServer(context.Background(), socket); err != nil {
		t.Errorf("a socket already gone is still a success: %v", err)
	}
}

func TestReapOrphanSockets_ReapsTheSameDeadSocketOnlyOnce(t *testing.T) {
	t.Setenv("TMUX_TMPDIR", t.TempDir())
	dir := tmuxSocketDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, pid := range []int{999991, 999992} {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("evolve-bridge-p%d", pid)), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	dead := func(int) bool { return false }

	first := ReapOrphanSockets(context.Background(), ExecListBridgeSockets, dead, ExecKillServer)
	second := ReapOrphanSockets(context.Background(), ExecListBridgeSockets, dead, ExecKillServer)

	if len(first.Killed) != 2 || len(second.Killed) != 0 {
		t.Errorf("first reaped %v, second %v; the 79 sockets gc 'reaped' on 2026-09-29 were all still listed", first.Killed, second.Killed)
	}
}
