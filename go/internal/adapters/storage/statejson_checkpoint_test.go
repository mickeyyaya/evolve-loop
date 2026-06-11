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

// statejson_checkpoint_test.go — RED contract for cycle-295 task
// checkpoint-clobber-fix.
//
// Live incident (host reboot during cycle-294 mutation-gate): every
// pre-dispatch WriteCycleState does a whole-struct json replace, and
// core.CycleState has NO "checkpoint" field, so each write ERASES the
// "checkpoint" block that PhaseBoundaryCheckpointer/ApplyToStateFile
// wrote after the prior phase. Net: a crash mid-phase leaves no
// checkpoint and `evolve loop --resume` fails with "no live checkpoint".
//
// Fix (builder): make WriteCycleState a read-merge-write that carries the
// existing "checkpoint" key through the rewrite (mirrors
// checkpoint.ApplyToStateFile). These tests exercise the REAL
// FilesystemStorage.WriteCycleState and assert on the REAL file bytes —
// a magic string cannot satisfy them.

// writeCheckpointedState seeds cycle-state.json with both the CycleState
// fields AND a "checkpoint" block (the shape ApplyToStateFile writes),
// returning the cycle-state.json path. The block is written by hand (not
// via the checkpoint package) to keep storage tests free of an upward
// dependency on checkpoint.
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

// readStateMap reads cycle-state.json back into a generic map so the test
// can inspect keys (incl. "checkpoint") the CycleState struct does not model.
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

// TestWriteCycleState_PreservesCheckpoint: a prior "checkpoint" block must
// survive a subsequent WriteCycleState, byte-for-byte. RED today —
// WriteCycleState whole-struct-replaces and drops the key entirely.
func TestWriteCycleState_PreservesCheckpoint(t *testing.T) {
	s, evolveDir := newStore(t)
	cp := map[string]any{
		"resume_from_phase": "tdd",
		"attempts":          float64(1), // json numbers decode as float64
		"phase_completed":   "tdd",
	}
	path := writeCheckpointedState(t, evolveDir, cp)

	// A new pre-dispatch write for the next phase.
	next := core.CycleState{CycleID: 294, Phase: "audit", WorkspacePath: "/ws"}
	if err := s.WriteCycleState(context.Background(), next); err != nil {
		t.Fatalf("WriteCycleState: %v", err)
	}

	got := readStateMap(t, path)
	// The new CycleState fields must be present (the write still happened)...
	if got["phase"] != "audit" {
		t.Errorf("phase = %v, want audit (the new cycle state was not written)", got["phase"])
	}
	// ...AND the checkpoint block must be carried through untouched.
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

// TestWriteCycleState_NoCheckpointWhenNonePrior: a fresh write with no
// prior checkpoint must NOT invent a "checkpoint" key (no spurious
// "checkpoint": null). Guards the fix against over-adding the key.
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

// TestWriteCycleState_CheckpointNotDuplicated: repeated writes must leave
// exactly ONE "checkpoint" key (no accidental duplication via string
// splicing, and the merge must remain idempotent). RED today — the key is
// dropped, so the count is 0, not 1.
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

// TestWriteCycleState_PreservesWorktreeBaseSHAWithCheckpoint: the cycle-156
// resume-parity fix persists the cycle base in CycleState.WorktreeBaseSHA.
// It must round-trip through WriteCycleState/ReadCycleState AND coexist with
// the checkpoint-block preservation splice (cycle-295 fix) — proving the two
// mechanisms don't interact.
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
