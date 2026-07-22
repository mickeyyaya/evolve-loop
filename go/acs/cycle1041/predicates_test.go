//go:build acs

// Package cycle1041 materialises the cycle-1041 acceptance criteria for the one
// fleet-scoped task pinned to this lane:
//
//   - retro-role-gate-lessons-write-allowance
//
// The defect. go/internal/guards/role.go:11-16 documents a per-phase allowance
// "learn/retrospective: workspace_path + .evolve/lessons/**", but Decide()
// (role.go:30-91) has exactly two allow branches for a non-always-safe,
// non-protected path: under cs.WorkspacePath, and (for WorktreePhase only) under
// cs.ActiveWorktree. There is NO branch keyed on the retro phase, so a retro
// write to the lessons directory falls through to the terminal deny. The
// documented allowance does not exist in code.
//
// Phase-name correction carried by these predicates. The doc comment names
// "learn/retrospective", but the canonical runtime value of CycleState.Phase is
// the string "retro" (go/internal/cyclestate/phase.go:18, PhaseRetro). A fix
// gated on the doc comment's wording would compile, pass a wording-shaped test,
// and STILL never fire in production. Predicate 001 therefore drives the guard
// with the real runtime value.
//
// Predicate strategy — every predicate CONSTRUCTS the guard under test and calls
// Decide(), asserting on the returned decision. None greps role.go's source
// (the cycle-85 degenerate-predicate ban): a predicate that merely looked for
// the string ".evolve/instincts/lessons" in role.go would pass on a comment.
//
//   - 001 is the crux (currently RED): phase=retro writing
//     <root>/.evolve/instincts/lessons/<name>.yaml must be ALLOWED.
//   - 002 is the scoping negative: under the SAME phase and the SAME root, a
//     path OUTSIDE lessons/** must stay DENIED. It is also the structural
//     control for 001 — if the temp root were an always-safe prefix (/tmp/**),
//     making 001 pass for the wrong reason, 002 fails loudly.
//   - 003 is the phase negative: a non-retro phase writing the very same lessons
//     path must stay DENIED, so the fix cannot be a blanket path allowance.
//   - 004 is the anti-gaming precedence pin (expected pre-existing GREEN): the
//     new allowance must be placed AFTER the ADR-0064 control-plane check, so a
//     retro phase still cannot edit a protected surface by routing through it.
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

// retroPhase is the CANONICAL runtime value of CycleState.Phase for the
// retrospective phase (core.PhaseRetro == cyclestate.PhaseRetro == "retro").
// Bound to the constant, not a literal, so a future rename of the phase
// vocabulary breaks this predicate loudly instead of silently un-gating.
var retroPhase = string(core.PhaseRetro)

// lessonsRel is the lessons directory the retro phase must be able to write,
// relative to the project's .evolve dir. Note this is .evolve/instincts/lessons
// — NOT the stale ".evolve/lessons" the role.go doc comment names.
var lessonsRel = filepath.Join("instincts", "lessons")

// decideWrite builds a Role guard over a real cycle-state for the given phase
// and asks it to decide a Write of path. It returns the guard's decision — the
// system under test is exercised, never inspected.
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
	// bypass=false: the constructor-injected emergency override must not be the
	// reason any assertion below holds.
	g := guards.NewRole(s, false)
	return g.Decide(context.Background(), core.GuardInput{
		ToolName:  "Write",
		ToolInput: map[string]any{"file_path": strings.ReplaceAll(path, "{root}", root)},
	})
}

// TestC1041_001_RetroPhaseMayWriteLessonsDir is the crux predicate. RED before
// the fix: Decide() has no retro branch, so the lessons write is denied with
// "may not write outside workspace".
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

// TestC1041_002_RetroPhaseStillDeniedOutsideLessonsDir is the scoping negative
// AND the structural control for 001: same phase, same temp root, a path that is
// merely a SIBLING of the lessons dir must remain denied. If this fails, the
// allowance is too wide (or the root is an always-safe prefix, which would make
// 001 a false GREEN).
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

// TestC1041_003_NonRetroPhasesStillDeniedLessonsDir is the phase negative: the
// allowance must be keyed on the phase, not on the path alone. A build phase
// (whose worktree is elsewhere) writing the lessons dir must stay denied.
func TestC1041_003_NonRetroPhasesStillDeniedLessonsDir(t *testing.T) {
	target := filepath.Join("{root}", ".evolve", lessonsRel, "smuggled.yaml")
	for _, phase := range []string{"build", "audit", "scout", "tdd"} {
		t.Run(phase, func(t *testing.T) {
			dec := decideWrite(t, phase, target, func(cs *core.CycleState) {
				// A worktree well away from the lessons dir, so the existing
				// WorktreePhase branch cannot be what decides this case.
				cs.ActiveWorktree = "/work/wt/cycle-1041"
			})
			if dec.Allow {
				t.Errorf("phase=%q write to the lessons dir ALLOWED, want DENY; the retro allowance leaked to a non-retro phase", phase)
			}
		})
	}
}

// TestC1041_004_ControlPlanePrecedenceSurvivesRetroAllowance pins the ordering
// invariant (ADR-0064): the new retro branch must sit AFTER the
// IsProtectedSurface check, so a retro phase still cannot edit the gate that
// grades its own cycle. Expected pre-existing GREEN — it is the regression pin
// that fails if the fix is inserted above the integrity boundary.
func TestC1041_004_ControlPlanePrecedenceSurvivesRetroAllowance(t *testing.T) {
	// A protected control-plane path that ALSO lives under the lessons dir: the
	// only way this is allowed is if the retro branch short-circuits ahead of
	// the integrity check.
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
