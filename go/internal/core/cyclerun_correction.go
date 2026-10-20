package core

import (
	"fmt"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
)

var nextCorrection = interaction.NextCorrection

func (cr *cycleRun) reviewWithCorrections(next Phase, dr *dispatchResult) (loopAction, error) {
	if cr.o.reviewer != nil && dr.resp.Verdict != VerdictSKIPPED {
		rin := cr.o.reviewInputFor(cr.cs, next, cr.req.ProjectRoot, dr.resp)
		baseRoutingCLI := dr.phaseReq.ModelRoutingCLI
		escalated := false
		salvageRetried := false
		var prevBlockIdentity contractBlockIdentity
		rr := cr.reviewDeliverable(next, rin, contractDispatch{cli: baseRoutingCLI})
		maxCorrections := cr.correctionLimitFor(next, cr.retryConfig.ContractCorrectionRetries)
		irec := interaction.NewRecorder(cr.cs.WorkspacePath)
		decisionID := ""
		if !rr.Approve && (maxCorrections > 0 || cr.o.contractVerifier != nil) {
			decisionID = fmt.Sprintf("%s-c%d-%d", next, cr.cycle, cr.o.now().UnixNano())
		}
		rungBudget := map[string]int{
			interaction.RungSalvage:    1,
			interaction.RungLiveFix:    1,
			interaction.RungRedispatch: maxCorrections,
		}
		if cr.o.contractVerifier == nil {
			rungBudget[interaction.RungSalvage] = 0
		}
		corr := 0
		salvagedFromInvalid := ""
		var prevPredicates []string
		for !rr.Approve {
			if diag, ok := cr.provenBlocker(next, rin, rr); ok {
				return cr.endOnProvenBlocker(next, dr, diag)
			}
			predicates := acsPredicateFailures(rr.Failures, cr.cycle)
			if cr.predicateDefectRepeats(next, dr, prevPredicates, predicates) {
				return cr.routePredicateDefectToTDD(dr, predicates), nil
			}
			act := nextCorrection(interaction.CorrectionInput{
				Phase:      string(next),
				Workspace:  cr.cs.WorkspacePath,
				Worktree:   rin.Worktree,
				Violation:  rr.Reason,
				NamedREPL:  false,
				Busy:       false,
				DecisionID: decisionID,
				RungBudget: rungBudget,
			})
			if act.Rung == "" {
				break
			}
			rungBudget[act.Rung]--
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
					fmt.Fprintf(cr.o.errOut(), "[orchestrator] phase %s: would-salvage misplaced deliverable (%s; EVOLVE_PHASE_RECOVERY=%s)\n", next, act.Reason, cr.o.cfg.PhaseRecovery)
					irec.Record(interaction.Outcome{Event: salvEv, Result: interaction.ResultWouldAct})
					continue
				}
				salvStart := cr.o.now()
				sr := cr.o.salvageDeliverable(cr.ctx, rin)
				switch {
				case sr.Relocated && sr.Verified:
					rr = cr.reviewDeliverable(next, rin, contractDispatch{cli: dr.phaseReq.ModelRoutingCLI, escalated: escalated, salvageRetried: salvageRetried})
					res := interaction.ResultRejectedAgain
					if rr.Approve {
						res = interaction.ResultAccepted
					}
					fmt.Fprintf(cr.o.errOut(), "[orchestrator] phase %s: salvaged %s → contracted path (gate approve=%v)\n", next, sr.From, rr.Approve)
					irec.Record(interaction.Outcome{Event: salvEv, Result: res, LatencyMS: cr.o.now().Sub(salvStart).Milliseconds()})
				case sr.Relocated:
					salvagedFromInvalid = sr.From
					fmt.Fprintf(cr.o.errOut(), "[orchestrator] phase %s: salvage relocated %s but the destination failed verification — falling through\n", next, sr.From)
					irec.Record(interaction.Outcome{Event: salvEv, Result: interaction.ResultFoundButInvalid, LatencyMS: cr.o.now().Sub(salvStart).Milliseconds()})
				default:
					fmt.Fprintf(cr.o.errOut(), "[orchestrator] phase %s: nothing to salvage (%s)\n", next, sr.Reason)
					irec.Record(interaction.Outcome{Event: salvEv, Result: interaction.ResultNotFound, LatencyMS: cr.o.now().Sub(salvStart).Milliseconds()})
				}
				continue
			}
			if act.Rung == interaction.RungLiveFix {
				continue
			}
			corr++
			prevPredicates = predicates
			fmt.Fprintf(cr.o.errOut(), "[orchestrator] phase %s: contract violation (correction %d/%d) — re-dispatching with correction: %s\n",
				next, corr, maxCorrections, rr.Reason)
			salvageRetry := false
			escalEligible := rr.Blocks >= contractEscalateAtBlock ||
				(rr.Remediation != "" && corr >= contractEscalateAtBlock)
			if escalEligible && contractBlocksShareIdentity(prevBlockIdentity, rr.Reason) {
				failedCLI := cr.contractDispatchCLI(next, dr.phaseReq.ModelRoutingCLI)
				esc, ok := cr.contractEscalationCLI(next, dr.phaseReq.ModelRoutingCLI)
				switch {
				case ok && esc != dr.phaseReq.ModelRoutingCLI:
					blocksForLog := rr.Blocks
					if blocksForLog == 0 {
						blocksForLog = corr
					}
					fmt.Fprintf(cr.o.errOut(), "[orchestrator] phase %s: CLI ESCALATION — %d consecutive identical block(s) on cli=%s; correction %d re-dispatches on cli=%s (these rejections never trigger cli_fallback, which is how an incapable CLI used to burn every round). The phase's primary routing is unchanged.\n",
						next, blocksForLog, failedCLI, corr, esc)
					dr.phaseReq.ModelRoutingCLI = esc
					escalated = true
				case !ok:
					fmt.Fprintf(cr.o.errOut(), "[orchestrator] phase %s: CONTRACT SALVAGE RETRY — %d consecutive contract blocks on cli=%s and no other CLI family is available to escalate to; correction %d re-dispatches on the SAME cli with a structured re-prompt carrying the validator output verbatim. Breaker-neutral: no extra dispatch, no extra correction, no routing change.\n",
						next, rr.Blocks, failedCLI, corr)
					salvageRetry = true
					salvageRetried = true
				}
			}
			prevBlockIdentity = newContractBlockIdentity(rr.Reason)
			if lerr := cr.o.ledger.Append(cr.ctx, LedgerEntry{
				TS:       cr.o.now().UTC().Format(time.RFC3339),
				Cycle:    cr.cycle,
				Role:     string(next),
				Kind:     "contract_correction",
				ExitCode: 0,
			}); lerr != nil {
				fmt.Fprintf(cr.o.errOut(), "[orchestrator] WARN contract_correction ledger append: %v\n", lerr)
			}
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
			directive := composeCorrection(corr, rr.Reason, rr.Remediation)
			if salvageRetry {
				directive = composeContractSalvageRetry(corr, rr.Reason, rr.Remediation)
			}
			if cr.o.cfg.PhaseRecovery == config.StageEnforce {
				if digest := kernelEvidenceDigest(rin.Worktree, salvagedFromInvalid); digest != "" {
					directive += "\n\n" + digest
				}
				salvagedFromInvalid = ""
			}
			dr.phaseReq.CorrectionDirective = directive
			cr.o.emitGateCorrection(gateCorrection{
				origin: "cycleRun.reviewWithCorrections", cycle: cr.cycle, phase: next, correction: corr, max: maxCorrections,
				rung: interaction.RungRedispatch, cli: dr.phaseReq.ModelRoutingCLI, escalated: escalated, salvageRetry: salvageRetry, reason: rr.Reason,
			})
			dr.phaseReq = cr.countDispatch(next, dr.phaseReq)
			obsCancel := cr.o.observer.Start(cr.ctx, string(next), dr.phaseReq)
			var rerr error
			dr.resp, rerr = dr.runner.Run(cr.ctx, dr.phaseReq)
			if obsCancel != nil {
				obsCancel()
			}
			if rerr != nil {
				if isQuotaWall(rerr) {
					recordCorrection(interaction.ResultQuotaDeferred)
					return loopAbort, cr.pauseForQuota(next, dr.resp, dr.attemptCount+corr, rerr)
				}
				recordCorrection(interaction.ResultDispatchFailed)
				phaseErr := fmt.Errorf("phase %q correction %d dispatch failed: %w", next, corr, rerr)
				cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, dr.resp, dr.attemptCount, phaseErr.Error(), cr.cs.PhaseStartedAt))
				cr.recordFailureLearning(next, phaseErr, corr)
				return loopAbort, wrapCycleLevelError(next, phaseErr)
			}
			if !IsVerdict(dr.resp.Verdict) {
				recordCorrection(interaction.ResultNonCanonicalVerdict)
				phaseErr := fmt.Errorf("phase %q correction %d produced a non-canonical verdict %q", next, corr, dr.resp.Verdict)
				cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, dr.resp, dr.attemptCount, phaseErr.Error(), cr.cs.PhaseStartedAt))
				cr.recordFailureLearning(next, phaseErr, corr)
				return loopAbort, wrapCycleLevelError(next, phaseErr)
			}
			if phaseErr := cr.prepareForReview(next, dr); phaseErr != nil {
				recordCorrection(interaction.ResultRejectedAgain)
				cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(next, dr.resp, dr.attemptCount, phaseErr.Error(), cr.cs.PhaseStartedAt))
				cr.recordFailureLearning(next, phaseErr, corr)
				return loopAbort, wrapCycleLevelError(next, phaseErr)
			}
			rin.Response = dr.resp
			rr = cr.reviewDeliverable(next, rin, contractDispatch{cli: dr.phaseReq.ModelRoutingCLI, escalated: escalated, salvageRetried: salvageRetried})
			if rr.Approve {
				recordCorrection(interaction.ResultAccepted)
			} else {
				recordCorrection(interaction.ResultRejectedAgain)
			}
		}
		dr.phaseReq.CorrectionDirective = ""
		dr.phaseReq.ModelRoutingCLI = baseRoutingCLI
		if !rr.Approve {
			return cr.rejectAfterCorrections(next, dr, rr, maxCorrections)
		}
	}

	return loopNext, nil
}
