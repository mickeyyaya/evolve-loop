//go:build acs

package cycle1042

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
)

type memStorage struct{ cs core.CycleState }

func (m *memStorage) ReadState(context.Context) (core.State, error) { return core.State{}, nil }
func (m *memStorage) WriteState(context.Context, core.State) error  { return nil }
func (m *memStorage) ReadCycleState(context.Context) (core.CycleState, error) {
	return m.cs, nil
}
func (m *memStorage) WriteCycleState(context.Context, core.CycleState) error { return nil }
func (m *memStorage) AcquireLock(context.Context) (func() error, error) {
	return func() error { return nil }, nil
}

func fixture(t *testing.T, phase string) (*guards.Role, string, string) {
	t.Helper()
	evolveDir := filepath.Join(t.TempDir(), ".evolve")
	workspace := filepath.Join(evolveDir, "runs", "cycle-1042")
	return guards.NewRole(&memStorage{cs: core.CycleState{
		CycleID:        1042,
		Phase:          phase,
		WorkspacePath:  workspace,
		ActiveWorktree: filepath.Join(t.TempDir(), "wt", "cycle-1042"),
	}}, false), evolveDir, workspace
}

func decideWrite(g *guards.Role, tool, path string) core.GuardDecision {
	return g.Decide(context.Background(), core.GuardInput{
		ToolName:  tool,
		ToolInput: map[string]any{"file_path": path},
	})
}

func lessonsPath(evolveDir, name string) string {
	return filepath.Join(evolveDir, "instincts", "lessons", name)
}

func TestC1042_001_RetroWritesLessonsCorpus(t *testing.T) {
	for _, tool := range []string{"Edit", "Write"} {
		g, evolveDir, _ := fixture(t, string(core.PhaseRetro))
		path := lessonsPath(evolveDir, "retro-role-gate.yaml")
		if dec := decideWrite(g, tool, path); !dec.Allow {
			t.Errorf("%s to lessons corpus denied for phase=%s (path=%s): %s",
				tool, core.PhaseRetro, path, dec.Reason)
		}
	}
}

func TestC1042_002_RetroWritesNestedLessonsPath(t *testing.T) {
	g, evolveDir, _ := fixture(t, string(core.PhaseRetro))
	path := lessonsPath(evolveDir, filepath.Join("2026-07", "nested.yaml"))
	if dec := decideWrite(g, "Write", path); !dec.Allow {
		t.Errorf("nested lessons write denied (path=%s): %s", path, dec.Reason)
	}
}

func TestC1042_003_NonRetroDeniedLessonsCorpus(t *testing.T) {
	for _, phase := range []string{
		string(core.PhaseBuild), string(core.PhaseAudit),
		string(core.PhaseTDD), string(core.PhaseShip),
	} {
		g, evolveDir, _ := fixture(t, phase)
		path := lessonsPath(evolveDir, "sneaky.yaml")
		if dec := decideWrite(g, "Write", path); dec.Allow {
			t.Errorf("phase=%s was ALLOWED to write the lessons corpus (path=%s) — "+
				"the allowance must be gated on phase=%s", phase, path, core.PhaseRetro)
		}
	}
}

func TestC1042_004_RetroDeniedOutsideLessonsAndWorkspace(t *testing.T) {
	g, evolveDir, _ := fixture(t, string(core.PhaseRetro))
	for _, rel := range []string{
		"state.json",
		"ledger.jsonl",
		filepath.Join("instincts", "instincts.yaml"),
		filepath.Join("inbox", "item.json"),
	} {
		path := filepath.Join(evolveDir, rel)
		if dec := decideWrite(g, "Write", path); dec.Allow {
			t.Errorf("phase=%s was ALLOWED to write %s — allowance must be scoped to "+
				"instincts/lessons/ only", core.PhaseRetro, path)
		}
	}
}

func TestC1042_005_RetroLessonsTraversalDenied(t *testing.T) {
	g, evolveDir, _ := fixture(t, string(core.PhaseRetro))
	escaped := lessonsPath(evolveDir, filepath.Join("..", "..", "policy.json"))
	if dec := decideWrite(g, "Write", escaped); dec.Allow {
		t.Errorf("traversal out of the lessons corpus was ALLOWED (path=%s)", escaped)
	}
}

func TestC1042_006_LessonsAllowanceNeverBeatsProtectedSurface(t *testing.T) {
	g, _, _ := fixture(t, string(core.PhaseRetro))
	protected := filepath.Join("/repo", "go", "internal", "guards", "role.go")
	if !guards.IsProtectedSurface(protected) {
		t.Fatalf("fixture invalid: %s is not a protected surface", protected)
	}
	dec := decideWrite(g, "Write", protected)
	if dec.Allow {
		t.Errorf("retro phase was ALLOWED to write protected surface %s", protected)
	}
	if !dec.Alarm {
		t.Errorf("protected-surface deny from phase=%s did not raise Alarm (reason=%q)",
			core.PhaseRetro, dec.Reason)
	}
}

func TestC1042_007_RetroWorkspaceWriteStillAllowed(t *testing.T) {
	g, _, workspace := fixture(t, string(core.PhaseRetro))
	path := filepath.Join(workspace, "retro-report.md")
	if dec := decideWrite(g, "Write", path); !dec.Allow {
		t.Errorf("retro workspace write regressed (path=%s): %s", path, dec.Reason)
	}
}
