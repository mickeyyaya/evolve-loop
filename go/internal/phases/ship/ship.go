// Package ship implements the native Go commit-and-push phase as a
// core.PhaseRunner. Unlike the LLM phases, ship does not call a Bridge;
// it runs the native atomic shipper (native.go) directly — the
// successor to legacy/scripts/lifecycle/ship.sh, reproducing the full
// audit-binding / EGPS-gate / atomic commit+ff-merge+push state machine.
package ship

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const phaseName = string(core.PhaseShip)

// CmdRunner is the subprocess injection seam, an alias of sysexec.RunFunc.
type CmdRunner = sysexec.RunFunc

type Config struct {
	Runner CmdRunner
	NowFn  func() time.Time
	Sleep  func(time.Duration)
	// See ADR-0050.
	// PhaseIO threads the EVOLVE_PHASE_IO stage into the audit-binding verdict
	// parse. Zero value (StageOff) = byte-identical (prose parse); set by the
	// composition root (cmd_cycle.go) from cfg.PhaseIO.
	PhaseIO config.Stage
	// ManifestGate threads the policy.json `gates.manifest_gate` rollout dial
	// (policy.GatesConfig().ManifestGate) into Options.ManifestGate for the
	// ship-bind manifest reconciliation. Raw resolved stage string, not a
	// config.Stage, because manifest.go's seam is value-compared against
	// ManifestGateEnforce. Zero value "" = shadow (byte-identical).
	ManifestGate string
	// RepoContractGate threads policy.json gates.repo_contract_gate into the
	// ship-time repo-contract scanner pack (repocontract.go). Zero value ""
	// = off for construction-site safety; the cmd_cycle wiring test asserts
	// the production site threads the resolved default ("enforce") so the
	// gate cannot be silently left unwired.
	RepoContractGate string
	// See ADR-0103.
	// Signals is the root's Signal Center. The landing's ship.warning events
	// reach it through Options.Signals. The orchestrator root (cmd_cycle.go)
	// passes its Center; the `evolve phase ship` registry factory leaves it
	// nil (a subprocess root has no Center yet).
	Signals *signalcenter.Center
}

// Phase implements core.PhaseRunner for the ship stage.
type Phase struct {
	runner           CmdRunner
	nowFn            func() time.Time
	sleep            func(time.Duration)
	phaseIO          config.Stage
	manifestGate     string
	repoContractGate string
	signals          *signalcenter.Center
}

func New(c Config) *Phase {
	nowFn := c.NowFn
	if nowFn == nil {
		nowFn = time.Now
	}
	return &Phase{runner: c.Runner, nowFn: nowFn, sleep: c.Sleep, phaseIO: c.PhaseIO, manifestGate: c.ManifestGate, repoContractGate: c.RepoContractGate, signals: c.Signals}
}

func (p *Phase) Name() string { return phaseName }

// See ADR-0103.
// signalsWired reports whether the phase carries a Signal Center for the
// landing's warnings — the in-package wiring test's handle. Unexported: its
// only consumer is TestShipOptions_ThreadsSignals, and the composition root
// pins its own site by source scan.
func (p *Phase) signalsWired() bool { return p.signals != nil }

