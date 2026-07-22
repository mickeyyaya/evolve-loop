package core

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/backfill"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards/treediff"
)

// dispatch runs the per-phase runner lookup, the pre-phase cycle-state write +
// tree-diff snapshot, the phase-request build, and the whole inner attempt loop
// (self-heal retries, backfill, optional-infra-skip, ship-error recovery)
// extracted behavior-preserving from RunCycle.
//
// Returns:
//   - loopAbort + error: missing built-in/mandatory runner, pre-phase state
//     write failure, or an exhausted/non-recoverable phase failure.
//   - loopContinue: a non-dispatchable optional USER phase skipped (cr.current
//     advanced), or a ShipError routed to a recovery phase (cr.scheduledNext set).
//   - loopNext + a populated dispatchResult: the phase produced a usable verdict;
//     reviewAndGuard/recordAndBranch consume the dispatchResult.
func (cr *cycleRun) dispatch(next Phase) (dispatchResult, loopAction, error) {
	runner, ok := cr.o.runners[next]
	if !ok {
		// The routing surface (registry order + catalog) can know phases
		// the dispatch surface cannot run (cycle-265: registry-listed
		// `memo` had no .evolve/phases config ⇒ no specrunner; the static
		// order walked into it post-ship and killed a PASSING batch). A
		// non-dispatchable OPTIONAL USER phase is skipped loudly and the
		// walk continues; a missing BUILT-IN or configured-mandatory
		// runner stays fatal — that is a wiring bug, not routing-surface
		// drift (built-ins always have factories; only registry/user
		// phases can be known to routing yet unregistered).
		if next.IsValid() || isConfiguredMandatory(cr.o.cfg, string(next)) {
			return dispatchResult{}, loopAbort, fmt.Errorf("%w: no runner registered for phase %s", ErrPhaseInvalid, next)
		}
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase %s selected but no runner is registered — skipping (register a runner or remove it from the registry order)\n", next)
		cr.current = next
		return dispatchResult{}, loopContinue, nil
	}

	phaseStarted := cr.o.now().UTC()
	cr.cs.Phase = string(next)
	cr.cs.PhaseStartedAt = phaseStarted.Format(time.RFC3339)
	cr.cs.ActiveAgent = string(next)
	if next == PhaseAudit {
		// A (re-)dispatch supersedes any prior attempt's diagnosed-FAIL
		// explanation (ship-error recovery re-audits, debugger RERUN_PHASE):
		// stale reasons must never mark a later, differently-caused FAIL as
		// diagnosed to the ADR-0072 floor. Cleared BEFORE the pre-phase
		// cycle-state write below so the cleared state is also what persists.
		resetFloorFailReason(&cr.cs, next)
	}
	if err := cr.o.storage.WriteCycleState(cr.ctx, cr.cs); err != nil {
		return dispatchResult{}, loopAbort, fmt.Errorf("write cycle-state pre-%s: %w", next, err)
	}
	if next == PhaseRetro {
		// S1 assembler, verdict path (ADR-0074 I2; cycle-1046 live gap): the
		// failure digest must exist BEFORE the retro agent runs — it is the
		// identity the disposition gate cross-checks and the blocker breaker
		// reads. Idempotent with the phase-error path in recordFailureLearning.
		reason := fmt.Sprintf("phase %s verdict FAIL routed to retro (agent-graded; see the %s report artifact)", cr.current, cr.current)
		if d := verdictFailDistinguisher(cr.cs.WorkspacePath); d != "" {
			// Fold per-failure content into the fingerprint input so distinct
			// failures never collide (cycles 1054/1060 false-identical pin).
			reason += " " + d
		}
		cr.o.ensureFailureDigest(cr.cycle, cr.req.ProjectRoot, cr.cs.WorkspacePath, string(cr.current), reason)
	}

	// CB.1 (concurrency campaign W4): EVERY phase runs with cwd = the cycle
	// worktree — not just the source writers (tdd/build, role-gate-permitted)
	// and audit (issue #9: its verification commands must inspect the
	// builder's pending work). A read-only phase's cwd in the main tree let
	// stray writes and guard misfires land in the live checkout (cycle-280);
	// with the worktree provisioned at cycle start, no phase subprocess
	// touches main at all. cwd is NOT write permission: the write axis
	// (role-gate / tree-diff guard / normalize) still keys off worktreePhase.
	// Empty when provisioning failed — the pre-existing degraded mode.
	phaseWorktree := cr.cs.ActiveWorktree
	// Workstream B: snapshot the main-tree dirty set BEFORE a source-
	// writing phase runs. After it runs we re-snapshot and compare —
	// any newly-dirty MAIN-tree path is a leak that escaped the bridge
	// sandbox (each git worktree is a separate working dir, so its
	// writes don't show up here). The treediff package owns the
	// snapshot/check + SnapshotMissed semantics; the orchestrator just
	// threads it through. Skipped entirely for non-worktree phases.
	var (
		treeGuard      *treediff.Guard
		beforeDirty    []string
		snapshotFailed bool
	)
	if cr.o.gitDirtyPaths != nil {
		treeGuard = treediff.New(cr.o.gitDirtyPaths)
		snap, err := treeGuard.Snapshot(cr.ctx, cr.req.ProjectRoot)
		if err != nil {
			snapshotFailed = true
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN tree-diff pre-phase snapshot failed for %s: %v (sandbox guard degraded; post-phase leak check skipped)\n", next, err)
		} else {
			beforeDirty = snap
		}
	}
	phaseCtx := cr.ctxSnap
	if next == PhaseRetro {
		phaseCtx = make(map[string]string, len(cr.ctxSnap)+1)
		for k, v := range cr.ctxSnap {
			phaseCtx[k] = v
		}
		phaseCtx["previous_verdict"] = cr.lastVerdict
	}
	phaseReq := PhaseRequest{
		Cycle:         cr.cycle,
		ProjectRoot:   cr.req.ProjectRoot,
		Workspace:     cr.cs.WorkspacePath,
		Worktree:      phaseWorktree,
		RunID:         cr.cs.RunID,
		GoalHash:      cr.req.GoalHash,
		PreviousPhase: string(cr.current),
		Env:           cr.envSnap,
		Context:       phaseCtx,
		BypassPolicy:  cr.req.BypassPolicy,
		// Runtime operator directives snapshotted once at cycle start (same value
		// for every phase this cycle); empty ⇒ byte-identical dispatch.
		OperatorDirectives: cr.directivesSet.Merged,
	}
	// MR4(c): project this phase's clamped {cli,tier} plan proposal onto the
	// dispatched request ONLY under model_routing=auto — the mode gate that
	// distinguishes "auto applies" from "advisory logs, never applies" (the
	// clamp itself already ran, and was persisted to phase-plan.json, for both
	// modes in planCycle). A nil cr.clampedPlan (advisor outage) or no
	// matching/proposing entry for this phase leaves both fields empty — the
	// degrade-to-profile-static floor (I4).
	if cr.o.cfg.ModelRouting == config.ModelRoutingAuto && cr.clampedPlan != nil {
		for _, e := range cr.clampedPlan.Entries {
			if e.Phase == string(next) {
				phaseReq.ModelRoutingCLI = e.CLI
				phaseReq.ModelRoutingTier = e.Tier
				break
			}
		}
	}
	// ADR-0076 D (deterministic escalation floor — applied AFTER the mode-
	// gated projection and INDEPENDENT of it, review finding D1: the live
	// registry runs model_routing=static and a policy floor must still fire):
	// a retried scoped item raises the build dispatch tier to deep, clamped
	// through the same envelope guardrail as the routing clamp.
	if next == PhaseBuild {
		if tier, raised := cr.escalatedBuildTier(phaseReq.ModelRoutingTier); raised {
			phaseReq.ModelRoutingTier = tier
		}
	}
	// ADR-0076 slice A: the build dispatch carries the cycle's difficulty
	// multiplier so the engine can stretch the artifact-wait deadline. Scale
	// 1.0 is left unset — byte-identical legacy dispatch.
	if next == PhaseBuild {
		if scale := cr.buildBudgetScale(); scale != 1.0 {
			phaseReq.BudgetScale = scale
		}
	}
	// ADR-0050 Phase 3.7: at advisory+, serve the build phase's upstream
	// build-plan via the typed envelope (read once here at the seam) instead of
	// an ad-hoc disk read inside the phase. Off/shadow leave it empty → the phase
	// reads disk as before (byte-identical dispatch).
	phaseReq.BuildPlan = readUpstreamBuildPlan(cr.o.cfg.PhaseIO, next, cr.workflowConfig.PhaseEnables, cr.cs.WorkspacePath)
	// ADR-0050 Phase 3.4 (SHADOW) + Phase 3.10 (ENFORCE input). When
	// EVOLVE_PHASE_IO>=shadow, assemble the typed Upstream view from the same
	// upstream this phase is about to receive, compare it to the legacy routing
	// digest, and record any divergence (shadow artifact + ledger). At >=enforce,
	// the same pass also returns the authoritative typed PhaseInput the phase
	// consumes in place of the legacy Context map. At EVOLVE_PHASE_IO=off (default)
	// this is skipped entirely and phaseReq.Input stays the zero value —
	// byte-identical dispatch; below enforce the assembled Input is still zero, so
	// only the flip to enforce changes what a phase observes.
	if cr.o.cfg.PhaseIO >= config.StageShadow {
		phaseReq.Input = cr.assemblePhaseIO(next, phaseWorktree, phaseCtx)
	}
	// Cycle-122 Fix 3 / ADR-0030: attach the per-phase observer
	// goroutine BEFORE runner.Run and cancel it AFTER. noopObserver
	// (default when WithObserver wasn't used) is byte-identical to
	// the pre-fix cycle. Real implementations spawn a stall detector
	// that watches <workspace>/<agent>-stdout.log and emits stall
	// events to <workspace>/<agent>-observer-events.ndjson.
	// Self-heal (Fix D): a bridge ArtifactTimeout (exit=81) is the
	// recoverable "agent produced no artifact within the wait window" case
	// — a stalled launch where a fresh relaunch usually succeeds. Retry the
	// phase a bounded number of times on THAT sentinel only; every other
	// error (and exhaustion of the budget) aborts the cycle as before. A
	// deterministic timeout (e.g. a misconfigured agent) simply fails again
	// and aborts after the cap — at most one wasted retry. The observer is
	// (re)started per attempt so each launch is watched.
	var resp PhaseResponse
	var err error
	// shipRecovered marks that a ShipError was intercepted and routed to a
	// recovery phase instead of aborting; the caller continues the outer loop
	// (skipping verdict/ledger handling for the failed ship).
	shipRecovered := false
	maxAttempts := cr.retryConfig.PhaseMaxAttempts
	var attemptCount int
	// attemptExits collects each failed attempt's bridge exit code so the
	// exhaustion arm can recognize the all-families quota-terminal signature
	// (every attempt exit=85 — cycle-656) and checkpoint-and-defer instead of
	// failing forward.
	var attemptExits []int
	for attempt := 1; ; attempt++ {
		attemptCount = attempt
		obsCancel := cr.o.observer.Start(cr.ctx, string(next), phaseReq)
		resp, err = runner.Run(cr.ctx, phaseReq)
		if obsCancel != nil {
			obsCancel()
		}
		if err == nil && IsVerdict(resp.Verdict) {
			// Self-healing trail: a bridge artifact-wait timeout (exit 81) was
			// reconciled by the runner against a well-formed, gate-passing
			// deliverable, so this phase ships on the agent's own verdict
			// instead of a synthesized FAIL. Record it (mirrors the backfill
			// entry) so the recovery is auditable, never silent.
			if resp.Reconciled {
				if lerr := cr.o.ledger.Append(cr.ctx, LedgerEntry{
					TS:       cr.o.now().UTC().Format(time.RFC3339),
					Cycle:    cr.cycle,
					Role:     string(next),
					Kind:     "reconciled_timeout",
					ExitCode: 81,
				}); lerr != nil {
					fmt.Fprintf(os.Stderr, "[orchestrator] WARN reconciled_timeout ledger append: %v\n", lerr)
				}
			}
			break
		}
		if err != nil {
			attemptExits = append(attemptExits, bridgeExitCode(err))
			if attempt >= maxAttempts || (!errors.Is(err, ErrArtifactTimeout) && !isTransientBridgeError(err)) {
				// All-families quota exhaustion (cycle-656 D2): every attempt
				// returned exit=85, so the cross-family failover (cycle-393)
				// has no remaining target — quota is a resource that resets in
				// hours, not a per-attempt transient. Spending more attempts
				// (or degrading an optional phase to WARN and advancing the
				// next LLM phase into the same wall) guarantees a FAIL plus a
				// quota-consuming retro. Instead: write a quota-likely
				// checkpoint (resumeFromPhase = this phase, completed phases +
				// worktree preserved) and abort with the typed sentinel so the
				// loop stops resumable (rc=5) and classification is DEFERRED,
				// not FAILED. Checked FIRST in the arm — before backfill
				// (disjoint: exit-81-only) and before optionalInfraSkip, which
				// would otherwise fail forward. Single-family 85 with a healthy
				// sibling never reaches here all-85 (the sibling attempt's exit
				// differs), so normal failover is unchanged.
				if allFamiliesQuotaExhausted(attemptExits) {
					phaseErr := fmt.Errorf("phase %s: %w: every family in the fallback chain returned exit=85 across %d attempts; checkpoint written — resume with `evolve loop --resume` after quota reset", next, ErrAllFamiliesExhausted, attempt)
					fmt.Fprintf(os.Stderr, "[orchestrator] WARN %v\n", phaseErr)
					if QuotaBoundaryCheckpointer != nil {
						if cperr := QuotaBoundaryCheckpointer(cr.cs, cr.req.ProjectRoot, cr.o.now()); cperr != nil {
							fmt.Fprintf(os.Stderr, "[orchestrator] WARN quota-boundary checkpoint write failed: %v (defer still recorded; resume may re-run completed phases)\n", cperr)
						}
					}
					if lerr := cr.o.ledger.Append(cr.ctx, LedgerEntry{
						TS:       cr.o.now().UTC().Format(time.RFC3339),
						Cycle:    cr.cycle,
						Role:     string(next),
						Kind:     "all_families_exhausted",
						ExitCode: 85,
					}); lerr != nil {
						fmt.Fprintf(os.Stderr, "[orchestrator] WARN all_families_exhausted ledger append: %v\n", lerr)
					}
					// ADR-0044 C1: record the abort reason with the DEFERRED
					// prefix so cyclehealth classifies the cycle DEFERRED.
					cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, resp, attempt,
						fmt.Sprintf("%s: %s", abortReasonAllFamiliesExhausted, phaseErr.Error()), cr.cs.PhaseStartedAt))
					writePhaseFailureDiag(cr.cs.WorkspacePath, string(next), cr.cycle, phaseErr, attempt, cr.o.now)
					cr.recordFailureLearning(next, phaseErr, attempt)
					return dispatchResult{}, loopAbort, wrapCycleLevelError(next, phaseErr)
				}
				// Backfill: when exhaustion is specifically due to ErrArtifactTimeout,
				// try to reconstruct the artifact from stdout.clean.txt before aborting.
				// Default-on; policy.json can disable artifact backfill for the cycle.
				backfillEnabled := cr.workflowConfig.BackfillEnabled
				if attempt >= maxAttempts && errors.Is(err, ErrArtifactTimeout) && backfillEnabled {
					artifactPath := backfillArtifactPath(cr.cs.WorkspacePath, string(next))
					if ok, berr := backfill.TryExtract(cr.cs.WorkspacePath, string(next), artifactPath, 200); berr != nil {
						fmt.Fprintf(os.Stderr, "[orchestrator] WARN backfill %s: %v\n", next, berr)
					} else if ok {
						fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase %s: ErrArtifactTimeout exhausted; backfilled artifact from stdout.clean.txt; proceeding with WARN verdict\n", next)
						resp = PhaseResponse{Phase: string(next), Verdict: VerdictWARN, ArtifactsDir: cr.cs.WorkspacePath}
						if lerr := cr.o.ledger.Append(cr.ctx, LedgerEntry{
							TS:       cr.o.now().UTC().Format(time.RFC3339),
							Cycle:    cr.cycle,
							Role:     string(next),
							Kind:     "backfill",
							ExitCode: 81,
						}); lerr != nil {
							fmt.Fprintf(os.Stderr, "[orchestrator] WARN backfill ledger append: %v\n", lerr)
						}
						break
					}
				}
				// Optional-phase infra skip (Workstream-D intent on
				// ErrArtifactTimeout; cycle-283): an enrichment phase must not
				// veto completed spine work. When backfill could not reconstruct
				// the artifact, a catalog-Optional, non-floor phase whose
				// exhaustion is infra-shaped degrades to a synthesized WARN and
				// the cycle advances toward audit/ship. The failed attempts stay
				// in failure-learning and the ledger — recovered, never silent.
				if cr.o.optionalInfraSkip(next, err) {
					fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase %s: optional phase exhausted infra retries (%v); degrading to WARN and advancing (optional_infra_skip)\n", next, err)
					cr.recordFailureLearning(next, fmt.Errorf("phase %s: %w", next, err), attempt)
					if lerr := cr.o.ledger.Append(cr.ctx, LedgerEntry{
						TS:       cr.o.now().UTC().Format(time.RFC3339),
						Cycle:    cr.cycle,
						Role:     string(next),
						Kind:     "optional_infra_skip",
						ExitCode: bridgeExitCode(err),
					}); lerr != nil {
						fmt.Fprintf(os.Stderr, "[orchestrator] WARN optional_infra_skip ledger append: %v\n", lerr)
					}
					resp = PhaseResponse{Phase: string(next), Verdict: VerdictWARN, ArtifactsDir: cr.cs.WorkspacePath}
					break
				}
				// Ship-error recovery seam (Component #7): ship is a pure
				// executor — a structured ShipError is resolved by the advisor's
				// recovery chain (Strategy + CoR), not by aborting the cycle. The
				// resolver records the error, picks the recovery phase
				// (re-audit / retry-ship / debugger), and bounds the depth.
				// Integrity breaches, an illegal edge, or exhausted depth return
				// (_, false) and fall through to the loud abort below.
				if se, ok := AsShipError(err); ok {
					// Preserve the worktree from the exit cleanup while a
					// ship failure is unresolved (ADR-0039 §8 / D10) —
					// cleared when a later ship attempt succeeds.
					cr.preserveWorktree = true
					fleetWidth := fleetWidthFromEnv(cr.req.Env)
					if rec, recovering := cr.o.recoverFromShipError(cr.ctx, cr.cycle, cr.cs, se, cr.recoveryDepth, fleetWidth); recovering {
						cr.ctxSnap["ship_error_code"] = string(se.Code)
						cr.ctxSnap["ship_error_class"] = string(se.Class)
						cr.ctxSnap["ship_error_stage"] = string(se.Stage)
						cr.ctxSnap["ship_error_debug"] = se.DebugString()
						// ADR-0044 C1: the failed ship attempt ran and burned
						// budget — record it before routing to recovery. A
						// later successful ship records its own outcome.
						cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, resp, attemptCount,
							fmt.Sprintf("ship error %s: recovering via %s (attempt %d/%d)", se.Code, rec, cr.recoveryDepth+1, shipRecoveryBudget(se.Code, fleetWidth)), cr.cs.PhaseStartedAt))
						cr.recoveryDepth++
						cr.scheduledNext = rec
						cr.current = PhaseShip // ship ran (and failed); keep forensics accurate
						shipRecovered = true
						break
					}
				}
				// Post-ship observer skip (cycle-574 memo-phase-tier-envelope):
				// a best-effort RoleControl observer (memo / post-ship-monitor)
				// that fails AFTER a healthy ship must not turn a shipped cycle
				// abnormal. Unlike optionalInfraSkip this fires on ANY error
				// shape (the memo tier/envelope error is a policy error, not
				// infra), gated on ship having already landed and the same
				// floor/mandatory guards so it can never weaken the integrity
				// floor. Degrade to a synthesized WARN and advance; the failed
				// attempts stay in failure-learning and the ledger — recovered,
				// never silent.
				if cr.o.postShipObserverSkip(next, cr.shipped) {
					fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase %s: best-effort post-ship observer failed after ship (%v); degrading to WARN and advancing (post_ship_observer_skip)\n", next, err)
					cr.recordFailureLearning(next, fmt.Errorf("phase %s: %w", next, err), attempt)
					if lerr := cr.o.ledger.Append(cr.ctx, LedgerEntry{
						TS:       cr.o.now().UTC().Format(time.RFC3339),
						Cycle:    cr.cycle,
						Role:     string(next),
						Kind:     "post_ship_observer_skip",
						ExitCode: bridgeExitCode(err),
					}); lerr != nil {
						fmt.Fprintf(os.Stderr, "[orchestrator] WARN post_ship_observer_skip ledger append: %v\n", lerr)
					}
					resp = PhaseResponse{Phase: string(next), Verdict: VerdictWARN, ArtifactsDir: cr.cs.WorkspacePath}
					break
				}
				phaseErr := fmt.Errorf("phase %s: %w", next, err)
				// ADR-0044 C1: record the dispatch outcome BEFORE the
				// failure-learning retro so the timing record stays
				// chronological (failed phase, then retro). No canonical
				// agent verdict exists on this path → synthesized FAIL.
				cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, resp, attempt, phaseErr.Error(), cr.cs.PhaseStartedAt))
				writePhaseFailureDiag(cr.cs.WorkspacePath, string(next), cr.cycle, err, attempt, cr.o.now)
				// ADR-0044 C3: enforce-only, best-effort — classify the
				// unclassified pane via the LLM tail and promote, so the
				// NEXT occurrence is deterministic. Never alters the abort.
				cr.o.adviseOnUnclassifiedFailure(cr.ctx, cr.cycle, cr.cs.WorkspacePath, cr.req.ProjectRoot, next, err, cr.envSnap)
				cr.recordFailureLearning(next, phaseErr, attempt)
				return dispatchResult{}, loopAbort, wrapCycleLevelError(next, phaseErr)
			}
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase %s attempt %d/%d hit a transient bridge error or timeout; relaunching (self-heal)\n", next, attempt, maxAttempts)
			// Emit structured audit trail for the self-heal retry.
			if lerr := cr.o.ledger.Append(cr.ctx, LedgerEntry{
				TS:       cr.o.now().UTC().Format(time.RFC3339),
				Cycle:    cr.cycle,
				Role:     string(next),
				Kind:     "phase_retry",
				ExitCode: bridgeExitCode(err),
			}); lerr != nil {
				fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase_retry ledger append: %v\n", lerr)
			}
			executeRetryBackoff(attempt, cr.retryConfig.RetryBackoffBaseS)
			continue
		}
		if err == nil && !IsVerdict(resp.Verdict) {
			if attempt >= maxAttempts {
				// Cycle-802 Task 3 (contract-exhaustion-degrades-non-floor,
				// subsumes advisory-phase-contract-degrade): an unparseable
				// verdict after retries exhausted is cycle-fatal ONLY for a
				// floor/ship phase. A non-floor post-verdict phase degrades to
				// SKIPPED+WARN and the cycle advances — recordFinalVerdict then
				// records the degrade into SkippedPhases without clobbering the
				// floor verdict, closing the same storm from the contract side.
				if degraded, ok := cr.o.nonFloorExhaustionDegrade(next, cr.cs.WorkspacePath, cr.o.floorAlreadyCompleted(cr.cs.CompletedPhases)); ok {
					fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase %s exhausted retries with non-canonical verdict %q; non-floor phase degrading to SKIPPED and advancing (contract_exhaustion_skip)\n", next, resp.Verdict)
					cr.recordFailureLearning(next, fmt.Errorf("phase %s: non-canonical verdict %q after %d attempts", next, resp.Verdict, attempt), attempt)
					if lerr := cr.o.ledger.Append(cr.ctx, LedgerEntry{
						TS:       cr.o.now().UTC().Format(time.RFC3339),
						Cycle:    cr.cycle,
						Role:     string(next),
						Kind:     "contract_exhaustion_skip",
						ExitCode: 0,
					}); lerr != nil {
						fmt.Fprintf(os.Stderr, "[orchestrator] WARN contract_exhaustion_skip ledger append: %v\n", lerr)
					}
					resp = degraded
					break
				}
				ferr := fmt.Errorf("phase %s returned non-canonical verdict %q", next, resp.Verdict)
				// ADR-0044 C1: a non-canonical verdict is never recorded
				// raw and never upgraded — phaseOutcomeFrom synthesizes FAIL.
				cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, resp, attempt, ferr.Error(), cr.cs.PhaseStartedAt))
				writePhaseFailureDiag(cr.cs.WorkspacePath, string(next), cr.cycle, ferr, attempt, cr.o.now)
				cr.recordFailureLearning(next, ferr, attempt)
				return dispatchResult{}, loopAbort, wrapCycleLevelError(next, ferr)
			}
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase %s attempt %d/%d returned non-canonical verdict %q; relaunching\n", next, attempt, maxAttempts, resp.Verdict)
			// Emit structured audit trail for the self-heal retry.
			if lerr := cr.o.ledger.Append(cr.ctx, LedgerEntry{
				TS:       cr.o.now().UTC().Format(time.RFC3339),
				Cycle:    cr.cycle,
				Role:     string(next),
				Kind:     "phase_retry",
				ExitCode: 0,
			}); lerr != nil {
				fmt.Fprintf(os.Stderr, "[orchestrator] WARN phase_retry ledger append: %v\n", lerr)
			}
			executeRetryBackoff(attempt, cr.retryConfig.RetryBackoffBaseS)
			continue
		}
	}
	if shipRecovered {
		return dispatchResult{}, loopContinue, nil // run the recovery phase (scheduledNext) next iteration
	}

	return dispatchResult{
		resp:           resp,
		attemptCount:   attemptCount,
		phaseWorktree:  phaseWorktree,
		treeGuard:      treeGuard,
		beforeDirty:    beforeDirty,
		snapshotFailed: snapshotFailed,
		runner:         runner,
		phaseReq:       phaseReq,
	}, loopNext, nil
}
