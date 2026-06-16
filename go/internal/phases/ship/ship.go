// Package ship implements the native Go commit-and-push phase as a
// core.PhaseRunner. Unlike the LLM phases, ship does not call a Bridge;
// it runs the native atomic shipper (native.go) directly — the
// successor to legacy/scripts/lifecycle/ship.sh, reproducing the full
// audit-binding / EGPS-gate / atomic commit+ff-merge+push state machine.
package ship

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const phaseName = string(core.PhaseShip)

// CmdRunner is the subprocess injection seam. It is an alias of the single
// canonical command-execution seam (sysexec.RunFunc); production wiring uses
// sysexec.DefaultRunner, tests inject fixtures.FakeExec. The alias keeps the
// `ship.CmdRunner` name at call sites while collapsing onto the one seam.
type CmdRunner = sysexec.RunFunc

type Config struct {
	Runner CmdRunner
	NowFn  func() time.Time
	// PhaseIO threads the EVOLVE_PHASE_IO stage into the audit-binding verdict
	// parse (ADR-0050 §3.10 Slice 6). Zero value (StageOff) = byte-identical
	// (prose parse). Set by the composition root (cmd_cycle.go) from cfg.PhaseIO.
	PhaseIO config.Stage
}

// Phase implements core.PhaseRunner for the ship stage.
type Phase struct {
	runner  CmdRunner
	nowFn   func() time.Time
	phaseIO config.Stage
}

func New(c Config) *Phase {
	nowFn := c.NowFn
	if nowFn == nil {
		nowFn = time.Now
	}
	return &Phase{runner: c.Runner, nowFn: nowFn, phaseIO: c.PhaseIO}
}

func (p *Phase) Name() string { return phaseName }

// defaultCommitMessage synthesizes a deterministic cycle commit message when
// the caller didn't supply Context["commit_message"]. Mirrors the shape
// `evolve cycle run` uses (cmd_cycle.go), plus the cycle number for traceable
// git history.
func defaultCommitMessage(req core.PhaseRequest) string {
	if req.GoalHash != "" {
		return fmt.Sprintf("evolve-cycle %d: goal=%s", req.Cycle, req.GoalHash)
	}
	return fmt.Sprintf("evolve-cycle %d", req.Cycle)
}

func (p *Phase) Run(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	start := p.nowFn()
	if p.runner == nil {
		return core.PhaseResponse{}, fmt.Errorf("ship: runner required")
	}
	// An explicit Context["commit_message"] always wins. When absent, synthesize
	// a deterministic message from the cycle identity rather than failing the
	// ship (single seam, cycle-147 lesson): `evolve cycle run` populates this
	// Context key but the autonomous-loop construction path (cmd_loop.go
	// buildCycleContext) does not, so erroring here silently sank EVERY loop
	// cycle at the ship step once the audit gate finally let cycles reach ship
	// (cycle-150). Defaulting here covers all current and future callers; the
	// manual `evolve ship` CLI always passes an explicit message and is
	// unaffected.
	// ADR-0050 §3.10 Slice 4: read commit_message from the typed envelope at
	// enforce, the legacy Context map below it (byte-identical — Active() is false
	// unless enforce). The empty→defaultCommitMessage fallback below covers both
	// paths, so an active envelope with no commit message still synthesizes one.
	msg := req.Context["commit_message"]
	if req.Input.Active() {
		msg = req.Input.CycleInputs().CommitMessage()
	}
	if msg == "" {
		msg = defaultCommitMessage(req)
	}

	return p.runNative(ctx, req, msg, start)
}

// NewWithDefaultRunner is a convenience constructor for production
// wiring that uses sysexec.DefaultRunner (exec.CommandContext).
func NewWithDefaultRunner() *Phase {
	return NewWithDefaultRunnerStage(config.StageOff)
}