// defaultCommitMessage synthesizes a deterministic cycle commit message when
// the caller didn't supply Context["commit_message"]. Mirrors the shape
// `evolve cycle run` uses (cmd_cycle.go), plus the cycle number for traceable
// git history.
func defaultCommitMessage(req core.PhaseRequest) string {
	// See ADR-0099.
	// A document cycle lands under `solution(<slug>)` so the commit-prefix
	// vocabulary names the deliverable it carries (the kernel's own digest of
	// the triage header decides the kind; the triage decision names the
	// slugs). Code cycles keep the legacy message byte-identical.
	if core.DocumentCycle(req.Workspace) {
		if ids := core.BoundTaskIDs(req.Workspace); len(ids) > 0 {
			// One slug in the scope (the prefix grammar admits [a-z0-9-] only);
			// any further bound slugs ride in the subject line.
			msg := fmt.Sprintf("solution(%s): evolve-cycle %d", ids[0], req.Cycle)
			if len(ids) > 1 {
				msg += " (also " + strings.Join(ids[1:], ", ") + ")"
			}
			return msg
		}
	}
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
	// ship: the autonomous-loop construction path does not always populate this
	// Context key, and the manual `evolve ship` CLI always passes an explicit
	// message and is unaffected.
	// See ADR-0050.
	// Read commit_message from the typed envelope at enforce, the legacy
	// Context map otherwise (byte-identical — Active() is false unless
	// enforce); the empty→defaultCommitMessage fallback covers both.
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

// See ADR-0050.
// NewWithDefaultRunnerStage is NewWithDefaultRunner plus the EVOLVE_PHASE_IO
// stage. The composition root (cmd_cycle.go) passes cfg.PhaseIO so the
// audit-binding verdict parse is sentinel-first at >= StageEnforce.
// NewWithDefaultRunner stays as the StageOff (byte-identical) convenience.
func NewWithDefaultRunnerStage(stage config.Stage) *Phase {
	return New(Config{Runner: sysexec.DefaultRunner, PhaseIO: stage})
}

// shipOptions is the PhaseRequest → Options translation, extracted from
// runNative so the sole production construction site is directly testable.
func (p *Phase) shipOptions(req core.PhaseRequest, msg string) Options {
	return Options{
		Class:                           ClassCycle, // PhaseRunner only invokes for cycle commits
		CommitMessage:                   msg,
		ProjectRoot:                     req.ProjectRoot,
		WorkspacePath:                   req.Workspace, // See ADR-0049: run-scope ship's reads
		RunID:                           req.RunID,     // See ADR-0049: run-scope the audit binding
		CycleID:                         req.Cycle,
		AuditRound:                      req.AuditRound,
		ActiveWorktree:                  req.Worktree,
		WorktreeBaseSHA:                 req.WorktreeBaseSHA,
		ExplanationDocumentationVersion: req.ExplanationDocumentationVersion,
		BuildExplanation:                req.BuildExplanation,
		RequireBuildExplanationHandoff:  req.ExplanationDocumentationVersion != 0,
		PluginRoot:                      req.Env["EVOLVE_PLUGIN_ROOT"],
		Env:                             req.Env,
		PhaseIO:                         p.phaseIO, // See ADR-0050: sentinel-first verdict parse at enforce
		ManifestGate:                    p.manifestGate,
		Runner:                          p.runner,
		Sleep:                           p.sleep,
		Signals:                         p.signals, // See ADR-0103: the landing's ship.warning events reach the root's Center
	}
}

func (p *Phase) runNative(ctx context.Context, req core.PhaseRequest, msg string, start time.Time) (core.PhaseResponse, error) {
	opts := p.shipOptions(req, msg)
	res, resumed, err := resumeFirst(ctx, &opts)
	if !resumed && err == nil {
		if gerr := p.runRepoContractGate(ctx, req, &opts); gerr != nil {
			return core.PhaseResponse{}, gerr
		}
		res, err = runStages(ctx, &opts, res)
	}
	resp, rerr := respond(req, res, err)
	resp.DurationMS = p.nowFn().Sub(start).Milliseconds()
	return resp, rerr
}

func (p *Phase) runRepoContractGate(ctx context.Context, req core.PhaseRequest, opts *Options) error {
	gateRoot, baseRef, err := repoContractGateRoot(opts)
	if err != nil {
		return fmt.Errorf("ship repo-contract gate: %w", err)
	}
	if err := runRepoContractGateAt(ctx, p.repoContractGate, gateRoot, baseRef, req.Workspace, os.Stderr, backstopFlakeSignal(p.signals, req.Cycle)); err != nil {
		return fmt.Errorf("ship repo-contract gate: %w", err)
	}
	return nil
}

func respond(req core.PhaseRequest, res RunResult, err error) (core.PhaseResponse, error) {
	resp := core.PhaseResponse{Phase: phaseName, Verdict: core.VerdictFAIL, ArtifactsDir: req.Workspace}
	switch {
	case err != nil:
		resp.Signals = failureSignals(err, res)
		resp.Diagnostics = []core.Diagnostic{{Severity: "error", Message: err.Error()}}
		return resp, fmt.Errorf("ship: native: %w", err)
	case res.ExitCode != ExitOK:
		resp.Diagnostics = []core.Diagnostic{{Severity: "error", Message: strings.Join(res.Logs, "\n")}}
		return resp, fmt.Errorf("ship: native exit=%d", res.ExitCode)
	}
	resp.Verdict, resp.NextPhase, resp.CommitSHA = core.VerdictPASS, string(core.PhaseRetro), res.CommitSHA
	if res.RepairAttempted != "" {
		resp.Signals = map[string]any{}
		addRepairSignals(resp.Signals, res)
	}
	return resp, nil
}

func failureSignals(err error, res RunResult) map[string]any {
	signals := map[string]any{}
	if se, ok := core.AsShipError(err); ok {
		signals["ship.error_code"] = string(se.Code)
		signals["ship.error_class"] = string(se.Class)
		signals["ship.error_stage"] = string(se.Stage)
		signals["ship.debug"] = se.DebugString()
	}
	addRepairSignals(signals, res)
	return signals
}

// See ADR-0039.
// addRepairSignals mirrors the repair ladder's observability fields onto the
// generic signal plane — present on both PASS (self-healed) and FAIL
// (repair declined) responses.
func addRepairSignals(signals map[string]any, res RunResult) {
	if res.RepairAttempted != "" {
		signals["ship.repair_attempted"] = res.RepairAttempted
		signals["ship.repair_outcome"] = res.RepairOutcome
	}
}

// See ADR-0035, ADR-0038.
// init self-registers the ship phase factory with the phase registry, like
// every other built-in phase. The subprocess dispatcher (internal/cli/phasecmd)
// resolves phases by name and never constructs ship directly — keeping the
// flow phase-agnostic. The orchestrator's in-process path (cmd_cycle.go)
// builds ship with a stage-threaded constructor; this registry factory
// serves the `evolve phase ship` subprocess entrypoint with the default
// (StageOff) runner, matching the prior phasecmd wiring byte-for-byte.
func init() {
	registry.Register(string(core.PhaseShip), func(_ core.PhaseRequest) core.PhaseRunner {
		return NewWithDefaultRunner()
	})
}

// repoContractGateRoot is the gate's projection of landingTree — the tree the
// ship will land, tested against the base its changes are measured from: the
// worktree's base SHA when the ship lands from a worktree and knows it, else
// HEAD. An unresolvable typed worktree fails closed here with the same
// CodeWorktreeResolve atomicShip raises: the gate never tests the project
// root in the lane's stead and reports a misleading green.
func repoContractGateRoot(opts *Options) (root, baseRef string, err error) {
	tree, fromWorktree, err := landingTree(opts)
	if err != nil {
		return "", "", err
	}
	if fromWorktree && opts.WorktreeBaseSHA != "" {
		return tree, opts.WorktreeBaseSHA, nil
	}
	return tree, "HEAD", nil
}
