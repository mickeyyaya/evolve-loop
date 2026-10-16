// Package debugger implements the ship-recovery diagnosis phase.
// See docs/architecture/packages/internal-phases-debugger.md.
package debugger

import (
	"context"
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
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

const decisionFilename = "debug-decision.json"

const (
	actionReship     = "RESHIP"
	actionRerunPhase = "RERUN_PHASE"
	actionBlock      = "BLOCK"
)

const (
	signalAction     = "debugger.action"
	signalRerunPhase = "debugger.rerun_phase"
	signalRootCause  = "debugger.root_cause"
)

type decision struct {
	Action     string `json:"action"`
	RerunPhase string `json:"rerun_phase"`
	FixApplied string `json:"fix_applied"`
	RootCause  string `json:"root_cause"`
	Reasoning  string `json:"reasoning"`
}

func Classify(artifactDir string) (verdict string, signals map[string]string, diags []core.Diagnostic) {
	signals = map[string]string{}

	path := filepath.Join(artifactDir, decisionFilename)
	data, err := os.ReadFile(path)
	if err != nil {
		signals[signalAction] = actionBlock
		return core.VerdictFAIL, signals, []core.Diagnostic{{
			Severity: "error",
			Message:  fmt.Sprintf("debugger: %s unreadable (%v) — safe-defaulting to BLOCK", decisionFilename, err),
		}}
	}

	var d decision
	if err := json.Unmarshal(data, &d); err != nil {
		signals[signalAction] = actionBlock
		return core.VerdictFAIL, signals, []core.Diagnostic{{
			Severity: "error",
			Message:  fmt.Sprintf("debugger: %s malformed JSON (%v) — safe-defaulting to BLOCK", decisionFilename, err),
		}}
	}

	if rc := strings.TrimSpace(d.RootCause); rc != "" {
		signals[signalRootCause] = rc
	}

	switch strings.ToUpper(strings.TrimSpace(d.Action)) {
	case actionReship:
		signals[signalAction] = actionReship
		return core.VerdictPASS, signals, nil

	case actionRerunPhase:
		signals[signalAction] = actionRerunPhase
		phase := strings.TrimSpace(d.RerunPhase)
		if phase == "" {
			phase = string(core.PhaseAudit)
		}
		signals[signalRerunPhase] = phase
		return core.VerdictPASS, signals, nil

	default:
		signals[signalAction] = actionBlock
		if d.Action != "" && strings.ToUpper(strings.TrimSpace(d.Action)) != actionBlock {
			return core.VerdictFAIL, signals, []core.Diagnostic{{
				Severity: "error",
				Message:  fmt.Sprintf("debugger: unknown action %q — safe-defaulting to BLOCK", d.Action),
			}}
		}
		return core.VerdictFAIL, signals, nil
	}
}

func nextPhaseFor(action string) string {
	if action == actionReship {
		return string(core.PhaseShip)
	}
	return ""
}

type hooks struct{}

func (hooks) PhaseName() string                           { return string(core.PhaseDebugger) }
func (hooks) AgentPromptName() string                     { return "evolve-debugger" }
func (hooks) ArtifactFilename(_ core.PhaseRequest) string { return decisionFilename }
func (hooks) DefaultModel() string                        { return "opus" }

func (hooks) ComposePrompt(body string, req core.PhaseRequest) string {
	var b strings.Builder
	b.WriteString(runner.BaseCycleContext(body, req))
	if req.Worktree != "" {
		fmt.Fprintf(&b, "- worktree: %s\n", req.Worktree)
	}
	b.WriteString("\n## Ship Failure Envelope\n")
	code, class, stage, dbg := req.Context[core.CtxKeyShipErrorCode], req.Context["ship_error_class"], req.Context["ship_error_stage"], req.Context["ship_error_debug"]
	if req.Input.Active() {
		ec, _ := req.Input.ErrorContext()
		code, class, stage, dbg = ec.Code, ec.Class, ec.Stage, ec.Debug
	}
	for _, f := range []struct{ k, v string }{
		{core.CtxKeyShipErrorCode, code},
		{"ship_error_class", class},
		{"ship_error_stage", stage},
		{"ship_error_debug", dbg},
	} {
		if f.v != "" {
			fmt.Fprintf(&b, "- %s: %s\n", f.k, f.v)
		}
	}
	if len(req.WorktreeWritablePaths) > 0 {
		fmt.Fprintf(&b, "- conflicted_paths: %s\n", strings.Join(req.WorktreeWritablePaths, ", "))
	}
	return b.String()
}

func (hooks) Classify(_ string, req core.PhaseRequest, _ core.BridgeResponse) (string, []core.Diagnostic, string) {
	verdict, signals, diags := Classify(req.Workspace)
	return verdict, diags, nextPhaseFor(signals[signalAction])
}

type Config struct {
	Bridge           core.Bridge
	Prompts          *prompts.Loader
	ContractVerifier func() runner.ContractVerifier
	HostEffects      func() core.HostEffects
	NowFn            func() time.Time
	CompactPrompts   bool
}

type Phase struct {
	*runner.BaseRunner
}

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
		}),
	}
}

func (p *Phase) Run(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	resp, err := p.BaseRunner.Run(ctx, req)
	if err != nil {
		return resp, err
	}
	_, signals, _ := Classify(req.Workspace)
	if resp.Signals == nil {
		resp.Signals = map[string]any{}
	}
	for k, v := range signals {
		resp.Signals[k] = v
	}
	return resp, nil
}

func init() {
	registry.Register(string(core.PhaseDebugger), func(req core.PhaseRequest) core.PhaseRunner {
		return New(Config{
			Bridge:  bridge.NewDefault(req.ProjectRoot, nil),
			Prompts: prompts.NewForProject(req.ProjectRoot),
		})
	})
}
