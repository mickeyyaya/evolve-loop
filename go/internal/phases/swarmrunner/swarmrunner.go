// Package swarmrunner is the orchestrator seam for the swarm harness (ADR-0032):
// a Decorator that wraps any core.PhaseRunner and, when the swarm stage is
// active and the planner produced a partitionable plan, dispatches the phase
// across N parallel workers instead of running it as a single agent.
//
// It is the ONE piece that turns the (otherwise inert) swarm building blocks
// into a live, orchestrator-managed multi-subagent phase. The orchestrator's
// dispatch call site is unchanged — it still calls runner.Run; this Decorator
// is wired in at the composition root (cmd/evolve/cmd_cycle.go) wrapping the
// build (writer) and scout (reader) runners.
//
// Rollout via Config.Stage (sourced from policy.json "swarm.stage"):
//   - off/shadow (default): delegate to the inner runner BYTE-FOR-BYTE. Zero risk.
//   - advisory: Dispatch runs LIVE (proving the parallel path + guaranteed
//     teardown) but the inner runner still produces the authoritative result;
//     swarm outcome is attached to Signals for inspection.
//   - enforce: the swarm result IS the phase output (writer: merge-train into the
//     integration branch; reader: synthesized summary).
//
// Safety: any missing/invalid/non-partitionable plan, or a writer plan that
// isn't provably disjoint, collapses to the inner single-writer runner (N=1).
package swarmrunner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/swarm"
)

type stage int

const (
	stageOff stage = iota
	stageAdvisory
	stageEnforce
)

// Config holds the swarm dispatch configuration sourced from policy.SwarmConfig().
// Both fields carry the same defaults as the removed env readers: Stage="shadow"
// (maps to stageOff, the byte-identical delegate path) and PortBase=0
// (swarm.DefaultPortBase sentinel).
type Config struct {
	Stage    string
	PortBase int
	// WorktreeBase is the resolved policy.json worktree.base ("" ⇒ default
	// <root>/.evolve/worktrees), threaded to the worker provisioner. Replaces
	// the former EVOLVE_WORKTREE_BASE env read (flag-reduction, ADR-0064).
	WorktreeBase string
}

// Decorator wraps a phase runner with swarm dispatch. Construct via New.
type Decorator struct {
	inner       core.PhaseRunner
	bridge      core.Bridge
	mode        swarm.Mode
	concurrency int
	cfg         Config
}

// New returns a Decorator wrapping inner for the given mode (writer phases like
// build, reader phases like scout). bridge is adapted to a swarm.Launcher.
// cfg carries the policy-resolved swarm stage and port base (see policy.SwarmConfig).
func New(inner core.PhaseRunner, bridge core.Bridge, mode swarm.Mode, cfg Config) *Decorator {
	return &Decorator{inner: inner, bridge: bridge, mode: mode, concurrency: 2, cfg: cfg}
}

// Name implements core.PhaseRunner (transparent — the orchestrator sees the
// inner phase name).
func (d *Decorator) Name() string { return d.inner.Name() }

// Run implements core.PhaseRunner.
func (d *Decorator) Run(ctx context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	st := parseSwarmStage(d.cfg.Stage)
	if st == stageOff {
		return d.inner.Run(ctx, req) // shadow/off → byte-identical delegate
	}
	plan, ok := loadPlan(req.Workspace)
	if !ok {
		return d.inner.Run(ctx, req) // no usable swarm-plan.json → N=1
	}
	plan.Mode = d.mode // the PHASE decides writer vs reader, not the planner
	if v := swarm.Validate(plan); v.Collapse {
		return d.inner.Run(ctx, req) // not partitionable / not disjoint → N=1
	}

	if st == stageAdvisory {
		// Advisory: the inner runner stays AUTHORITATIVE, so run it FIRST on the
		// full caller ctx — the live dispatch (a side-experiment proving the
		// parallel path + teardown) must never delay or deadline-starve inner
		// (go-reviewer HIGH: a long dispatch on a tight orchestrator ctx would
		// otherwise fail an inner run that shadow never fails). Dispatch then runs
		// on a detached context so its own latency/cancellation can't corrupt the
		// authoritative result; only its observations are annotated.
		resp, err := d.inner.Run(ctx, req)
		sr, derr := swarm.Dispatch(context.WithoutCancel(ctx), plan, swarm.DispatchRequest{
			ProjectRoot: req.ProjectRoot, Cycle: req.Cycle, Workspace: req.Workspace,
		}, d.dispatchDeps(req))
		annotate(&resp, plan, sr, derr, "advisory")
		return resp, err
	}

	// Enforce: the swarm result IS the phase output.
	sr, derr := swarm.Dispatch(ctx, plan, swarm.DispatchRequest{
		ProjectRoot: req.ProjectRoot, Cycle: req.Cycle, Workspace: req.Workspace,
	}, d.dispatchDeps(req))
	return d.enforce(ctx, req, plan, sr, derr) // enforce: the swarm result is the output
}

