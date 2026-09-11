package core

import (
	"fmt"
	"os"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
)

// reviewWithCorrections owns the bounded contract-review ladder. Every loop
// iteration consumes one rung budget or terminates, and temporary routing and
// correction directives are restored before the method returns.
func (cr *cycleRun) reviewWithCorrections(next Phase, dr *dispatchResult) (loopAction, error) {
	// Workstream E2: per-phase deliverable review gate. Runs ONLY for
	// non-SKIPPED verdicts (a SKIPPED phase produced no deliverable to
	// review) and BEFORE the tree-diff guard + ledger append, so a reject
	// aborts the cycle without recording the phase as a success. The
	// default reviewer is noopReviewer (every phase approved) so opt-out
	// is byte-identical to pre-E2. On reject the correction loop below
	// re-dispatches up to the configured correction limit before aborting.
	if cr.o.reviewer != nil && dr.resp.Verdict != VerdictSKIPPED {
		rin := ReviewInput{
			Cycle:                           cr.cycle,
			RunID:                           cr.cs.RunID,
			ExplanationDocumentationVersion: cr.cs.ExplanationDocumentationVersion,
			Phase:                           string(next),
			WorktreeBaseSHA:                 cr.cs.WorktreeBaseSHA,
			Response:                        dr.resp,
			Workspace:                       cr.cs.WorkspacePath,
			Worktree:                        dr.phaseWorktree,
			ProjectRoot:                     cr.req.ProjectRoot,
		}
		// baseRoutingCLI is the routing CLI this phase was DISPATCHED with (the
		// advisor overlay under model_routing=auto; empty otherwise). The
		// contract-block escalation below temporarily overrides it for a
		// re-dispatch and this value is restored when the ladder ends, so the
		// phase's own routing is never mutated (scoping constraint 1 in
		// contract_escalation.go).
		baseRoutingCLI := dr.phaseReq.ModelRoutingCLI
		// escalated latches once a contract-block escalation has re-pointed a
		// re-dispatch, so the demotion WARN can state truthfully whether
		// escalation was tried (it is a no-op for a phase whose whole chain is
		// one CLI family).
		escalated := false
		// salvageRetried latches once the no-escalation-target remedy (scoping
		// constraint 5) has turned a correction into a structured re-prompt, so
		// the demotion record can distinguish "a remedy was tried and failed"
		// from "no remedy was possible". Disjoint from escalated by construction:
		// the remedy fires only where contractEscalationCLI found no target.
		salvageRetried := false
		// prevBlockIdentity is the defect identity (violation-code set +
		// normalized reason) of the block that
		// triggered the previous correction on THIS ladder ("" ⇒ none seen yet,
		// e.g. a breaker left hot by an earlier cycle). It gates the escalation
		// trigger on defect identity, not just block count. The zero value
		// means no block seen yet.
		var prevBlockIdentity contractBlockIdentity
		rr := cr.reviewDeliverable(next, rin, contractDispatch{cli: baseRoutingCLI})
		// Contract-correction retry: on a deliverable-contract reject,
		// re-dispatch the phase with the violation injected as a
		// "## Correction" directive (bounded by policy, default 2). This re-runs
		// runner.Run directly (no
		// bridge-timeout retry on corrections — see the design's scope note).
		maxCorrections := cr.correctionLimitFor(next, cr.retryConfig.ContractCorrectionRetries)
		// ADR-0045 I1: a correction re-dispatch is an interaction — every
		// rung of ONE correction decision shares a DecisionID, and each
		// re-dispatch records an outcome resolved by its verdict + the
		// re-review. The I2 ladder's salvage/live-fix rungs will join
		// this same decision when they ship.
		irec := interaction.NewRecorder(cr.cs.WorkspacePath)
		decisionID := ""
		if !rr.Approve && (maxCorrections > 0 || cr.o.contractVerifier != nil) {
			decisionID = fmt.Sprintf("%s-c%d-%d", next, cr.cycle, cr.o.now().UnixNano())
		}
		// ADR-0045 I2: graduated correction ladder. The DECISION is the
		// pure interaction.NextCorrection CoR (salvage → live_fix →
		// redispatch, cheapest first); EXECUTION is stage-gated here.
		// Salvage gets budget only when a breaker-neutral verifier is
		// wired. Rung 2 (live_fix) is decision-complete but
		// execution-dormant at v1: the orchestrator does not yet request
		// named sessions, so NamedREPL is hard-false until the session
		// request + reaper plumbing lands (the C1→C3 deferred-unification
		// precedent; see interaction/correction.go).
		rungBudget := map[string]int{
			interaction.RungSalvage:    1,
			interaction.RungLiveFix:    1,
			interaction.RungRedispatch: maxCorrections,
		}
		if cr.o.contractVerifier == nil {
			rungBudget[interaction.RungSalvage] = 0
		}
		corr := 0
		salvagedFromInvalid := "" // found-but-invalid origin → rung-3 kernel evidence
		for !rr.Approve {
			act := interaction.NextCorrection(interaction.CorrectionInput{
				Phase:      string(next),
				Workspace:  cr.cs.WorkspacePath,
				Worktree:   dr.phaseWorktree,
				Violation:  rr.Reason,
				NamedREPL:  false, // v1: no named-session request plumbing yet
				Busy:       false,
				DecisionID: decisionID,
				RungBudget: rungBudget,
			})
			if act.Rung == "" {
				break // ladder exhausted → abort below, exactly as today
			}
			rungBudget[act.Rung]-- // every iteration spends budget: the loop is finite
			if act.Rung == interaction.RungSalvage {
				salvEv := interaction.Event{
					Kind:       interaction.KindSalvage,
					Phase:      string(next),
					Cycle:      cr.cycle,
					Trigger:    "contract_reject",
					Rung:       interaction.RungSalvage,
					DecisionID: decisionID,
					Payload:    rr.Reason,
				}
				if cr.o.cfg.PhaseRecovery != config.StageEnforce {
					fmt.Fprintf(os.Stderr, "[orchestrator] phase %s: would-salvage misplaced deliverable (%s; EVOLVE_PHASE_RECOVERY=%s)\n", next, act.Reason, cr.o.cfg.PhaseRecovery)
					irec.Record(interaction.Outcome{Event: salvEv, Result: interaction.ResultWouldAct})
					continue
				}
				salvStart := cr.o.now()
				sr := cr.o.salvageDeliverable(cr.ctx, rin)
				switch {
				case sr.Relocated && sr.Verified:
					// Never-upgrades-verdict: the relocated artifact faces
					// the SAME gate — the breaker-touching FINAL outcome.
					rr = cr.reviewDeliverable(next, rin, contractDispatch{cli: dr.phaseReq.ModelRoutingCLI, escalated: escalated, salvageRetried: salvageRetried})
					res := interaction.ResultRejectedAgain
					if rr.Approve {
						res = interaction.ResultAccepted
					}
					fmt.Fprintf(os.Stderr, "[orchestrator] phase %s: salvaged %s → contracted path (gate approve=%v)\n", next, sr.From, rr.Approve)
					irec.Record(interaction.Outcome{Event: salvEv, Result: res, LatencyMS: cr.o.now().Sub(salvStart).Milliseconds()})
				case sr.Relocated:
					salvagedFromInvalid = sr.From
					fmt.Fprintf(os.Stderr, "[orchestrator] phase %s: salvage relocated %s but the destination failed verification — falling through\n", next, sr.From)
					irec.Record(interaction.Outcome{Event: salvEv, Result: interaction.ResultFoundButInvalid, LatencyMS: cr.o.now().Sub(salvStart).Milliseconds()})
				default:
					fmt.Fprintf(os.Stderr, "[orchestrator] phase %s: nothing to salvage (%s)\n", next, sr.Reason)
					irec.Record(interaction.Outcome{Event: salvEv, Result: interaction.ResultNotFound, LatencyMS: cr.o.now().Sub(salvStart).Milliseconds()})
				}
				continue
			}
			if act.Rung == interaction.RungLiveFix {
				// Unreachable at v1 (NamedREPL hard-false). Termination
				// invariant for when the named-session plumbing lands:
				// the decrement above already spent this rung's budget,
				// and live_fix never mutates rr — so the `continue` is
				// safe under `for !rr.Approve` and the ladder stays
				// finite (every iteration spends budget or breaks).
				continue
			}
			corr++
			fmt.Fprintf(os.Stderr, "[orchestrator] phase %s: contract violation (correction %d/%d) — re-dispatching with correction: %s\n",
				next, corr, maxCorrections, rr.Reason)
			// CLI escalation (inbox contract-block-cli-escalation): from the
			// correction answering the SECOND consecutive block, re-dispatch on
			// a different CLI FAMILY instead of asking the CLI that just
			// mis-formatted the deliverable twice to try a third time. Scoped to
			// THIS re-dispatch only — the profile and the phase's own routing are
			// untouched, and the soft overlay keeps the original chain behind the
			// escalated primary. No target (already on the universal family with
			// no other family configured) ⇒ the correction becomes a structured
			// re-prompt on the SAME routing instead (scoping constraint 5) — the
			// ladder no longer falls through unremedied there.
			// The count alone is not the signature: the block must also be the
			// SAME defect as the one that triggered the previous correction
			// (contractBlocksShareIdentity, scoping constraint 4) — two different
			// violations are two honest defects, not an incapable CLI.
			salvageRetry := false
			// Escalation eligibility has two typed doors, one per rejection class:
			//   - rr.Blocks: the deliverable contract gate's own breaker count.
			//   - rr.Remediation + the local correction ordinal: a
			//     remediation-carrying rejection (set ONLY by evalgate) whose
			//     second identical round is starting. The evalgate keeps no
			//     breaker, so the in-cycle ordinal is its only honest counter —
			//     and constraint 2's desync warning is about the DELIVERABLE
			//     breaker's cross-cycle file, which this class does not have.
			//     Measured basis (2026-08-23): every eval-materialization
			//     failure since cycle-1450 (1471/1476/1504/1531/1540/1545) was
			//     one CLI family, 0-for-all correction rounds on that family,
			//     including rounds whose directive named the exact writable
			//     paths — for the CREATE-a-missing-artifact class, a different
			//     CLI is demonstrably the remedy. Remediation-LESS Blocks==0
			//     rejections (topngate/triagecap/build floor) keep constraint 3
			//     exactly: they never enter here.
			escalEligible := rr.Blocks >= contractEscalateAtBlock ||
				(rr.Remediation != "" && corr >= contractEscalateAtBlock)
			if escalEligible && contractBlocksShareIdentity(prevBlockIdentity, rr.Reason) {
				failedCLI := cr.contractDispatchCLI(next, dr.phaseReq.ModelRoutingCLI)
				esc, ok := cr.contractEscalationCLI(next, dr.phaseReq.ModelRoutingCLI)
				switch {
				case ok && esc != dr.phaseReq.ModelRoutingCLI:
					blocksForLog := rr.Blocks
					if blocksForLog == 0 {
						blocksForLog = corr // the evalgate door: its ordinal IS the count
					}
					fmt.Fprintf(os.Stderr, "[orchestrator] phase %s: CLI ESCALATION — %d consecutive identical block(s) on cli=%s; correction %d re-dispatches on cli=%s (these rejections never trigger cli_fallback, which is how an incapable CLI used to burn every round). The phase's primary routing is unchanged.\n",
						next, blocksForLog, failedCLI, corr, esc)
					dr.phaseReq.ModelRoutingCLI = esc
					escalated = true
				case !ok:
					// The two remedies are disjoint: this arm is reached only when
					// there is genuinely no other family to escalate to, so one
					// block's round-2 budget is never spent on both.
					fmt.Fprintf(os.Stderr, "[orchestrator] phase %s: CONTRACT SALVAGE RETRY — %d consecutive contract blocks on cli=%s and no other CLI family is available to escalate to; correction %d re-dispatches on the SAME cli with a structured re-prompt carrying the validator output verbatim. Breaker-neutral: no extra dispatch, no extra correction, no routing change.\n",
						next, rr.Blocks, failedCLI, corr)
					salvageRetry = true
					salvageRetried = true
				}
			}
			// Carry THIS block's identity to the next iteration: identity is
			// compared against the immediately preceding block, matching the
			// blocker breaker's own consecutive-identical-fingerprint semantics.
			prevBlockIdentity = newContractBlockIdentity(rr.Reason)
			if lerr := cr.o.ledger.Append(cr.ctx, LedgerEntry{
				TS:       cr.o.now().UTC().Format(time.RFC3339),
				Cycle:    cr.cycle,
				Role:     string(next),
				Kind:     "contract_correction",
				ExitCode: 0,
			}); lerr != nil {
				fmt.Fprintf(os.Stderr, "[orchestrator] WARN contract_correction ledger append: %v\n", lerr)
			}
			// Payload carries the violation that TRIGGERED this dispatch;
			// a rejected_again outcome's NEW violation appears as the next
			// iteration's payload (trigger semantics, not result semantics).
			corrEv := interaction.Event{
				Kind:       interaction.KindCorrectionRedispatch,
				Phase:      string(next),
				Cycle:      cr.cycle,
				Trigger:    "contract_reject",
				Rung:       "redispatch",
				DecisionID: decisionID,
				Payload:    rr.Reason,
			}
			corrStart := cr.o.now()
			recordCorrection := func(res string) {
				irec.Record(interaction.Outcome{
					Event:     corrEv,
					Result:    res,
					LatencyMS: cr.o.now().Sub(corrStart).Milliseconds(),
					CostUSD:   dr.resp.CostUSD,
				})
			}
			directive := composeCorrection(rr.Reason, rr.Remediation)
			if salvageRetry {
				directive = composeContractSalvageRetry(rr.Reason, rr.Remediation)
			}
			if cr.o.cfg.PhaseRecovery == config.StageEnforce {
				// Evidence-enriched re-dispatch (I2 rung 3): kernel-verified
				// facts only — never agent self-assessment. Shadow keeps
				// today's directive byte-identical.
				if digest := kernelEvidenceDigest(dr.phaseWorktree, salvagedFromInvalid); digest != "" {
					directive += "\n\n" + digest
				}
				// Consume-once: the found-but-invalid note describes what
				// rung 1 discovered for the IMMEDIATE next dispatch; after
				// that agent has rewritten the artifact, repeating the old
				// path would be stale, not kernel-verified.
				salvagedFromInvalid = ""
			}
			dr.phaseReq.CorrectionDirective = directive
			obsCancel := cr.o.observer.Start(cr.ctx, string(next), dr.phaseReq)
			var rerr error
			dr.resp, rerr = dr.runner.Run(cr.ctx, dr.phaseReq)
			if obsCancel != nil {
				obsCancel()
			}
			if rerr != nil {
				recordCorrection(interaction.ResultDispatchFailed)
				phaseErr := fmt.Errorf("phase %q correction %d dispatch failed: %w", next, corr, rerr)
				cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, dr.resp, dr.attemptCount, phaseErr.Error(), cr.cs.PhaseStartedAt))
				cr.recordFailureLearning(next, phaseErr, corr)
				return loopAbort, wrapCycleLevelError(next, phaseErr)
			}
			// A correction re-dispatch must produce a canonical verdict to be
			// evaluable, same invariant the outer attempt loop enforces before
			// breaking. Corrections deliberately skip the bridge-timeout retry
			// ladder (scope note), so a non-canonical result here aborts rather
			// than retrying.
			if !IsVerdict(dr.resp.Verdict) {
				recordCorrection(interaction.ResultNonCanonicalVerdict)
				phaseErr := fmt.Errorf("phase %q correction %d produced a non-canonical verdict %q", next, corr, dr.resp.Verdict)
				cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, dr.resp, dr.attemptCount, phaseErr.Error(), cr.cs.PhaseStartedAt))
				cr.recordFailureLearning(next, phaseErr, corr)
				return loopAbort, wrapCycleLevelError(next, phaseErr)
			}
			// A correction is a fresh authoring pass. Establish the same recovered
			// and normalized filesystem view used for the initial review.
			if phaseErr := cr.prepareForReview(next); phaseErr != nil {
				recordCorrection(interaction.ResultRejectedAgain)
				cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, dr.resp, dr.attemptCount, phaseErr.Error(), cr.cs.PhaseStartedAt))
				cr.recordFailureLearning(next, phaseErr, corr)
				return loopAbort, wrapCycleLevelError(next, phaseErr)
			}
			// rin.Response is refreshed for reviewer consistency; the deliverable
			// reviewer reads the filesystem (workspace/worktree), not this field.
			rin.Response = dr.resp
			rr = cr.reviewDeliverable(next, rin, contractDispatch{cli: dr.phaseReq.ModelRoutingCLI, escalated: escalated, salvageRetried: salvageRetried})
			if rr.Approve {
				recordCorrection(interaction.ResultAccepted)
			} else {
				recordCorrection(interaction.ResultRejectedAgain)
			}
		}
		// Defensive: phaseReq is fresh per phase iteration, but never let the
		// directive — or a contract-block CLI escalation — outlive the loop. The
		// escalation is scoped to the failing re-dispatch by construction; the
		// restore is what keeps it from becoming a phase-wide reroute if a later
		// consumer of dr.phaseReq is ever added.
		dr.phaseReq.CorrectionDirective = ""
		dr.phaseReq.ModelRoutingCLI = baseRoutingCLI
		if !rr.Approve {
			if maxCorrections == 0 {
				// Byte-identical to the pre-feature abort message.
				phaseErr := fmt.Errorf("review gate: phase %q deliverable rejected: %s", next, rr.Reason)
				// ADR-0044 C1: the phase ran and produced its own verdict;
				// the reject is recorded as the abort reason, not a rewrite.
				cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, dr.resp, dr.attemptCount, phaseErr.Error(), cr.cs.PhaseStartedAt))
				cr.recordFailureLearning(next, phaseErr, 1)
				return loopAbort, wrapCycleLevelError(next, phaseErr)
			}
			phaseErr := fmt.Errorf("review gate: phase %q deliverable rejected after %d correction(s): %s", next, maxCorrections, rr.Reason)
			cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, dr.resp, dr.attemptCount, phaseErr.Error(), cr.cs.PhaseStartedAt))
			cr.recordFailureLearning(next, phaseErr, maxCorrections)
			return loopAbort, wrapCycleLevelError(next, phaseErr)
		}
	}

	return loopNext, nil
}
