package core

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

// cycle_finalize_test.go — RED tests for S2 (workspace-hygiene-2026-07 plan):
// ClearCompletedCycleMarker is the clean-exit counterpart to SealCycle. Unlike
// SealCycle (abandon semantics: archive + faillearn lesson + ledger entry),
// a clean batch-end marker clear must be SILENT — no `.reset-*` archive, no
// lesson, no state.json mutation — or every healthy `max_cycles` exit would
// falsely poison failure-learning (the plan's stated root-cause concern).
//
// ClearCompletedCycleMarker(evolveDir, FinalizeOptions{Now, LeaseTTL, PidAlive})
// (bool, error) does not exist yet — this file, plus
// cmd/evolve/cmd_loop_finalize_test.go, is the RED contract Builder implements
// against.

const finalizeFixedNow = "2026-07-06T12:00:00Z"

func mustParseFinalizeNow(t *testing.T) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, finalizeFixedNow)
	if err != nil {
		t.Fatalf("parse fixed now: %v", err)
	}
	return ts
}

// writeFinalizeFixture lays out a minimal evolveDir with cycle-state.json
// (cycleID/workspace) and state.json (lastCycleNumber), mirroring the field
// names core.CycleState / reset.go's raw-map use (cycle_id, workspace_path,
// lastCycleNumber).
func writeFinalizeFixture(t *testing.T, evolveDir string, cycleID, lastCycleNumber int) (csPath, statePath, workspace string) {
	t.Helper()
	workspace = filepath.Join(evolveDir, "runs", "cycle-finalize-fixture")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	csPath = filepath.Join(evolveDir, CycleStateFile)
	csJSON := `{"cycle_id":` + strconv.Itoa(cycleID) + `,"phase":"ship","workspace_path":"` + filepath.ToSlash(workspace) + `"}`
	if err := os.WriteFile(csPath, []byte(csJSON), 0o644); err != nil {
		t.Fatalf("write cycle-state.json: %v", err)
	}
	statePath = filepath.Join(evolveDir, "state.json")
	stateJSON := `{"lastUpdated":"2026-07-06T11:00:00Z","lastCycleNumber":` + strconv.Itoa(lastCycleNumber) + `,"currentBatch":{"cycleAccruedCostUSD":0}}`
	if err := os.WriteFile(statePath, []byte(stateJSON), 0o644); err != nil {
		t.Fatalf("write state.json: %v", err)
	}
	return csPath, statePath, workspace
}

// TestClearCompletedCycleMarker_RemovesTerminalMarker: a completed cycle
// (cycle_id <= lastCycleNumber) with no live owner must have its marker
// removed — AND must leave state.json byte-identical and write neither a
// `.reset-*` archive nor a faillearn lesson (the ≠SealCycle pin).
func TestClearCompletedCycleMarker_RemovesTerminalMarker(t *testing.T) {
	evolveDir := t.TempDir()
	csPath, statePath, _ := writeFinalizeFixture(t, evolveDir, 5, 5)
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state.json before: %v", err)
	}

	now := mustParseFinalizeNow(t)
	cleared, err := ClearCompletedCycleMarker(evolveDir, FinalizeOptions{
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("ClearCompletedCycleMarker: %v", err)
	}
	if !cleared {
		t.Fatalf("cleared = false, want true for a completed (cycle_id<=lastCycleNumber), unowned cycle")
	}
	if _, statErr := os.Stat(csPath); !os.IsNotExist(statErr) {
		t.Errorf("cycle-state.json still present after clear (stat err=%v)", statErr)
	}

	after, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state.json after: %v", err)
	}
	if string(after) != string(before) {
		t.Errorf("state.json mutated by ClearCompletedCycleMarker:\nbefore=%s\nafter=%s", before, after)
	}

	matches, _ := filepath.Glob(filepath.Join(evolveDir, "runs", "*.reset-*"))
	if len(matches) != 0 {
		t.Errorf("unexpected .reset-* archive(s) created: %v (ClearCompletedCycleMarker must never archive — that is SealCycle's job)", matches)
	}
	if fi, statErr := os.Stat(filepath.Join(evolveDir, "instincts", "lessons")); statErr == nil {
		entries, _ := os.ReadDir(filepath.Join(evolveDir, "instincts", "lessons"))
		if fi.IsDir() && len(entries) != 0 {
			t.Errorf("faillearn lesson(s) written: %v (ClearCompletedCycleMarker must never write to failure-learning)", entries)
		}
	}
}

// TestClearCompletedCycleMarker_PreservesUnfinishedMarker: cycle_id greater
// than lastCycleNumber means the cycle never advanced the counter — it is
// still in progress (or crashed) — and must be left alone so --resume /
// quota-pause / a subsequent `evolve cycle reset` can still see it.
func TestClearCompletedCycleMarker_PreservesUnfinishedMarker(t *testing.T) {
	evolveDir := t.TempDir()
	csPath, _, _ := writeFinalizeFixture(t, evolveDir, 6, 5) // 6 > 5 → unfinished
	before, err := os.ReadFile(csPath)
	if err != nil {
		t.Fatalf("read cycle-state.json before: %v", err)
	}

	now := mustParseFinalizeNow(t)
	cleared, err := ClearCompletedCycleMarker(evolveDir, FinalizeOptions{
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("ClearCompletedCycleMarker: %v", err)
	}
	if cleared {
		t.Fatalf("cleared = true, want false — cycle_id(6) > lastCycleNumber(5) is unfinished and must be preserved")
	}
	after, err := os.ReadFile(csPath)
	if err != nil {
		t.Fatalf("cycle-state.json removed for an unfinished cycle — must be preserved: %v", err)
	}
	if string(after) != string(before) {
		t.Errorf("cycle-state.json content changed even though preserved:\nbefore=%s\nafter=%s", before, after)
	}
}

// TestClearCompletedCycleMarker_LiveOwnerUntouched: even a "completed"-looking
// marker (cycle_id<=lastCycleNumber) must NOT be cleared while its lease shows
// a live owner (runlease.OwnerLive) — clearing out from under a running loop
// would let a second `evolve loop` start concurrently on the same state.
func TestClearCompletedCycleMarker_LiveOwnerUntouched(t *testing.T) {
	evolveDir := t.TempDir()
	csPath, _, workspace := writeFinalizeFixture(t, evolveDir, 5, 5)

	now := mustParseFinalizeNow(t)
	// Fresh lease, OwnerPID=0 (no pid tracked) — runlease.OwnerLive falls back
	// to freshness-only for such a lease, so this alone must block the clear.
	if err := runlease.Write(workspace, runlease.Lease{}, now); err != nil {
		t.Fatalf("write lease: %v", err)
	}
	before, err := os.ReadFile(csPath)
	if err != nil {
		t.Fatalf("read cycle-state.json before: %v", err)
	}

	cleared, err := ClearCompletedCycleMarker(evolveDir, FinalizeOptions{
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("ClearCompletedCycleMarker: %v", err)
	}
	if cleared {
		t.Fatalf("cleared = true, want false — the lease shows a live owner (fresh heartbeat), must not clear under it")
	}
	after, err := os.ReadFile(csPath)
	if err != nil {
		t.Fatalf("cycle-state.json removed despite a live owner: %v", err)
	}
	if string(after) != string(before) {
		t.Errorf("cycle-state.json content changed even though preserved:\nbefore=%s\nafter=%s", before, after)
	}
}
