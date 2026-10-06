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
}

// dispatchPhaseAttempts owns one worktree fence around the complete CLI/tier
// fallback chain. It restores the fence before returning any result to
// reconciliation or classification.
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
	var attemptLog []string
	var wall bridgechain.WallKeeper
	base := b.baseRequest(req, prep, resolved)
	// The tier is passed as the model; the bridge maps it per CLI, so the runner resolves no model itself.
	tieredRes := llmroute.DispatchTiered(plan, func(candidateCLI, tier string) (int, error) {
		i := len(attemptLog)
		if i > 0 {
			// attemptLog, not Candidates: under tiering i outgrows Candidates, so indexing it panics on a step-down.
			log.Diag().Infof(
				"[runner] phase=%s fallback %d: trying cli=%s tier=%s (previous=%s exit=%d)\n",
				phase, i+1, candidateCLI, tier, attemptLog[i-1], bres.ExitCode)
		}
		// Overlays resolve per attempt because overlay rules key on the tier, which steps down across attempts.
		overlayDispatch := policy.DispatchFromPhaseRequest(phase, candidateCLI, tier, tier)
		// core's one per-dispatch signal projection; the runner never re-reads the workspace.
		overlayDispatch.Signals = req.Signals
		overlaySkills := overlayPolicy.ResolveOverlays(overlayDispatch)
		log.Diag().Infof("%s\n", FormatSkillOverlayLog(phase, overlaySkills, tier))
		attempt := base
		attempt.CLI, attempt.Model, attempt.Skills = candidateCLI, tier, overlaySkills
		bres, bridgeErr = b.bridge.Launch(ctx, attempt)
		wall.Observe(candidateCLI+"@"+tier, bres, bridgeErr)
		// Per attempt, so the events file cycleclassify reads describes the last CLI that ran.
		if err := b.eventsProducer(req.Workspace, phase, candidateCLI, req.Cycle, prompt); err != nil {
			log.Diag().Warnf("[runner] WARN events producer phase=%s cli=%s: %v (cost/classification degraded)\n", phase, candidateCLI, err)
		}
		attemptLog = append(attemptLog, fmt.Sprintf("%s@%s=%d", candidateCLI, tier, bres.ExitCode))
		// Every candidate, the last included, so a wall is benched even with no fallback left. Staleness counts
		// from the run start: the guard excludes other phases' leftovers, not this run's earlier attempts.
		if bridgeErr != nil && bres.ExitCode == 85 {
			b.maybeBenchOnEscalation(req.ProjectRoot, req.Workspace, candidateCLI, start, req.Env)
		}
		return bres.ExitCode, bridgeErr
	}, func(from, to string) {
		log.Diag().Infof("[runner] phase=%s tier step-down: %s → %s (CLI chain exhausted at quota)\n", phase, from, to)
	})
	if wall.Surfaces(tieredRes, bres) {
		log.Diag().Infof("[runner] phase=%s dispatch chain exhausted after a quota wall: surfacing the wall over the last rung's exit %d, so the cycle defers\n", phase, bres.ExitCode)
		bres, bridgeErr = wall.Surface(tieredRes, bres, bridgeErr)
	}
	if unlaunched := bridgechain.Unlaunched(tieredRes, plan); unlaunched != nil {
		bridgeErr = unlaunched
	}
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
	}
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
