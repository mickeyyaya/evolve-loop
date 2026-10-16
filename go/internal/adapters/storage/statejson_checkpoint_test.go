package storage

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func writeCheckpointedState(t *testing.T, evolveDir string, cp map[string]any) string {
	t.Helper()
	path := filepath.Join(evolveDir, "cycle-state.json")
	state := map[string]any{
		"cycle_id":   294,
		"phase":      "mutation-gate",
		"checkpoint": cp,
	}
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatalf("marshal seed state: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write seed state: %v", err)
	}
	return path
}

func readStateMap(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal state: %v", err)
	}
	return m
}

func TestWriteCycleState_PreservesCheckpoint(t *testing.T) {
	s, evolveDir := newStore(t)
	cp := map[string]any{
		"resume_from_phase": "tdd",
		"attempts":          float64(1),
		"phase_completed":   "tdd",
	}
	path := writeCheckpointedState(t, evolveDir, cp)

	next := core.CycleState{CycleID: 294, Phase: "audit", WorkspacePath: t.TempDir()}
	if err := s.WriteCycleState(context.Background(), next); err != nil {
		t.Fatalf("WriteCycleState: %v", err)
	}

	got := readStateMap(t, path)
	if got["phase"] != "audit" {
		t.Errorf("phase = %v, want audit (the new cycle state was not written)", got["phase"])
	}
	rawCP, ok := got["checkpoint"].(map[string]any)
	if !ok {
		t.Fatalf("RED: \"checkpoint\" block erased by WriteCycleState (got %T) — resume is impossible after a crash", got["checkpoint"])
	}
	for k, want := range cp {
		if rawCP[k] != want {
			t.Errorf("RED: checkpoint[%q] = %v, want %v (block was mutated, not preserved)", k, rawCP[k], want)
		}
	}
}

func TestWriteCycleState_NoCheckpointWhenNonePrior(t *testing.T) {
	s, evolveDir := newStore(t)
	if err := s.WriteCycleState(context.Background(), core.CycleState{CycleID: 295, Phase: "scout"}); err != nil {
		t.Fatalf("WriteCycleState: %v", err)
	}
	got := readStateMap(t, filepath.Join(evolveDir, "cycle-state.json"))
	if v, present := got["checkpoint"]; present {
		t.Errorf("no prior checkpoint, yet WriteCycleState wrote \"checkpoint\": %v — must omit the key entirely", v)
	}
}

func TestWriteCycleState_CheckpointNotDuplicated(t *testing.T) {
	s, evolveDir := newStore(t)
	path := writeCheckpointedState(t, evolveDir, map[string]any{"resume_from_phase": "build"})

	for i := 0; i < 2; i++ {
		if err := s.WriteCycleState(context.Background(), core.CycleState{CycleID: 294, Phase: "audit"}); err != nil {
			t.Fatalf("WriteCycleState #%d: %v", i, err)
		}
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	if n := strings.Count(string(raw), `"checkpoint"`); n != 1 {
		t.Errorf("RED: \"checkpoint\" key count = %d after two writes, want exactly 1 (0 = erased, >1 = duplicated)", n)
	}
}

func TestWriteCycleState_PreservesWorktreeBaseSHAWithCheckpoint(t *testing.T) {
	s, evolveDir := newStore(t)
	cp := map[string]any{"resume_from_phase": "build"}
	path := writeCheckpointedState(t, evolveDir, cp)

	next := core.CycleState{
		CycleID:         294,
		Phase:           "audit",
		WorktreeBaseSHA: "abc123def4567890abc123def4567890abc123de",
	}
	if err := s.WriteCycleState(context.Background(), next); err != nil {
		t.Fatalf("WriteCycleState: %v", err)
	}

	got := readStateMap(t, path)
	if got["worktree_base_sha"] != next.WorktreeBaseSHA {
		t.Errorf("worktree_base_sha = %v, want %q", got["worktree_base_sha"], next.WorktreeBaseSHA)
	}
	if _, ok := got["checkpoint"].(map[string]any); !ok {
		t.Errorf("checkpoint block lost when writing a state with WorktreeBaseSHA")
	}

	rt, err := s.ReadCycleState(context.Background())
	if err != nil {
		t.Fatalf("ReadCycleState: %v", err)
	}
	if rt.WorktreeBaseSHA != next.WorktreeBaseSHA {
		t.Errorf("ReadCycleState WorktreeBaseSHA = %q, want %q — the resume path depends on this round-trip", rt.WorktreeBaseSHA, next.WorktreeBaseSHA)
	}
}
