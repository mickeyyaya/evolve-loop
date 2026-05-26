// Package tdd implements the RED test-writer phase. The phase
// boilerplate lives in internal/phases/runner; this file only encodes
// tdd-specific variation points.
//
// Skip semantics (Skipper interface):
//   - EVOLVE_TEST_PHASE_ENABLED=0 → SKIPPED, NextPhase=build, no bridge call
//
// Verdict mapping (team-context.md body):
//   - empty artifact → FAIL
//   - missing "## Acceptance" → FAIL
//   - missing "## RED Tests" → FAIL
//   - otherwise → PASS
package tdd

import (
	"fmt"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

type hooks struct{}

func (hooks) PhaseName() string                           { return string(core.PhaseTDD) }
func (hooks) AgentPromptName() string                     { return "evolve-tdd-engineer" }
func (hooks) ArtifactFilename(_ core.PhaseRequest) string { return "team-context.md" }
func (hooks) DefaultModel() string                        { return "auto" }

// ShouldSkip delegates to the central PhasePolicy (config.Load is the sole
// reader of EVOLVE_TEST_PHASE_ENABLED). Legacy posture preserved: tdd runs
// unless the flag disables it. From Stage:Enforce up, the conditional-pin
// (cycle_size != trivial) makes tdd un-disablable by flag.
func (hooks) ShouldSkip(req core.PhaseRequest) (bool, string, string, []core.Diagnostic) {
	if router.PolicyForProject(req.ProjectRoot, req.Env).ShouldRunPhase(string(core.PhaseTDD)) {
		return false, "", "", nil
	}
	return true, core.VerdictSKIPPED, string(core.PhaseBuild), nil
}

func (hooks) ComposePrompt(body string, req core.PhaseRequest) string {
	var b strings.Builder
	b.WriteString(body)
	b.WriteString("\n\n## Cycle Context\n")
	fmt.Fprintf(&b, "- cycle: %d\n", req.Cycle)
	fmt.Fprintf(&b, "- goal_hash: %s\n", req.GoalHash)
	fmt.Fprintf(&b, "- project_root: %s\n", req.ProjectRoot)
	fmt.Fprintf(&b, "- workspace: %s\n", req.Workspace)
	if req.Worktree != "" {
		fmt.Fprintf(&b, "- worktree: %s\n", req.Worktree)
	}
	return b.String()
}

func (hooks) Classify(artifact string, _ core.PhaseRequest, _ core.BridgeResponse) (string, []core.Diagnostic, string) {
	return classify(artifact), nil, string(core.PhaseBuild)
}

func classify(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return core.VerdictFAIL
	}
	if !strings.Contains(trimmed, "## Acceptance") {
		return core.VerdictFAIL
	}
	if !strings.Contains(trimmed, "## RED Tests") {
		return core.VerdictFAIL
	}
	return core.VerdictPASS
}

type Config struct {
	Bridge  core.Bridge
	Prompts *prompts.Loader
	NowFn   func() time.Time
}

type Phase struct{ *runner.BaseRunner }

func New(c Config) *Phase {
	return &Phase{
		BaseRunner: runner.New(runner.Options{
			Hooks:   hooks{},
			Bridge:  c.Bridge,
			Prompts: c.Prompts,
			NowFn:   c.NowFn,
		}),
	}
}

func init() {
	registry.Register(string(core.PhaseTDD), func(req core.PhaseRequest) core.PhaseRunner {
		return New(Config{
			Bridge:  bridge.NewDefault(req.ProjectRoot),
			Prompts: prompts.NewForProject(req.ProjectRoot),
		})
	})
}