// dispatchDeps assembles the injected ports. Writers get a worktree provisioner;
// readers do not (overlap is harmless, no isolation needed). linkGuardDeps is
// nil here — wiring core's (unexported) guard-dep symlinker into advisory/enforce
// writer worktrees is a documented follow-up; it only matters for the opt-in
// live writer path, never for the default shadow delegate.
func (d *Decorator) dispatchDeps(req core.PhaseRequest) swarm.Deps {
	manifestPath := filepath.Join(req.Workspace, ".swarm", "sessions.json")
	reg := swarm.NewSessionRegistry(manifestPath, req.Cycle, d.inner.Name(), os.Getpid())
	deps := swarm.Deps{
		Launcher:    bridgeLauncher{bridge: d.bridge, env: req.Env},
		Registry:    reg,
		Killer:      swarm.ExecSessionKiller{KillGroup: groupKiller, KillTmux: swarm.ExecTmuxKill},
		Concurrency: d.concurrency,
	}
	if d.mode == swarm.ModeWriter {
		deps.Provisioner = swarm.NewGitWorkerProvisioner(nil, d.cfg.WorktreeBase)
		deps.PortBase = d.cfg.PortBase // 0 → swarm.DefaultPortBase
	}
	return deps
}

// enforce reduces the swarm to ONE authoritative PhaseResponse: writers run the
// serialized merge-train into the integration branch; readers synthesize. Any
// worker failure or merge failure → FAIL.
func (d *Decorator) enforce(ctx context.Context, req core.PhaseRequest, plan swarm.SwarmPlan, sr swarm.SwarmResult, derr error) (core.PhaseResponse, error) {
	resp := core.PhaseResponse{
		Phase:   d.inner.Name(),
		Verdict: core.VerdictPASS,
		CostUSD: sr.TotalCostUSD(),
		Tokens:  sr.TotalTokens(),
		Signals: map[string]any{},
	}
	annotate(&resp, plan, sr, derr, "enforce")

	if derr != nil || !sr.AllOK() {
		resp.Verdict = core.VerdictFAIL
		return resp, derr
	}

	if d.mode == swarm.ModeWriter {
		mr := swarm.RunMergeTrain(ctx, plan.IntegrationBranch, sr.MergeOrder, branchByID(plan),
			swarm.MergeTrainDeps{Merger: swarm.ExecGitMerger{IntegrationWorktree: sr.IntegrationWorktree}})
		resp.Signals["swarm.merged"] = mr.AllMerged
		if !mr.AllMerged {
			resp.Verdict = core.VerdictFAIL
			return resp, fmt.Errorf("swarm merge-train failed for %s", d.inner.Name())
		}
		resp.ArtifactsDir = sr.IntegrationWorktree
	}

	if d.mode == swarm.ModeReader {
		// Reader fan-in: fold each worker's report into ONE synthesized artifact at
		// the phase's canonical report path (<phase>-report.md), so downstream
		// phases read a single document instead of N reports scattered across
		// worker workspaces. Readers do no git merge — overlap is harmless.
		// sr.Workers is in plan order (results[i] = wr, one goroutine per index),
		// so synthesis section order is deterministic across runs — do not sort.
		order := make([]string, len(sr.Workers))
		artifactByID := make(map[string]string, len(sr.Workers))
		for i, w := range sr.Workers {
			order[i] = w.WorkerID
			body, rerr := os.ReadFile(w.ArtifactPath)
			if rerr != nil {
				// A worker that exited 0 should have written its report; surface the
				// gap in the synthesis rather than silently dropping it (Rule 12).
				artifactByID[w.WorkerID] = fmt.Sprintf("[no artifact at %s: %v]", w.ArtifactPath, rerr)
				continue
			}
			artifactByID[w.WorkerID] = string(body)
		}
		synthPath := filepath.Join(req.Workspace, d.inner.Name()+"-report.md")
		if werr := os.WriteFile(synthPath, []byte(swarm.Synthesize(order, artifactByID)), 0o644); werr != nil {
			resp.Verdict = core.VerdictFAIL
			return resp, fmt.Errorf("swarm reader synthesis write %s: %w", synthPath, werr)
		}
		resp.ArtifactsDir = req.Workspace
		resp.Signals["swarm.synthesis"] = synthPath
	}
	return resp, nil
}

