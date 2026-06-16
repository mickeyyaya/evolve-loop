// Package scout implements the discovery and planning phase. The
// phase boilerplate lives in internal/phases/runner; this file only
// encodes scout-specific variation points.
//
// Verdict mapping (artifact body inspected, all checks case-sensitive):
//   - convergence-confirmation strategy + no tasks section → SKIPPED
//   - "## Proposed Tasks" or "## Selected Tasks" section with at least one
//     list item or "### " task subheading → PASS
//   - empty/missing artifact, or tasks section absent/empty → FAIL
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
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

// proposedTasksRE confirms a non-empty backlog: the tasks heading followed by a
// list item OR a "### " task subheading. The accepted headings come from
// phasecontract.Scout (single source); the "≥1 item" shape is stable here.
var proposedTasksRE = buildScoutBacklogRE()

func buildScoutBacklogRE() *regexp.Regexp {
	accepted := phasecontract.Scout.Sections[0].Accepted
	alts := make([]string, len(accepted))
	for i, h := range accepted {
		alts[i] = regexp.QuoteMeta(h)
	}
	return regexp.MustCompile(`(?m)^(?:` + strings.Join(alts, "|") + `)\b[\s\S]*?^(?:[*\-0-9]+\.?\s+\S|###\s+\S)`)
}

type hooks struct{}

func (hooks) PhaseName() string                           { return string(core.PhaseScout) }
func (hooks) AgentPromptName() string                     { return "evolve-scout" }
func (hooks) ArtifactFilename(_ core.PhaseRequest) string { return "scout-report.md" }
func (hooks) DefaultModel() string                        { return "auto" }

func (hooks) ComposePrompt(body string, req core.PhaseRequest) string {
	var b strings.Builder
	b.WriteString(runner.BaseCycleContext(body, req))
	// ADR-0050 §3.10 Slice 3: read strategy/goal/challengeToken from the typed
	// envelope at enforce, the legacy Context map below it (byte-identical —
	// Active() is false unless enforce, and the typed getters mirror these keys).
	strategy, goal, tok := req.Context["strategy"], req.Context["goal"], req.Context["challengeToken"]
	if req.Input.Active() {
		ci := req.Input.CycleInputs()
		strategy, goal, tok = ci.Strategy(), ci.Goal(), ci.ChallengeToken()
	}
	if strategy != "" {
		fmt.Fprintf(&b, "- strategy: %s\n", strategy)
	}
	// Goal text propagates via Context["goal"] when the operator
	// passed --goal-text. Scout reads it as a CONSTRAINT — its
	// backlog-vs-goal selection should treat the goal as canonical.
	// (Pre-fix, Scout had only the hash and read backlog regardless.)
	if goal != "" {
		fmt.Fprintf(&b, "- goal: %s\n", goal)
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
	if tok != "" {
		fmt.Fprintf(&b, "- challenge_token: %s\n", tok)
	}
	return b.String()
}

func (hooks) Classify(artifact string, req core.PhaseRequest, _ core.BridgeResponse) (string, []core.Diagnostic, string) {
	// ADR-0050 §3.10 Slice 3: the strategy gate is verdict-affecting
	// (convergence-confirmation → SKIPPED), so Classify reads the typed envelope at
	// enforce too. Byte-identical below enforce (Active() == false).
	strategy := req.Context["strategy"]
	if req.Input.Active() {
		strategy = req.Input.CycleInputs().Strategy()
	}
	return classify(artifact, strategy), nil, string(core.PhaseTriage)
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

// Config holds the dependencies for constructing a scout Phase: the bridge
// used to dispatch the agent, the prompt loader, an optional clock, and the
// PhaseIO stage.
type Config struct {
	Bridge  core.Bridge
	Prompts *prompts.Loader
	NowFn   func() time.Time
	// PhaseIO threads the EVOLVE_PHASE_IO stage into the reconcile rung (ADR-0050
	// §3.10 Slice 1). Zero value (StageOff) = byte-identical.
	PhaseIO config.Stage
}

// Phase is the scout discovery-and-planning phase, a runner.BaseRunner
// specialized with the scout-specific hooks.
type Phase struct{ *runner.BaseRunner }

// New constructs a scout Phase from c, wiring the scout hooks, bridge,
// prompts, clock, and PhaseIO stage into a runner.BaseRunner.
func New(c Config) *Phase {
	return &Phase{
		BaseRunner: runner.New(runner.Options{
			Hooks:   hooks{},
			Bridge:  c.Bridge,
			Prompts: c.Prompts,
			NowFn:   c.NowFn,
			PhaseIO: c.PhaseIO,
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
