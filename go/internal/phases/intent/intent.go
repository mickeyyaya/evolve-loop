// Package intent implements the goal-capture phase.
// See docs/architecture/packages/internal-phases-intent.md.
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
	goal := req.Context["goal"]
	if req.Input.Active() {
		goal = req.Input.CycleInputs().Goal()
	}
	if goal != "" {
		fmt.Fprintf(&b, "- goal: %s\n", goal)
	}
	if isDeltaMode(req) {
		b.WriteString("- mode: delta (the goal changed since the last batch; emit intent-delta.md describing what is new or changed)\n")
	} else {
		b.WriteString("- mode: full\n")
	}
	if req.Input.Active() {
		b.WriteString(runner.UpstreamDigestBlock(req.Input.Upstream().DigestFor(string(core.PhaseIntent))))
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

type Config struct {
	Bridge           core.Bridge
	Prompts          *prompts.Loader
	ContractVerifier func() runner.ContractVerifier
	HostEffects      func() core.HostEffects
	NowFn            func() time.Time
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
			CompactPrompts:   c.CompactPrompts,
			SleepFn:          c.SleepFn,
		}),
	}
}

func init() {
	registry.Register(string(core.PhaseIntent), func(req core.PhaseRequest) core.PhaseRunner {
		return New(Config{
			Bridge:  bridge.NewDefault(req.ProjectRoot, nil),
			Prompts: prompts.NewForProject(req.ProjectRoot),
		})
	})
}
