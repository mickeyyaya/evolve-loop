package failurelog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func brWriteState(t *testing.T, dir, body string) string {
	t.Helper()
	p := filepath.Join(dir, "state.json")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func brReadState(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestPruneExpiredCarryoverTodos_RemovesExpiredKeepsFresh(t *testing.T) {
	now := time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC)
	past := now.Add(-24 * time.Hour).Format(time.RFC3339)
	future := now.Add(24 * time.Hour).Format(time.RFC3339)
	statePath := brWriteState(t, t.TempDir(), `{"carryoverTodos":[`+
		`{"id":"cycle-400-failed-scout","expiresAt":"`+past+`"},`+
		`{"id":"cycle-505-failed-changelog-sync","expiresAt":"`+future+`"}]}`)

	pr, err := PruneExpiredCarryoverTodos(statePath, now)
	if err != nil {
		t.Fatalf("PruneExpiredCarryoverTodos: %v", err)
	}
	if pr.Before != 2 || pr.After != 1 || pr.Removed != 1 {
		t.Fatalf("expected before=2 after=1 removed=1; got %+v", pr)
	}
	out := brReadState(t, statePath)
	if !strings.Contains(out, "cycle-505-failed-changelog-sync") {
		t.Error("a still-fresh carryover todo must survive the prune")
	}
	if strings.Contains(out, "cycle-400-failed-scout") {
		t.Error("an expired carryover todo must be removed from state.json on disk (not merely reported)")
	}
}

func TestPruneExpiredCarryoverTodos_KeepsUntimestampedLegacy(t *testing.T) {
	now := time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC)
	statePath := brWriteState(t, t.TempDir(),
		`{"carryoverTodos":[{"id":"cycle-366-failed-ship","action":"legacy, no ttl"}]}`)

	pr, err := PruneExpiredCarryoverTodos(statePath, now)
	if err != nil {
		t.Fatalf("PruneExpiredCarryoverTodos: %v", err)
	}
	if pr.Removed != 0 || pr.After != 1 {
		t.Fatalf("untimestamped legacy entry must be kept; got %+v", pr)
	}
	if !strings.Contains(brReadState(t, statePath), "cycle-366-failed-ship") {
		t.Error("untimestamped legacy carryover todo must never be auto-deleted")
	}
}

func TestPruneExpiredCarryoverTodos_MissingOrEmptyIsSafeNoOp(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist.json")
	if pr, err := PruneExpiredCarryoverTodos(missing, time.Now().UTC()); err != nil || pr != (PruneResult{}) {
		t.Fatalf("missing state must be a safe no-op; got pr=%+v err=%v", pr, err)
	}

	empty := brWriteState(t, t.TempDir(), `{"failedApproaches":[]}`)
	if pr, err := PruneExpiredCarryoverTodos(empty, time.Now().UTC()); err != nil || pr.Removed != 0 {
		t.Fatalf("state with no carryoverTodos must be a no-op; got pr=%+v err=%v", pr, err)
	}
}
