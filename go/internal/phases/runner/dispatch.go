package runner

import (
	"context"
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridgechain"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/log"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type phaseDispatchResult struct {
	bridgeResponse   core.BridgeResponse
	bridgeErr        error
	resolvedModel    string
	durationMS       int64
	worktreeVerified bool
	fenceDiagnostics []core.Diagnostic
	skills           []string
}

func (b *BaseRunner) dispatchPhaseAttempts(
	ctx context.Context,
	req core.PhaseRequest,
	prep phasePreparation,
	resolved phaseDispatchPlan,
) phaseDispatchResult {
	start := prep.start
	phase := prep.phase
	prompt := prep.prompt
	plan := resolved.plan
	overlayPolicy := resolved.overlayPolicy
	model := plan.Model

	fence := takeWorktreeFence(ctx, phase, req)

	var bres core.BridgeResponse
	var bridgeErr error
	var attemptLog, skills, walked []string
	var wall bridgechain.WallKeeper
	base := b.baseRequest(req, prep, resolved)
	tieredRes := llmroute.DispatchTiered(plan, func(candidateCLI, tier string) (int, error) {
		i := len(attemptLog)
		if i > 0 {
			log.Diag().Infof(
				"[runner] phase=%s fallback %d: trying cli=%s tier=%s (previous=%s exit=%d)\n",
				phase, i+1, candidateCLI, tier, attemptLog[i-1], bres.ExitCode)
		}
		skills = overlayPolicy.ResolveOverlays(overlayDispatchFor(req, phase, candidateCLI, tier))
		log.Diag().Infof("%s\n", FormatSkillOverlayLog(phase, skills, tier))
		attempt := base
		attempt.CLI, attempt.Model, attempt.Skills = candidateCLI, tier, skills
		bres, bridgeErr = b.bridge.Launch(ctx, attempt)
		wall.Observe(candidateCLI+"@"+tier, bres, bridgeErr)
		if err := b.eventsProducer(req.Workspace, phase, candidateCLI, req.Cycle, prompt); err != nil {
			log.Diag().Warnf("[runner] WARN events producer phase=%s cli=%s: %v (cost/classification degraded)\n", phase, candidateCLI, err)
		}
		attemptLog = append(attemptLog, fmt.Sprintf("%s@%s=%d", candidateCLI, tier, bres.ExitCode))
		walked = append(walked, candidateCLI)
		if bridgeErr != nil && bres.ExitCode == 85 {
			b.maybeBenchOnEscalation(bridgechain.Escalation{ProjectRoot: req.ProjectRoot, Workspace: req.Workspace, CLI: candidateCLI, DispatchStart: start, Env: req.Env})
		}
		return bres.ExitCode, bridgeErr
	}, func(from, to string) {
		log.Diag().Infof("[runner] phase=%s tier step-down: %s → %s (CLI chain exhausted at quota)\n", phase, from, to)
	})
	if wall.Surfaces(tieredRes, bres) {
		bres, bridgeErr = wall.Surface(tieredRes, bres, bridgeErr)
		log.Diag().Infof("[runner] phase=%s dispatch chain exhausted after a quota wall: surfacing exit %d: %v\n", phase, bres.ExitCode, bridgeErr)
	}
	if unlaunched := bridgechain.Unlaunched(tieredRes, plan); unlaunched != nil {
		bridgeErr = unlaunched
	}
	bridgeErr = withWalk(bridgeErr, walked)
	resolvedModel := terminalTier(tieredRes, model)
	if len(attemptLog) > 1 {
		log.Diag().Infof("[runner] phase=%s dispatch chain: %s\n", phase, joinAttempts(attemptLog))
	}
	durationMS := b.nowFn().Sub(start).Milliseconds()
	verified, fenceDiags := restoreWorktreeFence(context.WithoutCancel(ctx), phase, fence)
	req.WorktreeVerified = verified

	return phaseDispatchResult{
		bridgeResponse:   bres,
		bridgeErr:        bridgeErr,
		resolvedModel:    resolvedModel,
		durationMS:       durationMS,
		worktreeVerified: verified,
		fenceDiagnostics: fenceDiags,
		skills:           skills,
	}
}

func overlayDispatchFor(req core.PhaseRequest, phase, cli, tier string) policy.OverlayDispatch {
	d := policy.DispatchFromPhaseRequest(phase, cli, tier, tier)
	// core's one per-dispatch signal projection; the runner never re-reads the workspace.
	d.Signals = req.Signals
	d.WritesSource = req.WritesSource()
	return d
}

func terminalTier(walk llmroute.TieredDispatchResult, resolved string) string {
	if walk.Tier == "" {
		return resolved
	}
	return walk.Tier
}

func (b *BaseRunner) baseRequest(req core.PhaseRequest, prep phasePreparation, resolved phaseDispatchPlan) core.BridgeRequest {
	return core.BridgeRequest{
		Profile:             prep.profilePath,
		Prompt:              prep.prompt,
		Workspace:           req.Workspace,
		Worktree:            req.Worktree,
		RunID:               req.RunID,
		ProjectRoot:         req.ProjectRoot,
		ArtifactPath:        prep.artifactPath,
		SecondaryArtifacts:  secondaryArtifacts(b.hooks, req),
		Completion:          req.BridgeCompletion(),
		Agent:               prep.phase,
		Cycle:               req.Cycle,
		BudgetScale:         req.BudgetScale,
		RequireSandbox:      requiresExplanationSandbox(prep.phase, req),
		Env:                 req.Env,
		PermissionMode:      resolved.permissionMode,
		InteractivePolicy:   resolved.interactivePolicy,
		SystemPrompt:        resolved.systemPrompt,
		CorrectionDirective: req.CorrectionDirective,
		OperatorDirectives:  req.OperatorDirectives,
		ChainAttempt:        true,
	}
}

func joinAttempts(attempts []string) string {
	if len(attempts) == 0 {
		return ""
	}
	out := attempts[0]
	for _, a := range attempts[1:] {
		out += " -> " + a
	}
	return out
}

func FormatSkillOverlayLog(phase string, skills []string, tier string) string {
	return "[runner] phase=" + phase +
		" skill-overlays=[" + strings.Join(skills, ",") + "]" +
		" (tier=" + tier + ")"
}

func withWalk(err error, walked []string) error {
	if err == nil || len(walked) == 0 {
		return err
	}
	return core.WalkError{CLIs: walked, Err: err}
}
