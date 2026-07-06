package main

// cmd_swarm_test.go — RED contract for cycle-549's
// cli-command-layer-test-coverage task (see cmd_worktree_test.go's package
// doc comment for the full task/lane background). `evolve swarm
// status|reap|reap-orphans` (cmd_swarm.go) had ZERO direct test coverage
// (0.0% on every handler per `go tool cover -func`).
//
// SAFETY: runSwarmReap's production killer sends a REAL syscall.Kill(-pgid,
// SIGKILL) when a session's PGID>1, and a real tmux kill-session when
// TmuxSession != "" (see internal/swarm/kill.go's 2026-06-11 killer-B
// incident doc). Fixtures here use PGID=0 and an empty TmuxSession — both
// gates in ExecSessionKiller.Kill (h.PGID > 1 / h.TmuxSession != "") skip the
// dangerous calls entirely, so the reap path is exercised end to end (manifest
// load, registry rebuild, Reap dispatch, output) without ever touching a real
// process group or tmux server. reap-orphans tests additionally pass
// --dry-run, which the CLI itself stubs to a no-op Kill regardless of fixture
// content.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/swarm"
)

// writeEmptyManifest writes a valid, session-less swarm manifest at path
// (SessionRegistry has no exported Persist(); mirroring the on-disk shape
// LoadManifest reads is simplest and keeps this file decoupled from that
// package's internals).
func writeEmptyManifest(t *testing.T, path string, cycle int, phase string, pid int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := fmt.Sprintf(`{"cycle":%d,"phase":%q,"pid":%d,"sessions":[]}`, cycle, phase, pid)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}

func TestManifestPath(t *testing.T) {
	got := manifestPath(".evolve", 42)
	want := filepath.Join(".evolve", "runs", "cycle-42", ".swarm", "sessions.json")
	if got != want {
		t.Errorf("manifestPath = %q, want %q", got, want)
	}
}

func TestRunSwarmStatus_MissingCycle_ExitTwo(t *testing.T) {
	var stdout, stderr strings.Builder
	code := runSwarmStatus([]string{"--evolve-dir", t.TempDir()}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit = %d, want 2 (missing --cycle); stderr=%s", code, stderr.String())
	}
}

