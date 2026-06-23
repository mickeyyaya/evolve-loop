// Coverage-completing tests for build.go: the Worktree branch of
// ComposePrompt and the init()-registered factory closure. Each pins a
// behavior the existing suite left unverified.
package build

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
	"github.com/mickeyyaya/evolveloop/go/internal/phases/registry"
)

// TestComposePrompt_EmitsWorktreeLineWhenSet pins that a non-empty
// req.Worktree is surfaced in the cycle-context block so the builder
// agent knows which worktree to write into.
func TestComposePrompt_EmitsWorktreeLineWhenSet(t *testing.T) {
	// Arrange
	req := core.PhaseRequest{
		Cycle:       7,
		ProjectRoot: "/proj",
		Workspace:   "/ws",
		Worktree:    "/proj/.evolve/worktrees/cycle-7",
	}

	// Act
	got := hooks{}.ComposePrompt("body", req)

	// Assert
	wantLine := "- worktree: /proj/.evolve/worktrees/cycle-7\n"
	if !strings.Contains(got, wantLine) {
		t.Errorf("ComposePrompt missing worktree line %q; got:\n%s", wantLine, got)
	}
}

// TestComposePrompt_OmitsWorktreeLineWhenEmpty is the negative companion:
// an empty Worktree must not emit a worktree line.
func TestComposePrompt_OmitsWorktreeLineWhenEmpty(t *testing.T) {
	// Arrange
	req := core.PhaseRequest{Cycle: 7, ProjectRoot: "/proj", Workspace: "/ws"}

	// Act
	got := hooks{}.ComposePrompt("body", req)

	// Assert
	if strings.Contains(got, "- worktree:") {
		t.Errorf("ComposePrompt emitted a worktree line for empty Worktree; got:\n%s", got)
	}
}

// TestRegistry_BuildFactoryConstructsRunnablePhase pins that the init()
// self-registration publishes a factory under "build" whose closure
// constructs a non-nil PhaseRunner named "build". This exercises the
// registered closure (bridge + prompts wiring) that the direct New()
// tests bypass.
func TestRegistry_BuildFactoryConstructsRunnablePhase(t *testing.T) {
	// Arrange
	factory, ok := registry.For(string(core.PhaseBuild))
	if !ok {
		t.Fatalf("registry.For(%q) = not found; init() registration missing", core.PhaseBuild)
	}

	// Act
	runner := factory(core.PhaseRequest{ProjectRoot: t.TempDir()})

	// Assert
	if runner == nil {
		t.Fatal("build factory returned a nil PhaseRunner")
	}
	if got := runner.Name(); got != "build" {
		t.Errorf("factory-built phase Name=%q, want build", got)
	}
}
