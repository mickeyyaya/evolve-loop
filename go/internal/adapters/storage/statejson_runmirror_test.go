package storage

// CB.4 (concurrency campaign, run-scoped guard state): WriteCycleState
// dual-writes the cycle state to <WorkspacePath>/run.json — the per-run
// guard-read surface. The worktree provisioner symlinks the worktree's
// .evolve/cycle-state.json at that run.json, so guards running inside a
// cycle worktree decide on the run's OWN phase even when the host-global
// cycle-state.json was last written by a different concurrent run.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestWriteCycleState_MirrorsRunJSON(t *testing.T) {
	dir := t.TempDir()
	ws := filepath.Join(dir, ".evolve", "runs", "cycle-7")
	s := New(filepath.Join(dir, ".evolve"))
	cs := core.CycleState{
		CycleID:        7,
		Phase:          "build",
		WorkspacePath:  ws,
		ActiveWorktree: filepath.Join(dir, "wt"),
		RunID:          "01JTESTRUNID0000000000000A",
	}
	if err := s.WriteCycleState(context.Background(), cs); err != nil {
		t.Fatalf("WriteCycleState: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(ws, core.RunStateFile))
	if err != nil {
		t.Fatalf("run.json not mirrored into workspace: %v", err)
	}
	var got core.CycleState
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("run.json unmarshal: %v", err)
	}
	if !reflect.DeepEqual(got, cs) {
		t.Errorf("run.json round-trip mismatch:\n got %+v\nwant %+v", got, cs)
	}
}

func TestWriteCycleState_EmptyWorkspace_NoMirror(t *testing.T) {
	dir := t.TempDir()
	s := New(filepath.Join(dir, ".evolve"))
	cs := core.CycleState{CycleID: 3, Phase: "scout"} // WorkspacePath unset
	if err := s.WriteCycleState(context.Background(), cs); err != nil {
		t.Fatalf("WriteCycleState: %v", err)
	}
	// No run.json may appear anywhere under the evolve dir.
	err := filepath.Walk(dir, func(path string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if filepath.Base(path) == core.RunStateFile {
			t.Errorf("unexpected %s at %s with empty WorkspacePath", core.RunStateFile, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// The checkpoint block is resume state owned by the GLOBAL cycle-state.json
// (spliced in by internal/checkpoint); guards never read it, and resume reads
// the global file directly — so the per-run mirror stays a plain CycleState.
func TestWriteCycleState_MirrorOmitsCheckpoint(t *testing.T) {
	dir := t.TempDir()
	evolveDir := filepath.Join(dir, ".evolve")
	ws := filepath.Join(evolveDir, "runs", "cycle-9")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	seed := `{"cycle_id":9,"phase":"scout","checkpoint":{"reason":"phase-complete"}}`
	if err := os.WriteFile(filepath.Join(evolveDir, "cycle-state.json"), []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}

	s := New(evolveDir)
	cs := core.CycleState{CycleID: 9, Phase: "build", WorkspacePath: ws}
	if err := s.WriteCycleState(context.Background(), cs); err != nil {
		t.Fatalf("WriteCycleState: %v", err)
	}

	// Global keeps the checkpoint block (existing contract).
	globalRaw, err := os.ReadFile(filepath.Join(evolveDir, "cycle-state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var globalMap map[string]json.RawMessage
	if err := json.Unmarshal(globalRaw, &globalMap); err != nil {
		t.Fatal(err)
	}
	if _, ok := globalMap["checkpoint"]; !ok {
		t.Error("global cycle-state.json lost its checkpoint block")
	}

	// Mirror must not carry it.
	runRaw, err := os.ReadFile(filepath.Join(ws, core.RunStateFile))
	if err != nil {
		t.Fatalf("run.json not mirrored: %v", err)
	}
	var runMap map[string]json.RawMessage
	if err := json.Unmarshal(runRaw, &runMap); err != nil {
		t.Fatal(err)
	}
	if _, ok := runMap["checkpoint"]; ok {
		t.Error("run.json must not carry the checkpoint block (resume owns it via the global file)")
	}
}

// A failed mirror means guards inside the worktree would decide on a STALE
// phase — that must surface as an error, not degrade silently.
func TestWriteCycleState_MirrorFailureSurfaces(t *testing.T) {
	dir := t.TempDir()
	// Make the workspace path un-creatable: its parent is a regular file.
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := New(filepath.Join(dir, ".evolve"))
	cs := core.CycleState{CycleID: 5, Phase: "build", WorkspacePath: filepath.Join(blocker, "cycle-5")}
	err := s.WriteCycleState(context.Background(), cs)
	if err == nil {
		t.Fatal("WriteCycleState must fail loudly when the run.json mirror cannot be written")
	}
	if !strings.Contains(err.Error(), core.RunStateFile) {
		t.Errorf("error should name the run-state mirror; got: %v", err)
	}
}
