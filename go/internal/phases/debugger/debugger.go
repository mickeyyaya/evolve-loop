// Package debugger implements the ship-recovery diagnosis phase. When the
// ship phase fails with a novel/unresolved error, the orchestrator routes
// the structured core.ShipError to this phase. An LLM persona
// (evolve-debugger) diagnoses the ROOT CAUSE and emits debug-decision.json
// declaring ONE recovery action: RESHIP, RERUN_PHASE, or BLOCK. This phase's
// Classify reads that file and maps it to a verdict + cross-phase signals the
// orchestrator consumes in its decideAfterDebugger logic.
//
// Phase boilerplate lives in internal/phases/runner; this file only encodes
// debugger-specific variation points plus the Run wrapper that surfaces the
// decision via PhaseResponse.Signals (the base runner does not populate
// Signals from Classify).
//
// Verdict + signal mapping (see Classify):
//
//   - action "RESHIP"      → PASS, Signals[debugger.action]=RESHIP, NextPhase=ship
//   - action "RERUN_PHASE" → PASS, Signals[debugger.action]=RERUN_PHASE,
//     Signals[debugger.rerun_phase]=<phase> (defaults to "audit")
//   - action "BLOCK"       → FAIL, Signals[debugger.action]=BLOCK
//   - missing / malformed / unknown action → SAFE DEFAULT = BLOCK (FAIL).
//     Never RESHIP on a parse failure — a wrong reship corrupts history.
//
// Default model is "opus": ship-failure diagnosis is high-reasoning work.
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

// decisionFilename is the artifact the debugger persona is contracted to
// produce in the workspace.
const decisionFilename = "debug-decision.json"

// Recovery action vocabulary the persona may declare. Any other value
// (including empty) is treated as BLOCK by Classify.
const (
	actionReship     = "RESHIP"
	actionRerunPhase = "RERUN_PHASE"
	actionBlock      = "BLOCK"
)

// Signal keys surfaced via PhaseResponse.Signals for the orchestrator's
// decideAfterDebugger to read.
const (
	signalAction     = "debugger.action"
	signalRerunPhase = "debugger.rerun_phase"
	signalRootCause  = "debugger.root_cause"
)

// decision mirrors the debug-decision.json shape the persona emits.
type decision struct {
	Action     string `json:"action"`
	RerunPhase string `json:"rerun_phase"`
	FixApplied string `json:"fix_applied"`
	RootCause  string `json:"root_cause"`
	Reasoning  string `json:"reasoning"`
}

// Classify reads debug-decision.json from artifactDir and maps the declared
// recovery action to a verdict, the cross-phase signals the orchestrator
// consumes, and any diagnostics. It is a pure function (filesystem read only)
// so it is directly unit-testable without the bridge.
//
// SAFE DEFAULT: a missing file, malformed JSON, an empty action, or an
// unknown action ALL map to BLOCK (verdict FAIL). The phase never defaults to
// RESHIP on a parse failure — recovering by replaying a corrupt decision is
// strictly worse than stopping loudly.
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
			// A RERUN_PHASE with no phase named defaults to audit — the
			// dominant precondition-recovery target (audit-binding).
			phase = string(core.PhaseAudit)
		}
		signals[signalRerunPhase] = phase
		return core.VerdictPASS, signals, nil

	default:
		// BLOCK, empty, or any unknown action → safe block.
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

// nextPhaseFor returns the orchestrator's next-phase hint for a verdict+action
// pair. RESHIP routes back to ship; everything else leaves it empty (the
// orchestrator's decideAfterDebugger reads the action/rerun_phase signals to
// pick the real successor).
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
func (hooks) DefaultModel() string                        { return "opus" } // High-reasoning ship-failure diagnosis.

func (hooks) ComposePrompt(body string, req core.PhaseRequest) string {
	var b strings.Builder
	b.WriteString(runner.BaseCycleContext(body, req))
	if req.Worktree != "" {
		fmt.Fprintf(&b, "- worktree: %s\n", req.Worktree)
	}
	// Ship-failure envelope, carried in via PhaseRequest.Context by the
	// orchestrator (string keys: ship_error_code/class/stage/debug). Each key is
	// optional; only render what is present so a partial envelope still produces
	// a clean prompt. ADR-0050 §3.10 Slice 2: at enforce the typed ErrorContext
	// channel replaces those keys (byte-identical — Active() is false unless
	// enforce, and the zero ErrorContext renders nothing, matching an empty map).
	b.WriteString("\n## Ship Failure Envelope\n")
	code, class, stage, dbg := req.Context["ship_error_code"], req.Context["ship_error_class"], req.Context["ship_error_stage"], req.Context["ship_error_debug"]
	if req.Input.Active() {
		ec, _ := req.Input.ErrorContext() // zero ErrorContext when no upstream error
		code, class, stage, dbg = ec.Code, ec.Class, ec.Stage, ec.Debug
	}
	for _, f := range []struct{ k, v string }{
		{"ship_error_code", code},
		{"ship_error_class", class},
		{"ship_error_stage", stage},
		{"ship_error_debug", dbg},
	} {
		if f.v != "" {
			fmt.Fprintf(&b, "- %s: %s\n", f.k, f.v)
		}
	}
	return b.String()
}

func (hooks) Classify(_ string, req core.PhaseRequest, _ core.BridgeResponse) (string, []core.Diagnostic, string) {
	// Re-read the decision file from the workspace via the pure Classify so the
	// hook and the unit-tested function share one code path. The base runner's
	// `artifact` arg may be stdout, but the decision is a contracted FILE, so
	// we read it directly from the workspace.
	verdict, signals, diags := Classify(req.Workspace)
	return verdict, diags, nextPhaseFor(signals[signalAction])
}

// Config is the debugger phase constructor envelope.
type Config struct {
	Bridge  core.Bridge
	Prompts *prompts.Loader
	NowFn   func() time.Time
}

// Phase is the debugger phase runner. It wraps BaseRunner to additionally
// surface the recovery decision via PhaseResponse.Signals (BaseRunner does not
// populate Signals from Classify, but the orchestrator needs them).
type Phase struct {
	*runner.BaseRunner
}

// New constructs a debugger Phase.
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

// Run runs the base template, then enriches the response with the recovery
// signals (debugger.action, debugger.rerun_phase, debugger.root_cause) read
// from debug-decision.json, so the orchestrator's decideAfterDebugger can act
// on them. On the base-runner skip/error paths (empty workspace, bridge
// failure) the decision file is absent and Classify safe-defaults to BLOCK —
// the signals reflect that loudly rather than silently dropping out.
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
			Bridge:  bridge.NewDefault(req.ProjectRoot),
			Prompts: prompts.NewForProject(req.ProjectRoot),
		})
	})
}
