//go:build acs

package cycle1041

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
)

var retroPhase = string(core.PhaseRetro)

var lessonsRel = filepath.Join("instincts", "lessons")

func decideWrite(t *testing.T, phase, path string, cs func(*core.CycleState)) core.GuardDecision {
	t.Helper()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	workspace := filepath.Join(evolveDir, "runs", "cycle-1041")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	state := core.CycleState{
		CycleID:       1041,
		Phase:         phase,
		WorkspacePath: workspace,
	}
	if cs != nil {
		cs(&state)
	}
	s := storage.New(evolveDir)
	if err := s.WriteCycleState(context.Background(), state); err != nil {
		t.Fatalf("write cycle-state: %v", err)
	}
	g := guards.NewRole(s, false)
	return g.Decide(context.Background(), core.GuardInput{
		ToolName:  "Write",
		ToolInput: map[string]any{"file_path": strings.ReplaceAll(path, "{root}", root)},
	})
}

func TestC1041_001_RetroPhaseMayWriteLessonsDir(t *testing.T) {
	target := filepath.Join("{root}", ".evolve", lessonsRel, "cycle-1035-audit-stall.yaml")
	dec := decideWrite(t, retroPhase, target, nil)
	if !dec.Allow {
		t.Errorf("phase=%q write to .evolve/%s/*.yaml DENIED, want ALLOW; the retro phase cannot persist a lesson. reason=%q",
			retroPhase, lessonsRel, dec.Reason)
	}
	if dec.Alarm {
		t.Errorf("phase=%q lessons write raised the integrity alarm; a sanctioned retro deliverable must not be alarmed", retroPhase)
	}
}

func TestC1041_002_RetroPhaseStillDeniedOutsideLessonsDir(t *testing.T) {
	cases := []struct {
		name string
		path string
	}{
		{"sibling under .evolve/instincts", filepath.Join("{root}", ".evolve", "instincts", "not-a-lesson.yaml")},
		{"stale doc path .evolve/lessons", filepath.Join("{root}", ".evolve", "lessons", "wrong-dir.yaml")},
		{"ordinary source file", filepath.Join("{root}", "go", "internal", "retrofile", "write.go")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dec := decideWrite(t, retroPhase, tc.path, nil)
			if dec.Allow {
				t.Errorf("phase=%q write to %s ALLOWED, want DENY; the retro allowance must be scoped to .evolve/%s/** only",
					retroPhase, tc.path, lessonsRel)
			}
		})
	}
}

func TestC1041_003_NonRetroPhasesStillDeniedLessonsDir(t *testing.T) {
	target := filepath.Join("{root}", ".evolve", lessonsRel, "smuggled.yaml")
	for _, phase := range []string{"build", "audit", "scout", "tdd"} {
		t.Run(phase, func(t *testing.T) {
			dec := decideWrite(t, phase, target, func(cs *core.CycleState) {
				cs.ActiveWorktree = "/work/wt/cycle-1041"
			})
			if dec.Allow {
				t.Errorf("phase=%q write to the lessons dir ALLOWED, want DENY; the retro allowance leaked to a non-retro phase", phase)
			}
		})
	}
}

func TestC1041_004_ControlPlanePrecedenceSurvivesRetroAllowance(t *testing.T) {
	protected := filepath.Join("{root}", ".evolve", lessonsRel, "go", "internal", "guards", "role.go")
	if !guards.IsProtectedSurface(strings.ReplaceAll(protected, "{root}", "/work/proj")) {
		t.Fatalf("fixture is not on the protected surface; the precedence pin would be vacuous")
	}
	dec := decideWrite(t, retroPhase, protected, nil)
	if dec.Allow {
		t.Errorf("phase=%q was allowed to write a protected control-plane path via the lessons dir; the retro allowance must not precede the ADR-0064 integrity check", retroPhase)
	}
	if !dec.Alarm {
		t.Errorf("protected-surface denial for phase=%q did not raise the integrity alarm", retroPhase)
	}
}
