package swarm

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/sessionrecord"
)

func writeRegistry(t *testing.T, dir string, sessions ...string) string {
	t.Helper()
	for _, s := range sessions {
		if err := sessionrecord.Append(sessionrecord.PathIn(dir), sessionrecord.Record{
			Session: s, RunID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", Agent: "build",
		}); err != nil {
			t.Fatalf("append: %v", err)
		}
	}
	return sessionrecord.PathIn(dir)
}

func TestReapRunSessions_KillsOwnRegistryOnly(t *testing.T) {
	t.Parallel()
	pathA := writeRegistry(t, t.TempDir(), "evolve-bridge-rAAAA0000-c1-build-pid1-1", "evolve-bridge-rAAAA0000-c1-audit-pid1-2")
	_ = writeRegistry(t, t.TempDir(), "evolve-bridge-rBBBB1111-c2-build-pid2-1")

	var killed []string
	kill := func(_ context.Context, session string) error {
		killed = append(killed, session)
		return nil
	}
	report := ReapRunSessions(context.Background(), pathA, kill)
	if len(killed) != 2 || report.Killed != 2 {
		t.Fatalf("killed=%v report=%+v, want exactly run A's 2 sessions", killed, report)
	}
	for _, s := range killed {
		if s == "evolve-bridge-rBBBB1111-c2-build-pid2-1" {
			t.Fatal("run A teardown killed run B's session — the isolation contract is broken")
		}
	}
}

func TestReapRunSessions_RefusesUnsafeNames(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := sessionrecord.PathIn(dir)
	for _, s := range []string{"", "main", "evolve-bridge-rCCCC2222-c3-tdd-pid3-1"} {
		if err := sessionrecord.Append(path, sessionrecord.Record{Session: s}); err != nil {
			t.Fatalf("append: %v", err)
		}
	}
	var killed []string
	report := ReapRunSessions(context.Background(), path, func(_ context.Context, s string) error {
		killed = append(killed, s)
		return nil
	})
	if len(killed) != 1 || killed[0] != "evolve-bridge-rCCCC2222-c3-tdd-pid3-1" {
		t.Fatalf("killed=%v, want only the evolve-bridge-namespaced session", killed)
	}
	if report.Skipped != 2 {
		t.Errorf("report.Skipped=%d, want 2 (empty + foreign name)", report.Skipped)
	}
}

func TestReapRunSessions_MissingRegistryIsNoop(t *testing.T) {
	t.Parallel()
	report := ReapRunSessions(context.Background(), filepath.Join(t.TempDir(), "absent.jsonl"), func(_ context.Context, s string) error {
		t.Fatalf("killer called (%s) for a missing registry", s)
		return nil
	})
	if report.Killed != 0 || report.Skipped != 0 {
		t.Errorf("report=%+v, want zero activity", report)
	}
}