// NewWithDefaultRunnerStage is NewWithDefaultRunner plus the EVOLVE_PHASE_IO stage
// (ADR-0050 §3.10 Slice 6). The composition root (cmd_cycle.go) passes cfg.PhaseIO
// so the audit-binding verdict parse is sentinel-first at >= StageEnforce.
// NewWithDefaultRunner stays as the StageOff (byte-identical) convenience.
func NewWithDefaultRunnerStage(stage config.Stage) *Phase {
	return New(Config{Runner: sysexec.DefaultRunner, PhaseIO: stage})
}

// runNative dispatches to the native Go ship implementation. Translates
// PhaseRequest → Options, then RunResult → PhaseResponse.
func (p *Phase) runNative(ctx context.Context, req core.PhaseRequest, msg string, start time.Time) (core.PhaseResponse, error) {
	opts := Options{
		Class:         ClassCycle, // PhaseRunner only invokes for cycle commits
		CommitMessage: msg,
		ProjectRoot:   req.ProjectRoot,
		WorkspacePath: req.Workspace, // ADR-0049 S3 / G3: run-scope ship's reads
		RunID:         req.RunID,     // ADR-0049 S4 / G5: run-scope the audit binding
		PluginRoot:    req.Env["EVOLVE_PLUGIN_ROOT"],
		Env:           req.Env,
		PhaseIO:       p.phaseIO, // ADR-0050 §3.10 Slice 6: sentinel-first verdict parse at enforce
		Runner:        sysexec.DefaultRunner,
	}
	res, err := Run(ctx, opts)
	durationMS := p.nowFn().Sub(start).Milliseconds()

	if err != nil {
		// Boundary preservation: wrap with %w so core.AsShipError still
		// recovers the structured *core.ShipError from the orchestrator side,
		// and surface its Code/Class/Debug as PhaseResponse.Signals so the
		// debugger-routing layer can act on them without re-parsing strings.
		signals := map[string]any{}
		if se, ok := core.AsShipError(err); ok {
			signals["ship.error_code"] = string(se.Code)
			signals["ship.error_class"] = string(se.Class)
			signals["ship.error_stage"] = string(se.Stage)
			signals["ship.debug"] = se.DebugString()
		}
		addRepairSignals(signals, res)
		return core.PhaseResponse{
			Phase:        phaseName,
			Verdict:      core.VerdictFAIL,
			ArtifactsDir: req.Workspace,
			DurationMS:   durationMS,
			Signals:      signals,
			Diagnostics: []core.Diagnostic{
				{Severity: "error", Message: err.Error()},
			},
		}, fmt.Errorf("ship: native: %w", err)
	}
	if res.ExitCode != ExitOK {
		return core.PhaseResponse{
			Phase:        phaseName,
			Verdict:      core.VerdictFAIL,
			ArtifactsDir: req.Workspace,
			DurationMS:   durationMS,
			Diagnostics: []core.Diagnostic{
				{Severity: "error", Message: strings.Join(res.Logs, "\n")},
			},
		}, fmt.Errorf("ship: native exit=%d", res.ExitCode)
	}
	// Signals stays nil on a repair-free success — byte-identical to the
	// pre-ladder response shape.
	var signals map[string]any
	if res.RepairAttempted != "" {
		signals = map[string]any{}
		addRepairSignals(signals, res)
	}
	return core.PhaseResponse{
		Phase:        phaseName,
		Verdict:      core.VerdictPASS,
		ArtifactsDir: req.Workspace,
		NextPhase:    string(core.PhaseRetro),
		CommitSHA:    res.CommitSHA,
		DurationMS:   durationMS,
		Signals:      signals,
	}, nil
}

// addRepairSignals mirrors the repair ladder's observability fields
// (ADR-0039 §8) onto the generic signal plane — present on both PASS
// (self-healed) and FAIL (repair declined) responses.
func addRepairSignals(signals map[string]any, res RunResult) {
	if res.RepairAttempted != "" {
		signals["ship.repair_attempted"] = res.RepairAttempted
		signals["ship.repair_outcome"] = res.RepairOutcome
	}
}