// TestRunSwarmStatus_ManifestMissing_ExitZero: LoadManifest is documented
// fail-open on a missing file (internal/swarm/registry.go: "not an error — it
// returns an empty manifest so the reaper is a safe no-op") — a cycle with no
// swarm dispatch at all must report cleanly, not error.
func TestRunSwarmStatus_ManifestMissing_ExitZero(t *testing.T) {
	var stdout, stderr strings.Builder
	code := runSwarmStatus([]string{"--evolve-dir", t.TempDir(), "--cycle", "1"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (missing manifest is fail-open); stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "no swarm sessions recorded") {
		t.Errorf("stdout = %q, want the no-sessions message", stdout.String())
	}
}

func TestRunSwarmStatus_NoSessions_ExitZero(t *testing.T) {
	evolveDir := t.TempDir()
	writeEmptyManifest(t, manifestPath(evolveDir, 5), 5, "build", 12345)

	var stdout, stderr strings.Builder
	code := runSwarmStatus([]string{"--evolve-dir", evolveDir, "--cycle", "5"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "no swarm sessions recorded") {
		t.Errorf("stdout = %q, want the no-sessions message", stdout.String())
	}
}

func TestRunSwarmStatus_WithSession_ListsIt(t *testing.T) {
	evolveDir := t.TempDir()
	reg := swarm.NewSessionRegistry(manifestPath(evolveDir, 7), 7, "build", 999)
	if err := reg.Register(swarm.SessionHandle{
		WorkerID: "w1", Agent: "build-w1", TmuxSession: "", PGID: 0,
		Branch: "cycle-7-w1", Status: swarm.StatusLive,
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	var stdout, stderr strings.Builder
	code := runSwarmStatus([]string{"--evolve-dir", evolveDir, "--cycle", "7"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "w1") || !strings.Contains(stdout.String(), "cycle-7-w1") {
		t.Errorf("stdout = %q, want it to list worker w1 / branch cycle-7-w1", stdout.String())
	}
}

func TestRunSwarmReap_MissingCycle_ExitTwo(t *testing.T) {
	var stdout, stderr strings.Builder
	code := runSwarmReap([]string{"--evolve-dir", t.TempDir()}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit = %d, want 2 (missing --cycle)", code)
	}
}

// TestRunSwarmReap_ManifestMissing_ExitZero mirrors LoadManifest's documented
// fail-open contract on a missing manifest file (see status's equivalent test).
func TestRunSwarmReap_ManifestMissing_ExitZero(t *testing.T) {
	var stdout, stderr strings.Builder
	code := runSwarmReap([]string{"--evolve-dir", t.TempDir(), "--cycle", "3"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (missing manifest is fail-open); stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "no swarm sessions to reap") {
		t.Errorf("stdout = %q, want the no-sessions message", stdout.String())
	}
}

func TestRunSwarmReap_NoSessions_ExitZero(t *testing.T) {
	evolveDir := t.TempDir()
	writeEmptyManifest(t, manifestPath(evolveDir, 9), 9, "build", 1)
	var stdout, stderr strings.Builder
	code := runSwarmReap([]string{"--evolve-dir", evolveDir, "--cycle", "9"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "no swarm sessions to reap") {
		t.Errorf("stdout = %q, want the no-sessions message", stdout.String())
	}
}

// TestRunSwarmReap_LiveSession_ReapsWithoutTouchingOS is the SAFE reap path:
// PGID=0 and an empty TmuxSession mean ExecSessionKiller.Kill's two gates
// (h.PGID > 1 / h.TmuxSession != "") both skip — no real signal or tmux call
// fires — yet the session is still marked reaped and counted, proving the
// manifest-load → registry-rebuild → Reap → report wiring end to end.
func TestRunSwarmReap_LiveSession_ReapsWithoutTouchingOS(t *testing.T) {
	evolveDir := t.TempDir()
	path := manifestPath(evolveDir, 11)
	reg := swarm.NewSessionRegistry(path, 11, "build", 1)
	if err := reg.Register(swarm.SessionHandle{
		WorkerID: "w1", Agent: "build-w1", TmuxSession: "", PGID: 0, Status: swarm.StatusLive,
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	var stdout, stderr strings.Builder
	code := runSwarmReap([]string{"--evolve-dir", evolveDir, "--cycle", "11"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "reaped 1 session") {
		t.Errorf("stdout = %q, want it to report reaping 1 session", stdout.String())
	}
	if stderr.String() != "" {
		t.Errorf("stderr = %q, want empty (no kill errors for a PGID=0/no-tmux session)", stderr.String())
	}
}

func TestRunSwarmReapOrphans_NoRunsDir_ExitZero(t *testing.T) {
	evolveDir := t.TempDir() // no runs/ subdir at all
	var stdout, stderr strings.Builder
	code := runSwarmReapOrphans([]string{"--evolve-dir", evolveDir, "--dry-run"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "orphan runs=0") {
		t.Errorf("stdout = %q, want orphan runs=0", stdout.String())
	}
}

func TestRunSwarmReapOrphans_UnknownFlag_ExitTwo(t *testing.T) {
	var stdout, stderr strings.Builder
	code := runSwarmReapOrphans([]string{"--not-a-real-flag"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit = %d, want 2 (flag parse error)", code)
	}
}

func TestRunSwarm_NoSubcommand_ExitTwo(t *testing.T) {
	var stdout, stderr strings.Builder
	code := runSwarm(nil, nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit = %d, want 2 (no subcommand)", code)
	}
}

func TestRunSwarm_UnknownSubcommand_ExitTwo(t *testing.T) {
	var stdout, stderr strings.Builder
	code := runSwarm([]string{"bogus"}, nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit = %d, want 2 (unknown subcommand)", code)
	}
}

func TestRunSwarm_DispatchesStatus(t *testing.T) {
	evolveDir := t.TempDir()
	writeEmptyManifest(t, manifestPath(evolveDir, 1), 1, "build", 1)
	var stdout, stderr strings.Builder
	code := runSwarm([]string{"status", "--evolve-dir", evolveDir, "--cycle", "1"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, want 0; stderr=%s", code, stderr.String())
	}
}

// sanity: manifestPath's directory must actually exist after Persist, so the
// fixtures above are exercising the real on-disk format runSwarmStatus/Reap
// read — not a shortcut.
func TestSwarmFixture_PersistsRealManifestFile(t *testing.T) {
	evolveDir := t.TempDir()
	path := manifestPath(evolveDir, 4)
	writeEmptyManifest(t, path, 4, "build", 1)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("manifest file not written at %q: %v", path, err)
	}
}
