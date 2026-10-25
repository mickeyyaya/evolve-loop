package guards

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
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

func noCellEnv(string) string { return "" }

func inCellEnv(key string) string {
	if key == "EVOLVE_PHASE" {
		return "build"
	}
	return ""
}

func personaCall(tool, persona string) core.GuardInput {
	return core.GuardInput{ToolName: tool, ToolInput: map[string]any{"subagent_type": persona}}
}

func TestPhase_Name(t *testing.T) {
	g := NewPhase(nil, false)
	if g.Name() != "phase" {
		t.Errorf("name=%q", g.Name())
	}
}

func TestPhase_APhasePersonaOutsideACellIsDeniedDuringACycleForAgentAndTask(t *testing.T) {
	s, _ := setupStorageWithCS(t, core.CycleState{CycleID: 42, Phase: "build", ActiveAgent: "builder"})
	g := newPhaseWithEnv(s, false, noCellEnv)
	for _, tool := range []string{"Agent", "Task"} {
		for _, persona := range []string{"evo:evolve-builder", "evolve-auditor", "EVO:Evolve-Scout"} {
			dec := g.Decide(context.Background(), personaCall(tool, persona))
			if dec.Allow || !strings.HasPrefix(dec.Reason, tool+" of the phase persona "+persona+" denied outside a phase cell during cycle build") {
				t.Errorf("%s %s outside a cell during a cycle must be denied with a reason that names both: %+v", tool, persona, dec)
			}
		}
	}
}

func TestPhase_AGenericSubagentOutsideACellIsAllowedDuringACycle(t *testing.T) {
	s, _ := setupStorageWithCS(t, core.CycleState{CycleID: 42, Phase: "build"})
	g := newPhaseWithEnv(s, false, noCellEnv)
	for _, persona := range []string{"", "general-purpose", "code-reviewer", "evo:code-review-simplify", "my-evolve-helper"} {
		if dec := g.Decide(context.Background(), personaCall("Task", persona)); !dec.Allow {
			t.Errorf("a generic subagent %q is not a phase agent and must pass: %s", persona, dec.Reason)
		}
	}
}

func TestPhase_InsideACellEveryAgentAndTaskIsAllowed(t *testing.T) {
	s, _ := setupStorageWithCS(t, core.CycleState{CycleID: 42, Phase: "build"})
	g := newPhaseWithEnv(s, false, inCellEnv)
	for _, tool := range []string{"Agent", "Task"} {
		if dec := g.Decide(context.Background(), personaCall(tool, "evo:evolve-builder")); !dec.Allow {
			t.Errorf("a subagent of the own harness inside a cell must pass (spec 7.2): %s %s", tool, dec.Reason)
		}
	}
}

func TestPhase_APhasePersonaOutsideACycleIsAllowed(t *testing.T) {
	s, _ := setupStorageNoCS(t)
	if dec := newPhaseWithEnv(s, false, noCellEnv).Decide(context.Background(), personaCall("Agent", "evo:evolve-scout")); !dec.Allow {
		t.Errorf("a phase persona outside a cycle must pass, got: %s", dec.Reason)
	}
}

func TestPhase_BypassAllowsAPhasePersona(t *testing.T) {
	s, _ := setupStorageWithCS(t, core.CycleState{CycleID: 42, Phase: "build"})
	if dec := newPhaseWithEnv(s, true, noCellEnv).Decide(context.Background(), personaCall("Agent", "evo:evolve-builder")); !dec.Allow {
		t.Errorf("bypass must allow: %s", dec.Reason)
	}
}

func TestPhase_NonSubagentToolsPass(t *testing.T) {
	s, _ := setupStorageWithCS(t, core.CycleState{CycleID: 1, Phase: "build"})
	g := newPhaseWithEnv(s, false, noCellEnv)
	for _, tool := range []string{"Bash", "Edit", "Write", "Read"} {
		if dec := g.Decide(context.Background(), personaCall(tool, "evo:evolve-builder")); !dec.Allow {
			t.Errorf("tool=%s denied: %s", tool, dec.Reason)
		}
	}
}

type erroringStorage struct{ core.Storage }

func (e erroringStorage) ReadCycleState(_ context.Context) (core.CycleState, error) {
	return core.CycleState{}, errors.New("forced read fail")
}

func TestPhase_ReadCycleStateErrorDenies(t *testing.T) {
	s, _ := setupStorageNoCS(t)
	if dec := newPhaseWithEnv(erroringStorage{s}, false, noCellEnv).Decide(context.Background(), personaCall("Agent", "evo:evolve-builder")); dec.Allow {
		t.Error("read error must deny by default")
	}
}

func TestPhase_NilStorageDenies(t *testing.T) {
	if dec := newPhaseWithEnv(nil, false, noCellEnv).Decide(context.Background(), personaCall("Task", "evo:evolve-builder")); dec.Allow || !strings.Contains(dec.Reason, "refusing Task of the phase persona evo:evolve-builder") {
		t.Errorf("nil storage must deny by default: %+v", dec)
	}
}

func TestNewPhase_ReadsTheCellMarkerOfTheProcessEnvironment(t *testing.T) {
	s, _ := setupStorageWithCS(t, core.CycleState{CycleID: 42, Phase: "build"})
	t.Setenv("EVOLVE_PHASE", "build")
	if dec := NewPhase(s, false).Decide(context.Background(), personaCall("Task", "evo:evolve-builder")); !dec.Allow {
		t.Errorf("NewPhase must read EVOLVE_PHASE from the process: %s", dec.Reason)
	}
}

func TestPhase_ACellOrAGenericSubagentNeedsNoStorage(t *testing.T) {
	if dec := newPhaseWithEnv(nil, false, inCellEnv).Decide(context.Background(), personaCall("Task", "evo:evolve-builder")); !dec.Allow {
		t.Errorf("inside a cell the guard must allow before it reads storage: %s", dec.Reason)
	}
	if dec := newPhaseWithEnv(nil, false, noCellEnv).Decide(context.Background(), personaCall("Task", "general-purpose")); !dec.Allow {
		t.Errorf("a generic subagent must pass before the guard reads storage: %s", dec.Reason)
	}
}
