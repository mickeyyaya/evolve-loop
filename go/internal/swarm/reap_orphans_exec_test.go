package swarm

import (
	"context"
	"errors"
	"os"
	"testing"
)

// Tests that swap the tmuxListRun or socketGlob seams must not call t.Parallel().

func TestExecPidAlive(t *testing.T) {
	t.Parallel()
	if !ExecPidAlive(os.Getpid()) {
		t.Fatal("ExecPidAlive(own pid) = false, want true (this process is alive)")
	}
	if ExecPidAlive(-1) {
		t.Fatal("ExecPidAlive(-1) = true — negative pid must be treated dead, never signalled")
	}
	if ExecPidAlive(0) {
		t.Fatal("ExecPidAlive(0) = true — pid 0 (caller's group) must be treated dead")
	}
}

func TestExecListBridgeSessions_ParsesLines(t *testing.T) {
	old := tmuxListRun
	defer func() { tmuxListRun = old }()
	tmuxListRun = func(_ context.Context, _ ...string) ([]byte, error) {
		return []byte("evolve-bridge-c1-build-pid1-n1-1\n\nevolve-recipe-c0-probe-pid2-n1-2\n"), nil
	}
	names, err := ExecListBridgeSessions(context.Background())
	if err != nil {
		t.Fatalf("ExecListBridgeSessions: %v", err)
	}
	if len(names) != 2 {
		t.Fatalf("names=%v, want 2 (blank line dropped)", names)
	}
}

func TestExecListBridgeSessions_NoServerIsNoError(t *testing.T) {
	old := tmuxListRun
	defer func() { tmuxListRun = old }()
	tmuxListRun = func(_ context.Context, _ ...string) ([]byte, error) {
		return nil, errors.New("no server running on /tmp/tmux-evolve-bridge")
	}
	names, err := ExecListBridgeSessions(context.Background())
	if err != nil || names != nil {
		t.Fatalf("got (names=%v, err=%v), want (nil, nil) for a stopped server", names, err)
	}
}

func TestExecListBridgeSessions_PartialOutputOnError(t *testing.T) {
	old := tmuxListRun
	defer func() { tmuxListRun = old }()
	tmuxListRun = func(_ context.Context, _ ...string) ([]byte, error) {
		return []byte("evolve-bridge-c1-build-pid1-n1-1\n"), errors.New("tmux died mid-list")
	}
	names, err := ExecListBridgeSessions(context.Background())
	if err == nil {
		t.Fatal("partial output + tmux error must return an error, not a truncated list")
	}
	if names != nil {
		t.Fatalf("names=%v on error, want nil (no action on a partial view)", names)
	}
}

func TestExecReapOrphans_CleanServerNoop(t *testing.T) {
	old := tmuxListRun
	defer func() { tmuxListRun = old }()
	tmuxListRun = func(_ context.Context, _ ...string) ([]byte, error) { return nil, nil }

	var rep OrphanReapReport = ExecReapOrphans(context.Background())
	if len(rep.Killed) != 0 || len(rep.Errors) != 0 {
		t.Fatalf("clean server must be a no-op, got %+v", rep)
	}
}

func TestExecListBridgeSockets_NamesOnly(t *testing.T) {
	old := socketGlob
	defer func() { socketGlob = old }()
	socketGlob = func() ([]string, error) { return []string{"evolve-bridge-p7"}, nil }
	got, err := ExecListBridgeSockets()
	if err != nil || len(got) != 1 || got[0] != "evolve-bridge-p7" {
		t.Fatalf("ExecListBridgeSockets = (%v, %v), want ([evolve-bridge-p7], nil)", got, err)
	}
}

func TestExecReapOrphanSockets_NoSocketsNoop(t *testing.T) {
	old := socketGlob
	defer func() { socketGlob = old }()
	socketGlob = func() ([]string, error) { return nil, nil }
	var rep OrphanSocketReport = ExecReapOrphanSockets(context.Background())
	if len(rep.Killed) != 0 || len(rep.Errors) != 0 {
		t.Fatalf("no sockets must be a no-op, got %+v", rep)
	}
}

func TestExecKillServer_RefusesEmpty(t *testing.T) {
	t.Parallel()
	if ExecKillServer(context.Background(), "") == nil {
		t.Fatal("ExecKillServer must refuse an empty socket name")
	}
}
