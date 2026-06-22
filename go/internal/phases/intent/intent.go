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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
		// The unchanged-goal case is decided deterministically in ShouldSkip
		// BEFORE dispatch (ADR-0062/T1.5, Core Rule 5), so by the time the LLM
		// runs the goal HAS changed — ask only for the delta, not a hash compare.
		b.WriteString("- mode: delta (the goal changed since the last batch; emit intent-delta.md describing what is new or changed)\n")
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

// ShouldSkip implements runner.Skipper: it makes the delta-mode "is the goal
// unchanged?" decision DETERMINISTICALLY in code (Core Rule 5 / ADR-0062 T1.5),
// instead of delegating a goal-hash comparison to the LLM prompt. When this
// cycle's goal hash matches the current batch's recorded hash, the goal is
// unchanged → skip intent (and its LLM dispatch entirely) and route to scout.
// Any uncertainty (full mode, blank hash, unreadable state) FAILS OPEN: intent
// runs. When the goal changed, intent runs so the LLM synthesizes the delta.
func (hooks) ShouldSkip(req core.PhaseRequest) (bool, string, string, []core.Diagnostic) {
	if !isDeltaMode(req) || req.GoalHash == "" {
		return false, "", "", nil
	}
	prior := readBatchGoalHash(req.ProjectRoot)
	if prior != "" && prior == req.GoalHash {
		return true, core.VerdictSKIPPED, string(core.PhaseScout), nil
	}
	return false, "", "", nil
}

// readBatchGoalHash reads state.json:currentBatch.goalHash from the project's
// .evolve dir. Fail-open: any read/parse error yields "" (→ intent runs). It
// decodes only the one field it needs so it does not couple intent to the full
// cyclestate.State schema.
func readBatchGoalHash(projectRoot string) string {
	data, err := os.ReadFile(filepath.Join(projectRoot, ".evolve", "state.json"))
	if err != nil {
		return ""
	}
	var s struct {
		CurrentBatch struct {
			GoalHash string `json:"goalHash"`
		} `json:"currentBatch"`
	}
	if json.Unmarshal(data, &s) != nil {
		return ""
	}
	return s.CurrentBatch.GoalHash
}

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
