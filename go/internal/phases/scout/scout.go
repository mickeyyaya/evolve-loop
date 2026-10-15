// Package scout implements the discovery and planning phase.
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

var proposedTasksRE = buildScoutBacklogRE()

func buildScoutBacklogRE() *regexp.Regexp {
	accepted := phasecontract.SelectedTasks.Accepted
	alts := make([]string, len(accepted))
	for i, h := range accepted {
		alts[i] = regexp.QuoteMeta(h)
	}
	return regexp.MustCompile(`(?m)^(?:` + strings.Join(alts, "|") + `)\b[\s\S]*?^(?:[*\-0-9]+\.?\s+\S|###\s+\S)`)
}

type hooks struct{}

func (hooks) PhaseName() string       { return string(core.PhaseScout) }
func (hooks) AgentPromptName() string { return "evolve-scout" }
func (hooks) ArtifactFilename(_ core.PhaseRequest) string {
	return phasecontract.ArtifactName(string(core.PhaseScout))
}
func (hooks) DefaultModel() string { return "auto" }

func (hooks) ComposePrompt(body string, req core.PhaseRequest) string {
	var b strings.Builder
	b.WriteString(runner.BaseCycleContext(body, req))
	strategy, goal, tok := req.Context["strategy"], req.Context["goal"], req.Context["challengeToken"]
	if req.Input.Active() {
		ci := req.Input.CycleInputs()
		strategy, goal, tok = ci.Strategy(), ci.Goal(), ci.ChallengeToken()
	}
	if strategy != "" {
		fmt.Fprintf(&b, "- strategy: %s\n", strategy)
	}
	if kind := req.Context[core.CtxKeyDeliverableKindDefault]; kind != "" {
		fmt.Fprintf(&b, "- deliverable_kind_default: %s\n", kind)
	}
	if root := req.Context[core.CtxKeyDeliverableRoot]; root != "" {
		fmt.Fprintf(&b, "- deliverable_root: %s\n", root)
	}
	if goal != "" {
		fmt.Fprintf(&b, "- goal: %s\n", goal)
	}
	if tok != "" {
		fmt.Fprintf(&b, "- challenge_token: %s\n", tok)
	}
	if scope := runner.LaneScope(req); scope != "" {
		fmt.Fprintf(&b, "- fleet_scope: this is one of several concurrent cycles; scout ONLY within this assigned todo-id set, ignore all other candidate work: %s\n", scope)
		if paths := req.Context["fleet_scope_paths"]; paths != "" {
			fmt.Fprintf(&b, "- fleet_scope_records: the LIVE inbox record for each assigned id — READ THESE EXACT FILES; same-named files under inbox/consumed|processed|rejected are STALE NAMESAKES of already-handled work, never your task: %s\n", paths)
		}
		b.WriteString("- lane_coherence: echo the pinned goal_hash verbatim under a \"goal_hash\" key in your report's Decision Trace JSON block\n")
	}
	if ro := req.Context["recent_outcomes"]; ro != "" {
		fmt.Fprintf(&b, "- recent_outcomes: %s\n", ro)
	}
	return b.String()
}

func (hooks) Classify(artifact string, req core.PhaseRequest, _ core.BridgeResponse) (string, []core.Diagnostic, string) {
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

type Config struct {
	Bridge           core.Bridge
	Prompts          *prompts.Loader
	ContractVerifier func() runner.ContractVerifier
	HostEffects      func() core.HostEffects
	NowFn            func() time.Time
	PhaseIO          config.Stage
	CompactPrompts   bool
	SleepFn          func(time.Duration)
}

type Phase struct{ *runner.BaseRunner }

func New(c Config) *Phase {
	return &Phase{
		BaseRunner: runner.New(runner.Options{
			Hooks:            hooks{},
			Bridge:           c.Bridge,
			ContractVerifier: c.ContractVerifier,
			HostEffects:      c.HostEffects,
			Prompts:          c.Prompts,
			NowFn:            c.NowFn,
			PhaseIO:          c.PhaseIO,
			CompactPrompts:   c.CompactPrompts,
			SleepFn:          c.SleepFn,
		}),
	}
}

func init() {
	registry.Register(string(core.PhaseScout), func(req core.PhaseRequest) core.PhaseRunner {
		return New(Config{
			Bridge:  bridge.NewDefault(req.ProjectRoot, nil),
			Prompts: prompts.NewForProject(req.ProjectRoot),
		})
	})
}
