package tdd

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
