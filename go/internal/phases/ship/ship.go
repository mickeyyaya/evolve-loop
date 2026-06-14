// Package ship implements the native Go commit-and-push phase as a
// core.PhaseRunner. Unlike the LLM phases, ship does not call a Bridge;
// it runs the native atomic shipper (native.go) directly — the
// successor to legacy/scripts/lifecycle/ship.sh, reproducing the full
// audit-binding / EGPS-gate / atomic commit+ff-merge+push state machine.
package ship

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

const phaseName = string(core.PhaseShip)

// CmdRunner is the subprocess injection seam. Tests override it; the
// production wiring uses execRunner.
type CmdRunner func(ctx context.Context, name string, args, env []string, cwd string,
	stdin io.Reader, stdout, stderr io.Writer) (exitCode int, err error)

type Config struct {
	Runner CmdRunner
	NowFn  func() time.Time
}

// Phase implements core.PhaseRunner for the ship stage.
type Phase struct {
	runner CmdRunner
	nowFn  func() time.Time
}

func New(c Config) *Phase {
	nowFn := c.NowFn
	if nowFn == nil {
		nowFn = time.Now
	}
	return &Phase{runner: c.Runner, nowFn: nowFn}
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
	msg := req.Context["commit_message"]
	if msg == "" {
		msg = defaultCommitMessage(req)
	}

	return p.runNative(ctx, req, msg, start)
}

// execRunner is the production CmdRunner.
func execRunner(ctx context.Context, name string, args, env []string, cwd string,
	stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = env
	cmd.Dir = cwd
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), nil
		}
		return -1, err
	}
	return 0, nil
}

// NewWithDefaultRunner is a convenience constructor for production
// wiring that uses exec.CommandContext.
func NewWithDefaultRunner() *Phase {
	return New(Config{Runner: execRunner})
}

// runNative dispatches to the native Go ship implementation. Translates
// PhaseRequest → Options, then RunResult → PhaseResponse.
func (p *Phase) runNative(ctx context.Context, req core.PhaseRequest, msg string, start time.Time) (core.PhaseResponse, error) {
	opts := Options{
		Class:         ClassCycle, // PhaseRunner only invokes for cycle commits
		CommitMessage: msg,
		ProjectRoot:   req.ProjectRoot,
		WorkspacePath: req.Workspace, // ADR-0049 S3 / G3: run-scope ship's reads
		PluginRoot:    req.Env["EVOLVE_PLUGIN_ROOT"],
		Env:           req.Env,
		Runner:        execRunner,
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
