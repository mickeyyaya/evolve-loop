// Package scout implements the discovery and planning phase. The
// phase boilerplate lives in internal/phases/runner; this file only
// encodes scout-specific variation points.
//
// Verdict mapping (artifact body inspected, all checks case-sensitive):
//   - convergence-confirmation strategy + no Proposed Tasks → SKIPPED
//   - "## Proposed Tasks" section with at least one item → PASS
//   - empty/missing artifact, or "## Proposed Tasks" missing → FAIL
//
// Convergence is the only path that maps to SKIPPED. Empty backlog
// elsewhere is a real failure (Scout has nothing to feed Triage).
package scout

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

// proposedTasksRE matches "## Proposed Tasks" followed by at least one
// Markdown list item (numbered or bulleted). Used to confirm Scout
// produced a non-empty backlog.
var proposedTasksRE = regexp.MustCompile(`(?m)^## Proposed Tasks\b[\s\S]*?^[*\-0-9]+\.?\s+\S`)

type hooks struct{}

func (hooks) PhaseName() string                           { return string(core.PhaseScout) }
func (hooks) AgentPromptName() string                     { return "evolve-scout" }
func (hooks) ArtifactFilename(_ core.PhaseRequest) string { return "scout-report.md" }
func (hooks) DefaultModel() string                        { return "auto" }

func (hooks) ComposePrompt(body string, req core.PhaseRequest) string {
	var b strings.Builder
	b.WriteString(body)
	b.WriteString("\n\n## Cycle Context\n")
	fmt.Fprintf(&b, "- cycle: %d\n", req.Cycle)
	fmt.Fprintf(&b, "- goal_hash: %s\n", req.GoalHash)
	fmt.Fprintf(&b, "- project_root: %s\n", req.ProjectRoot)
	fmt.Fprintf(&b, "- workspace: %s\n", req.Workspace)
	if s := req.Context["strategy"]; s != "" {
		fmt.Fprintf(&b, "- strategy: %s\n", s)
	}
	// Goal text propagates via Context["goal"] when the operator
	// passed --goal-text. Scout reads it as a CONSTRAINT — its
	// backlog-vs-goal selection should treat the goal as canonical.
	// (Pre-fix, Scout had only the hash and read backlog regardless.)
	if g := req.Context["goal"]; g != "" {
		fmt.Fprintf(&b, "- goal: %s\n", g)
	}
	// Cycle-135 fix (PR 6): plumb the cycle's challenge token into the
	// Cycle Context block so scout doesn't have to mint its own. Per
	// agent-templates.md PR 5, every phase report must include
	// `<!-- challenge-token: <value> -->` on line 2 — the source-of-truth
	// is the orchestrator-minted token, surfaced via
	// Context["challengeToken"]. Scout was previously inventing tokens
	// (cycle 134 audit C1: `no-token-manual-run-cycle-134`; cycle 135
	// audit C1: `59576594e2e8d5c3` mint vs `5b96ecb69a0c848f` truth)
	// when this line was absent. Pairs with the runner-side write of
	// `<workspace>/challenge-token.txt` so the fallback source (per
	// agent-templates.md PR 5 precedence step 2) is also populated.
	if tok := req.Context["challengeToken"]; tok != "" {
		fmt.Fprintf(&b, "- challenge_token: %s\n", tok)
	}
	return b.String()
}

func (hooks) Classify(artifact string, req core.PhaseRequest, _ core.BridgeResponse) (string, []core.Diagnostic, string) {
	return classify(artifact, req.Context["strategy"]), nil, string(core.PhaseTriage)
}

func classify(content, strategy string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return core.VerdictFAIL
	}
	hasBacklog := proposedTasksRE.MatchString(trimmed)
	if strategy == "convergence-confirmation" && !hasBacklog {
		return core.VerdictSKIPPED
	}
	if !hasBacklog {
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
	registry.Register(string(core.PhaseScout), func(req core.PhaseRequest) core.PhaseRunner {
		return New(Config{
			Bridge:  bridge.NewDefault(req.ProjectRoot),
			Prompts: prompts.NewForProject(req.ProjectRoot),
		})
	})
}
