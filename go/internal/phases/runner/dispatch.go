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

	// WS-G1: dispatch through the chain via llmroute.Dispatch — the SAME
	// chain-walk implementation the advisor uses (cycle-435,
	// [[never_duplicate_centralize_via_design_patterns]]), rather than a
	// hand-rolled copy of it. Each attempt: build BridgeRequest for the
	// candidate CLI, Launch, normalize events. On a trigger exit (default
	// {80, 81, 124, 127} per cli_chain.go:defaultFallbackOnExit —
	// REPL-boot-timeout / artifact-timeout / coreutils-timeout /
	// missing-binary) Dispatch advances to the next candidate. Any other exit
	// (or success) stops the walk — a legitimate FAIL verdict from a model
	// never silently routes to a different CLI. Final attempt's (bres,
	// bridgeErr) is what the rest of the function consumes; events file
	// reflects the final CLI's stdout so cycleclassify sees what actually
	// happened last.
	// Worktree fence (ADR-0097): a phase without write permission hands
	// downstream the exact tree it was given. Snapshot before the first
	// attempt, restore after the last — the classify hooks below (the audit's
	// explanation binding among them) must judge the builder's tree, not the
	// auditor's probes (cycles 1603-1605).
	fence := takeWorktreeFence(ctx, phase, req)

	var bres core.BridgeResponse
	var bridgeErr error
	var attemptLog []string
	// WS-876: dispatch through the TIER fallback chain. DispatchTiered walks
	// plan.Tiers outer × plan.Candidates inner: within a tier it behaves exactly
	// like Dispatch (trigger exit advances the CLI, a real FAIL stops), and it
	// steps DOWN to the next tier ONLY when every CLI at the current tier exited
	// 85 (quota) — the fable/opus→sonnet step-down the operator needs so a
	// fully-quota-walled top tier fails over to a lower-cost live tier instead of
	// aborting the phase. The tier string flows straight into BridgeRequest.Model:
	// the bridge realizer maps it per-CLI via the manifest's model_tier_map
	// (opus→opus/gpt-5.5, balanced→sonnet/gpt-5.4, …), so no runner-side model
	// resolution is needed — passing the tier is as literal as passing plan.Model.
	tieredRes := llmroute.DispatchTiered(plan, func(candidateCLI, tier string) (int, error) {
		i := len(attemptLog)
		if i > 0 {
			// Read the previous attempt from attemptLog, NOT plan.Candidates[i-1]:
			// under tiering i grows past len(Candidates) (candidates × tiers), so
			// indexing Candidates would panic on the first real step-down.
			log.Diag().Infof(
				"[runner] phase=%s fallback %d: trying cli=%s tier=%s (previous=%s exit=%d)\n",
				phase, i+1, candidateCLI, tier, attemptLog[i-1], bres.ExitCode)
		}
		// Skill overlays are resolved PER ATTEMPT: the fallback tier steps down
		// across attempts, and overlay rules key on tier (e.g. deep/top→fable), so
		// the configured skill set is recomputed for each (cli, tier) actually
		// dispatched. Pure policy lookup; the adapter materializes the SKILL.md.
		overlayDispatch := policy.DispatchFromPhaseRequest(phase, candidateCLI, tier, tier)
		// ADR-0099 slice 3: the objective signals the `when` selector reads are
		// core's projection (PhaseRequest.Signals, one digest per dispatch); the
		// runner copies, never re-reads the workspace.
		overlayDispatch.Signals = req.Signals
		overlaySkills := overlayPolicy.ResolveOverlays(overlayDispatch)
		// Observability: announce the resolved overlay set for THIS (cli, tier)
		// attempt so operators/graders see the persona fired without diffing the
		// prompt file. Rendered even for the empty set (skill-overlays=[]).
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
		})
		// Normalize per attempt so the final events file reflects the
		// final CLI's stdout — cycleclassify reads <phase>-events.ndjson
		// and we want it to describe what actually happened last.
		if err := b.eventsProducer(req.Workspace, phase, candidateCLI, req.Cycle, prompt); err != nil {
			log.Diag().Warnf("[runner] WARN events producer phase=%s cli=%s: %v (cost/classification degraded)\n", phase, candidateCLI, err)
		}
		attemptLog = append(attemptLog, fmt.Sprintf("%s@%s=%d", candidateCLI, tier, bres.ExitCode))
		// CLI-health bench: an exit-85 with a fresh benchable escalation
		// report (rate_limit class) is remembered ACROSS dispatches — run on
		// every candidate including the last, so the wall is recorded even
		// when no fallback remains (cycle-283). Staleness is judged against
		// the RUN start: the guard exists to exclude cross-PHASE leftovers in
		// the shared workspace, not earlier attempts of this same run.
		if bridgeErr != nil && bres.ExitCode == 85 {
			b.maybeBenchOnEscalation(req.ProjectRoot, req.Workspace, candidateCLI, start, req.Env)
		}
		return bres.ExitCode, bridgeErr
	}, func(from, to string) {
		log.Diag().Infof("[runner] phase=%s tier step-down: %s → %s (CLI chain exhausted at quota)\n", phase, from, to)
	})
	// ResolvedModel reports the tier the terminal attempt actually ran at (a
	// step-down means the phase ran below its resolved tier); fall back to the
	// resolved model for the empty-candidates edge case DispatchTiered guards.
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
