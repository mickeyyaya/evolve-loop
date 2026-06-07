package tdd

// RED-phase projection-equivalence contract for cycle-249 task
// `runner-base-cycle-context` (intent AC: "new tests prove projection
// equivalence — old hardcoded values == new config-derived values").
//
// tdd is the representative migrated caller: its ComposePrompt must equal
// runner.BaseCycleContext(body, req) + its single phase-specific extra
// (the worktree line). Byte-equality here is the behavioral guarantee that
// the refactor changed structure, not prompts.
//
// Fails at baseline: runner.BaseCycleContext is undefined (compile RED).

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
)

func TestComposePromptParity_WithWorktree(t *testing.T) {
	req := core.PhaseRequest{
		Cycle:       249,
		GoalHash:    "8274f532",
		ProjectRoot: "/proj/root",
		Workspace:   "/ws/dir",
		Worktree:    "/wt/cycle-249",
	}
	got := hooks{}.ComposePrompt("AGENT BODY", req)
	want := runner.BaseCycleContext("AGENT BODY", req) + "- worktree: /wt/cycle-249\n"
	if got != want {
		t.Errorf("tdd ComposePrompt != BaseCycleContext + worktree extra\n got: %q\nwant: %q", got, want)
	}
}

// Negative: with no worktree, the prompt is the bare core block — no
// trailing worktree line, no other drift.
func TestComposePromptParity_NoWorktree(t *testing.T) {
	req := core.PhaseRequest{
		Cycle:       3,
		GoalHash:    "g",
		ProjectRoot: "/p",
		Workspace:   "/w",
	}
	got := hooks{}.ComposePrompt("B", req)
	want := runner.BaseCycleContext("B", req)
	if got != want {
		t.Errorf("tdd ComposePrompt without worktree must equal the bare core block\n got: %q\nwant: %q", got, want)
	}
}