// annotate records the swarm outcome on the response Signals bus (namespaced).
func annotate(resp *core.PhaseResponse, plan swarm.SwarmPlan, sr swarm.SwarmResult, derr error, stageName string) {
	if resp.Signals == nil {
		resp.Signals = map[string]any{}
	}
	resp.Signals["swarm.stage"] = stageName
	resp.Signals["swarm.mode"] = string(plan.Mode)
	resp.Signals["swarm.worker_count"] = len(sr.Workers)
	resp.Signals["swarm.all_ok"] = sr.AllOK()
	if derr != nil {
		resp.Signals["swarm.error"] = derr.Error()
	}
}

func branchByID(plan swarm.SwarmPlan) map[string]string {
	out := make(map[string]string, len(plan.Workers))
	for _, w := range plan.Workers {
		out[w.WorkerID] = w.Branch
	}
	return out
}

// parseSwarmStage parses a stage string (from policy.SwarmConfig().Stage).
// Unknown/empty/off/shadow → stageOff (byte-identical delegate).
func parseSwarmStage(s string) stage {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "advisory":
		return stageAdvisory
	case "enforce":
		return stageEnforce
	default:
		return stageOff
	}
}

// loadPlan reads swarm-plan.json (or swarm-plan.md with a fenced block) from the
// workspace. Returns ok=false on any read/parse failure so the caller falls back
// to N=1 — a swarm is never forced when the plan is unusable.
func loadPlan(workspace string) (swarm.SwarmPlan, bool) {
	for _, name := range []string{"swarm-plan.json", "swarm-plan.md"} {
		data, err := os.ReadFile(filepath.Join(workspace, name))
		if err != nil {
			continue
		}
		if plan, perr := swarm.ParsePlan(string(data)); perr == nil {
			return plan, true
		}
	}
	return swarm.SwarmPlan{}, false
}

// bridgeLauncher adapts core.Bridge to swarm.Launcher (accept-interface-where-
// used). It forwards the swarm-controlled SessionName so the dispatcher can
// PRE-REGISTER a deterministic tmux session before launch (orphan-on-cancel
// hardening) — core.BridgeResponse returns no session identity, but it no longer
// needs to: the dispatcher already knows the name it pinned. The crash-safe
// manifest sweep (`evolve swarm reap`) remains the backstop for a hard parent kill.
type bridgeLauncher struct {
	bridge core.Bridge
	env    map[string]string
}

func (l bridgeLauncher) Launch(ctx context.Context, r swarm.LaunchRequest) (swarm.LaunchResult, error) {
	resp, err := l.bridge.Launch(ctx, core.BridgeRequest{
		CLI: r.CLI, Model: r.Model, Profile: r.Profile, Agent: r.Agent,
		SessionName: r.SessionName, Prompt: r.Prompt,
		Workspace: r.Workspace, Worktree: r.Worktree, ProjectRoot: r.ProjectRoot,
		ArtifactPath: r.ArtifactPath, Cycle: r.Cycle, Env: mergeEnv(l.env, r.Env),
	})
	if err != nil {
		return swarm.LaunchResult{}, err
	}
	return swarm.LaunchResult{ExitCode: resp.ExitCode, CostUSD: resp.CostUSD, Tokens: resp.Tokens}, nil
}

// mergeEnv overlays per-worker env over the shared phase env (overlay wins),
// ALWAYS returning a fresh map. The shared phase-env (l.env) is owned by the
// orchestrator and handed to every concurrent worker, so we never return it
// aliased — the bridge is external code that may store or mutate what it's given.
func mergeEnv(base, overlay map[string]string) map[string]string {
	out := make(map[string]string, len(base)+len(overlay))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range overlay {
		out[k] = v
	}
	return out
}
