package guards

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func setupStorageWithCS(t *testing.T, cs core.CycleState) (*storage.FilesystemStorage, string) {
	t.Helper()
	dir := t.TempDir()
	evolveDir := filepath.Join(dir, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	s := storage.New(evolveDir)
	if err := s.WriteCycleState(context.Background(), cs); err != nil {
		t.Fatal(err)
	}
	return s, evolveDir
}

func setupStorageNoCS(t *testing.T) (*storage.FilesystemStorage, string) {
	t.Helper()
	dir := t.TempDir()
	evolveDir := filepath.Join(dir, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return storage.New(evolveDir), evolveDir
}

// Phase is the port of scripts/guards/phase-gate-precondition.sh.
// Phase-1 rule subset:
//   - When cycle-state.json reports an active cycle, the Agent tool
//     is denied (in-process Agent calls bypass subagent-run.sh's
//     audit-binding contract).
//   - Outside an active cycle, Agent passes through.
//   - EVOLVE_BYPASS_PHASE_GATE=1 bypasses.
func TestPhase_Name(t *testing.T) {
	g := NewPhase(nil)
	if g.Name() != "phase" {
		t.Errorf("name=%q", g.Name())
	}
}

func TestPhase_AgentDuringCycleDenied(t *testing.T) {
	s, _ := setupStorageWithCS(t, core.CycleState{CycleID: 42, Phase: "build", ActiveAgent: "builder"})
	g := NewPhase(s)
	dec := g.Decide(context.Background(), core.GuardInput{ToolName: "Agent"})
	if dec.Allow {
		t.Error("Agent during cycle must deny")
	}
}

func TestPhase_AgentOutsideCycleAllowed(t *testing.T) {
	s, _ := setupStorageNoCS(t)
	g := NewPhase(s)
	dec := g.Decide(context.Background(), core.GuardInput{ToolName: "Agent"})
	if !dec.Allow {
		t.Errorf("Agent outside cycle must allow, got: %s", dec.Reason)
	}
}

func TestPhase_BypassEnvAllowsAgent(t *testing.T) {
	t.Setenv("EVOLVE_BYPASS_PHASE_GATE", "1")
	s, _ := setupStorageWithCS(t, core.CycleState{CycleID: 42, Phase: "build"})
	g := NewPhase(s)
	dec := g.Decide(context.Background(), core.GuardInput{ToolName: "Agent"})
	if !dec.Allow {
		t.Errorf("bypass must allow Agent: %s", dec.Reason)
	}
}

func TestPhase_NonAgentToolsPass(t *testing.T) {
	s, _ := setupStorageWithCS(t, core.CycleState{CycleID: 1, Phase: "build"})
	g := NewPhase(s)
	for _, tool := range []string{"Bash", "Edit", "Write", "Read"} {
		dec := g.Decide(context.Background(), core.GuardInput{ToolName: tool})
		if !dec.Allow {
			t.Errorf("tool=%s denied: %s", tool, dec.Reason)
		}
	}
}

// erroringStorage wraps a real storage but injects ReadCycleState errors.
type erroringStorage struct{ core.Storage }

func (e erroringStorage) ReadCycleState(_ context.Context) (core.CycleState, error) {
	return core.CycleState{}, errors.New("forced read fail")
}

func TestPhase_ReadCycleStateErrorDenies(t *testing.T) {
	s, _ := setupStorageNoCS(t)
	g := NewPhase(erroringStorage{s})
	dec := g.Decide(context.Background(), core.GuardInput{ToolName: "Agent"})
	if dec.Allow {
		t.Error("read error must deny by default")
	}
}

func TestPhase_NilStorageDenies(t *testing.T) {
	g := NewPhase(nil)
	dec := g.Decide(context.Background(), core.GuardInput{ToolName: "Agent"})
	if dec.Allow {
		t.Error("nil storage must deny by default")
	}
}
