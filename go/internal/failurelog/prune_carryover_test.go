package failurelog

// prune_carryover_test.go — RED tests (cycle 507, task
// prune-stale-carryover-todos) for the PRUNE half of the carryoverTodos TTL
// contract. Mirrors the existing PruneExpired (failedApproaches) behavior,
// applied to the structurally-parallel state.json:carryoverTodos array which
// today has no removal path at all (65 entries / 26,601 bytes, cycles 366→506).
//
// Semantics mirror PruneExpired exactly (single-sourced intent):
//   - entry.expiresAt in the past           → removed
//   - entry with NO expiresAt (legacy)      → KEPT (age unknown; never delete)
//   - missing / carryoverTodos-less state   → {0,0,0}, nil (safe no-op)
//
// References PruneExpiredCarryoverTodos, which the Builder implements beside
// PruneExpired in this package (and wires into cmd_loop.go's AutoPrune block).
// RED now (undefined symbol → failurelog test package fails to compile). Do NOT
// modify this file — implement the production seam.

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

// AC (positive): an entry whose expiresAt is in the past is removed; a
// still-fresh entry survives.
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

// AC (negative): a legacy entry with NO expiresAt must be KEPT — its age is
// unknowable, so auto-deleting it would destroy data. This is the anti-no-op
// counterpart to the positive case: a prune that just truncates the array would
// fail here.
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

// AC (edge): a missing state.json (and a state with no carryoverTodos) is a safe
// no-op — {0,0,0}, nil — never an error that would abort loop start.
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
