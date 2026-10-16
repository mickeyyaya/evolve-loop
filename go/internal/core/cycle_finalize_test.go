package core

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

const finalizeFixedNow = "2026-07-06T12:00:00Z"

func mustParseFinalizeNow(t *testing.T) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, finalizeFixedNow)
	if err != nil {
		t.Fatalf("parse fixed now: %v", err)
	}
	return ts
}

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

func TestClearCompletedCycleMarker_RemovesTerminalMarker(t *testing.T) {
	t.Parallel()

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

func TestClearCompletedCycleMarker_PreservesUnfinishedMarker(t *testing.T) {
	t.Parallel()

	evolveDir := t.TempDir()
	csPath, _, _ := writeFinalizeFixture(t, evolveDir, 6, 5)
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

func TestClearCompletedCycleMarker_LiveOwnerUntouched(t *testing.T) {
	t.Parallel()

	evolveDir := t.TempDir()
	csPath, _, workspace := writeFinalizeFixture(t, evolveDir, 5, 5)

	now := mustParseFinalizeNow(t)
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
