// Package intent implements the goal-capture phase. The phase
// boilerplate (profile lookup, prompt composition, bridge dispatch,
// artifact reading, response packaging) lives in
// internal/phases/runner; this file only encodes intent-specific
// variation points.
//
// Delta mode (EVOLVE_INTENT_DELTA=1):
//   - artifact filename switches to intent-delta.md
//   - prompt advertises delta mode to the agent
//   - "[intent-unchanged]" body classifies as SKIPPED
//
// Verdict mapping:
//   - empty artifact → FAIL
//   - delta mode + "[intent-unchanged]" → SKIPPED
//   - "goal:" and "acceptance_checks:" both present → PASS
//   - anything else → FAIL
package intent

import (
	"fmt"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/specrunner"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

type hooks struct{}

func (hooks) PhaseName() string       { return string(core.PhaseIntent) }
func (hooks) AgentPromptName() string { return "evolve-intent" }
func (hooks) DefaultModel() string    { return "auto" }

func (hooks) ArtifactFilename(req core.PhaseRequest) string {
	if isDeltaMode(req) {
		return "intent-delta.md"
	}
	return "intent.md"
}

func (hooks) ComposePrompt(body string, req core.PhaseRequest) string {
	var b strings.Builder
	b.WriteString(runner.BaseCycleContext(body, req))
	// The persona's template instructs it to "parse the user's goal" —
	// that requires the actual TEXT, not just the hash. When the
	// operator passes --goal-text "...", the dispatcher routes it
	// through Context["goal"] so intent.md gets structured around the
	// real goal instead of inferred from leftover workspace artifacts.
	// ADR-0050 §3.10 Slice 2: read from the typed envelope at enforce, the legacy
	// Context map below it (byte-identical — Active() is false unless enforce).
	goal := req.Context["goal"]
	if req.Input.Active() {
		goal = req.Input.CycleInputs().Goal()
	}
	if goal != "" {
		fmt.Fprintf(&b, "- goal: %s\n", goal)
	}
	if isDeltaMode(req) {
		b.WriteString("- mode: delta (emit intent-delta.md or [intent-unchanged] stub if goal_hash matches state.json:currentBatch.goalHash)\n")
	} else {
		b.WriteString("- mode: full\n")
	}
	return b.String()
}

func (hooks) Classify(artifact string, req core.PhaseRequest, _ core.BridgeResponse) (string, []core.Diagnostic, string) {
	if isDeltaMode(req) && strings.Contains(strings.TrimSpace(artifact), "[intent-unchanged]") {
		return core.VerdictSKIPPED, nil, string(core.PhaseScout)
	}
	verdict, diags := specrunner.EvaluateClassify(artifact, &phasespec.ClassifyRules{
		RequireSections: []string{"goal:", "acceptance_checks:"},
		FailIfEmpty:     true,
	})
	return verdict, diags, string(core.PhaseScout)
}

func isDeltaMode(req core.PhaseRequest) bool { return req.Env["EVOLVE_INTENT_DELTA"] == "1" }

// Config preserves the existing public constructor surface.
type Config struct {
	Bridge  core.Bridge
	Prompts *prompts.Loader
	NowFn   func() time.Time
}

// Phase wraps a runner.BaseRunner so callers still get a concrete
// *Phase from New.
type Phase struct{ *runner.BaseRunner }

// New constructs an intent Phase from c, wiring the intent hooks, bridge,
// prompts, and clock into a runner.BaseRunner.
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
	registry.Register(string(core.PhaseIntent), func(req core.PhaseRequest) core.PhaseRunner {
		return New(Config{
			Bridge:  bridge.NewDefault(req.ProjectRoot),
			Prompts: prompts.NewForProject(req.ProjectRoot),
		})
	})
}
