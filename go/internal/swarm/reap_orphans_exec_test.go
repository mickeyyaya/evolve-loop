// reap_orphans_exec_test.go — production exec-layer contracts + apicover naming
// for the symbols only the wiring (not the pure suite) exercises. The seam
// tests mutate the package-global tmuxListRun, so they do NOT run in parallel.
package swarm

import (
	"context"
	"errors"
	"os"
	"testing"
)

// TestExecPidAlive: the running test process is alive; sentinel/absurd pids are
// dead. This is the liveness oracle the GC trusts to spare concurrent runs.
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

// TestExecListBridgeSessions_ParsesLines pins the line parsing: one name per
// line, blanks dropped. Drives the production lister through the tmuxListRun
// seam so it never touches a real server.
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

// TestExecListBridgeSessions_NoServerIsNoError: a stopped server exits non-zero
// with empty output — that is "nothing to reap", not a failure.
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

// TestExecListBridgeSessions_PartialOutputOnError: when tmux errors but has
// already emitted partial output, the lister must surface the error and drop the
// partial list — never act on a truncated view of the server.
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

// TestExecReapOrphans_CleanServerNoop names+executes the production wiring end
// to end against an empty server — it must be a harmless no-op.
func TestExecReapOrphans_CleanServerNoop(t *testing.T) {
	old := tmuxListRun
	defer func() { tmuxListRun = old }()
	tmuxListRun = func(_ context.Context, _ ...string) ([]byte, error) { return nil, nil }

	var rep OrphanReapReport = ExecReapOrphans(context.Background())
	if len(rep.Killed) != 0 || len(rep.Errors) != 0 {
		t.Fatalf("clean server must be a no-op, got %+v", rep)
	}
}

// TestExecListBridgeSockets_NamesOnly drives the production socket lister through
// the socketGlob seam (never touches the real tmux socket dir).
func TestExecListBridgeSockets_NamesOnly(t *testing.T) {
	old := socketGlob
	defer func() { socketGlob = old }()
	socketGlob = func() ([]string, error) { return []string{"evolve-bridge-p7"}, nil }
	got, err := ExecListBridgeSockets()
	if err != nil || len(got) != 1 || got[0] != "evolve-bridge-p7" {
		t.Fatalf("ExecListBridgeSockets = (%v, %v), want ([evolve-bridge-p7], nil)", got, err)
	}
}

// TestExecReapOrphanSockets_NoSocketsNoop names+executes the production socket-GC
// wiring against an empty host — a harmless no-op.
func TestExecReapOrphanSockets_NoSocketsNoop(t *testing.T) {
	old := socketGlob
	defer func() { socketGlob = old }()
	socketGlob = func() ([]string, error) { return nil, nil }
	var rep OrphanSocketReport = ExecReapOrphanSockets(context.Background())
	if len(rep.Killed) != 0 || len(rep.Errors) != 0 {
		t.Fatalf("no sockets must be a no-op, got %+v", rep)
	}
}

// TestExecKillServer_RefusesEmpty: never aim kill-server at an empty socket name.
func TestExecKillServer_RefusesEmpty(t *testing.T) {
	t.Parallel()
	if ExecKillServer(context.Background(), "") == nil {
		t.Fatal("ExecKillServer must refuse an empty socket name")
	}
}
