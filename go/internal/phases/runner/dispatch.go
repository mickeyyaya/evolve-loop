package runner

import (
	"context"
	"fmt"

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
	artifactPath := prep.artifactPath
	profilePath := prep.profilePath
	plan := resolved.plan
	overlayPolicy := resolved.overlayPolicy
	permissionMode := resolved.permissionMode
	interactivePolicy := resolved.interactivePolicy
	sysPrompt := resolved.systemPrompt
	model := plan.Model

	fence := takeWorktreeFence(ctx, phase, req)

	var bres core.BridgeResponse
	var bridgeErr error
	var attemptLog []string
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
		bres, bridgeErr = b.bridge.Launch(ctx, core.BridgeRequest{
			CLI:                 candidateCLI,
			Profile:             profilePath,
			Model:               tier,
			Prompt:              prompt,
			Workspace:           req.Workspace,
			Worktree:            req.Worktree,
			RunID:               req.RunID,
			ProjectRoot:         req.ProjectRoot,
			ArtifactPath:        artifactPath,
			SecondaryArtifacts:  secondaryArtifacts(b.hooks, req),
			Agent:               phase,
			Cycle:               req.Cycle,
			BudgetScale:         req.BudgetScale,
			RequireSandbox:      requiresExplanationSandbox(phase, req),
			Env:                 req.Env,
			PermissionMode:      permissionMode,
			InteractivePolicy:   interactivePolicy,
			SystemPrompt:        sysPrompt,
			Skills:              overlaySkills,
			CorrectionDirective: req.CorrectionDirective,
			OperatorDirectives:  req.OperatorDirectives,
			ChainAttempt:        true, // one attempt of this walk, so a chain-walking handle passes it through
		})
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
	// The terminal attempt's tier, below the resolved one after a step-down; empty only without candidates.
	resolvedModel := tieredRes.Tier
	if resolvedModel == "" {
		resolvedModel = model
	}
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
