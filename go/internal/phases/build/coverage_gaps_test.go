package build

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
)

func TestComposePrompt_EmitsWorktreeLineWhenSet(t *testing.T) {
	req := core.PhaseRequest{
		Cycle:       7,
		ProjectRoot: "/proj",
		Workspace:   "/ws",
		Worktree:    "/proj/.evolve/worktrees/cycle-7",
	}

	got := hooks{}.ComposePrompt("body", req)

	wantLine := "- worktree: /proj/.evolve/worktrees/cycle-7\n"
	if !strings.Contains(got, wantLine) {
		t.Errorf("ComposePrompt missing worktree line %q; got:\n%s", wantLine, got)
	}
}

func TestComposePrompt_OmitsWorktreeLineWhenEmpty(t *testing.T) {
	req := core.PhaseRequest{Cycle: 7, ProjectRoot: "/proj", Workspace: "/ws"}

	got := hooks{}.ComposePrompt("body", req)

	if strings.Contains(got, "- worktree:") {
		t.Errorf("ComposePrompt emitted a worktree line for empty Worktree; got:\n%s", got)
	}
}

func TestRegistry_BuildFactoryConstructsRunnablePhase(t *testing.T) {
	factory, ok := registry.For(string(core.PhaseBuild))
	if !ok {
		t.Fatalf("registry.For(%q) = not found; init() registration missing", core.PhaseBuild)
	}

	runner := factory(core.PhaseRequest{ProjectRoot: t.TempDir()})

	if runner == nil {
		t.Fatal("build factory returned a nil PhaseRunner")
	}
	if got := runner.Name(); got != "build" {
		t.Errorf("factory-built phase Name=%q, want build", got)
	}
}
