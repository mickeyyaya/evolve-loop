package core

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/convergence"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	CodeLadderDecision   signalcenter.Code = retryladder.SignalDecision
	CodeLadderStopSpec   signalcenter.Code = retryladder.SignalStopSpec
	CodeLadderNoHeadroom signalcenter.Code = retryladder.SignalNoHeadroom
	CodeLadderAuditFix   signalcenter.Code = "LADDER_AUDIT_FIX"

	originContractCorrection = "cycleRun.reviewWithCorrections"
	originResumedCorrection  = "Orchestrator.reviewResumedDeliverable"
	originAuditRepair        = "cycleRun.applyDispatchPolicy"
	originNextItem           = "cycleRun.escalatedBuildTier"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleLadder, CodeLadderDecision, "the retry ladder chose the rung of the next try of a phase (ADR-0142); fields.stage is shadow (recorded, the dispatch does not change) or enforce, fields.origin the retry path, fields.rung the rung before, fields.next the chosen rung, fields.action raise, retry or stop, fields.class and fields.fingerprint the classified failure, fields.tier and fields.effort the target")
	signalcenter.RegisterCode(signalcenter.ModuleLadder, CodeLadderStopSpec, "two tiers failed on the same check, so the ladder reclassified the failure as spec and stopped; fields.fingerprint names the check")
	signalcenter.RegisterCode(signalcenter.ModuleLadder, CodeLadderAuditFix, "the audit_fix rule ran after an audit FAIL; fields.audit_fix is would (shadow), chosen (enforce) or decline, fields.audit_fix_rule names the rule that decided, and the other fields give its data: the findings, the prescriptions, the estimated files, lines, production code lines and tokens, and if a rung handler is registered")
	signalcenter.RegisterCode(signalcenter.ModuleLadder, CodeLadderNoHeadroom, "no family of the chain has headroom at the next tier, so the ladder holds at R1 or stops; fields.rung is the rung before")
}

type ladderAsk struct {
	phase    Phase
	origin   string
	facts    retryladder.Facts
	failures []string
	findings string
	worktree string
}

func contractLadderAsk(phase Phase, origin, worktree string, rr ReviewResult) ladderAsk {
	failures := []string{rr.Reason}
	if rr.Remediation != "" {
		failures = append(failures, rr.Remediation)
	}
	return ladderAsk{phase: phase, origin: origin, worktree: worktree, failures: failures,
		facts: retryladder.Facts{ContractRejected: true, ProvenBlocker: rr.ProvenBlocker != "", Reason: rr.Reason}}
}

func (cr *cycleRun) askLadder(ask ladderAsk, req PhaseRequest) (PhaseRequest, bool) {
	if !retryladder.Climbs(string(ask.phase)) {
		return req, false
	}
	verdict := retryladder.Classify(ask.facts)
	prior := cr.cs.Ladder
	if prior == nil {
		prior = &LadderState{Rung: string(retryladder.R0)}
	}
	base, chain, headroom := ladderRoute(cr.ladderPlan(ask.phase))
	d := retryladder.Next(retryladder.Input{
		Rung: retryladder.Rung(prior.Rung), Earlier: prior.Fingerprints, Verdict: verdict, Base: base, Chain: chain, Headroom: headroom,
	})
	cr.cs.Ladder = nextLadderState(prior, d, verdict)
	cr.o.emitLadderDecision(cr.cs, ask, prior.Rung, d, verdict)
	cfg := cr.o.workflowConfig.RetryLadder
	try := retryladder.Try{FailureClass: verdict.Class, Fingerprint: verdict.Fingerprint, Would: d.Rung}
	if !cfg.Enforced() {
		req.Ladder = &try
		return req, false
	}
	if d.Action != retryladder.ActionRaise {
		return req, false
	}
	try.Rung, try.Context, try.Stage, try.Tier = d.Rung, retryladder.ContextFresh, retryladder.StageEnforce, d.Target.Tier
	req.Ladder = &try
	req.Assignment.Effort = d.Target.Effort
	req.CorrectionDirective = retryladder.Digest{
		Rung: d.Rung, Class: verdict.Class, Failures: ask.failures, Findings: ask.findings,
		ChangedFiles: gitStatusNames(ask.worktree, evidenceStatusMaxNames), History: prior.History,
	}.Render(cfg.DigestTokens)
	return req, true
}

func nextLadderState(prior *LadderState, d retryladder.Decision, v retryladder.Verdict) *LadderState {
	next := &LadderState{
		Rung: prior.Rung, Tier: prior.Tier, Effort: prior.Effort, RepairRound: prior.RepairRound, AuditFix: prior.AuditFix,
		Fingerprints: append(slices.Clone(prior.Fingerprints), v.Fingerprint),
		History:      append(slices.Clone(prior.History), fmt.Sprintf("%s %s %s", prior.Rung, v.Class, v.Fingerprint)),
	}
	if d.Action == retryladder.ActionRaise {
		next.Rung, next.Tier, next.Effort = string(d.Rung), d.Target.Tier, d.Target.Effort
	}
	return next
}

func (cr *cycleRun) ladderPlan(phase Phase) llmroute.Plan {
	d, _ := cr.routingDecision(phase)
	return d.Plan
}

func ladderRoute(plan llmroute.Plan) (convergence.TierEffort, []string, convergence.HeadroomTable) {
	var chain []string
	headroom := convergence.HeadroomTable{}
	for _, cli := range plan.Candidates {
		family := llmroute.Family(cli)
		if _, seen := headroom[family]; !seen {
			chain = append(chain, family)
			headroom[family] = map[string]convergence.ModelEffort{}
		}
		for _, tier := range modelcatalog.CanonicalTiers {
			if plan.Permits(cli, tier) {
				headroom[family][tier] = convergence.ModelEffort{}
			}
		}
	}
	base := convergence.TierEffort{Tier: plan.Model, Effort: plan.Effort}
	if len(chain) > 0 {
		base.Family = chain[0]
	}
	return base, chain, headroom
}

func (cr *cycleRun) keepLadderRung(phase Phase, req PhaseRequest) PhaseRequest {
	st := cr.cs.Ladder
	if st == nil || st.Rung == string(retryladder.R0) {
		return req
	}
	if !cr.o.workflowConfig.RetryLadder.Enforced() || !retryladder.Climbs(string(phase)) {
		return req
	}
	req.Ladder = &retryladder.Try{Rung: retryladder.Rung(st.Rung), Stage: retryladder.StageEnforce, Tier: st.Tier}
	req.Assignment.Effort = st.Effort
	return req
}

func (cr *cycleRun) climbResumed(phase Phase, review ReviewResult, req *PhaseRequest) bool {
	next, raised := cr.askLadder(contractLadderAsk(phase, originResumedCorrection, cr.cs.ActiveWorktree, review), *req)
	*req = next
	return raised
}

func (cr *cycleRun) ladderForDispatch(phase Phase, req PhaseRequest) PhaseRequest {
	round := cr.cs.AuditRepairAttempts
	if !cr.cs.AuditRepairActive || !repairSeededPhase(phase) || (cr.cs.Ladder != nil && cr.cs.Ladder.RepairRound == round) {
		return cr.keepLadderRung(phase, req)
	}
	ask, auditFix := cr.auditLadderAsk(phase)
	next, raised := cr.askLadder(ask, req)
	cr.cs.Ladder = withRepairRound(cr.cs.Ladder, round, auditFix)
	if raised {
		return next
	}
	return cr.keepLadderRung(phase, next)
}

func withRepairRound(st *LadderState, round int, auditFix string) *LadderState {
	stamped := LadderState{Rung: string(retryladder.R0)}
	if st != nil {
		stamped = *st
	}
	stamped.RepairRound = round
	stamped.AuditFix = cmp.Or(auditFix, stamped.AuditFix)
	return &stamped
}

func (cr *cycleRun) auditLadderAsk(phase Phase) (ladderAsk, string) {
	_, fb := cr.o.auditFailEnvelope(cr.cs, "")
	auditFix := cr.recordAuditFix(phase, fb)
	return ladderAsk{
		phase: phase, origin: originAuditRepair, worktree: cr.cs.ActiveWorktree, failures: cr.cs.AuditFailReasons,
		findings: composeRepairBrief(cr.cs, cr.o.errOut()),
		facts:    retryladder.Facts{AuditClass: fb.Class, Reason: firstNonEmpty(cr.cs.AuditFailReasons...)},
	}, auditFix
}

func (cr *cycleRun) nextItemStartRung(ids []string) retryladder.Rung {
	d := retryladder.Decision{Action: retryladder.ActionStop, Rung: retryladder.R0}
	if read := cr.o.failureClassFor; read != nil {
		for _, id := range ids {
			class := retryladder.Class(read(id))
			if class == retryladder.Capability {
				d = retryladder.Decision{Action: retryladder.ActionRaise, Rung: retryladder.R2, Class: class}
				break
			}
			if d.Class == "" {
				d.Class = class
			}
		}
	}
	if cr.cs.Ladder == nil {
		cr.cs.Ladder = &LadderState{Rung: string(d.Rung)}
		if d.Rung == retryladder.R2 {
			cr.cs.Ladder.Tier = retryEscalationTier
		}
		cr.o.emitLadderDecision(cr.cs, ladderAsk{phase: PhaseBuild, origin: originNextItem}, string(retryladder.R0), d, retryladder.Verdict{})
	}
	return d.Rung
}

func preferDigest(plain string, req PhaseRequest) string {
	if req.Ladder != nil && req.Ladder.Context == retryladder.ContextFresh {
		return req.CorrectionDirective
	}
	return plain
}

func (o *Orchestrator) emitLadderDecision(cs CycleState, ask ladderAsk, from string, d retryladder.Decision, v retryladder.Verdict) {
	stage := o.workflowConfig.RetryLadder.Stage
	base := signalcenter.Event{
		Cycle: cs.CycleID, RunID: o.signalRunID(), Phase: string(ask.phase),
		Module: signalcenter.ModuleLadder, Origin: ask.origin, Kind: signalcenter.KindLadderDecision,
		Fields: map[string]string{
			"stage": stage, "origin": ask.origin, "rung": from, "next": string(d.Rung), "action": string(d.Action),
			"class": string(d.Class), "fingerprint": v.Fingerprint, "tier": d.Target.Tier, "effort": d.Target.Effort,
		},
	}
	decision := base
	decision.Severity, decision.Code = signalcenter.SeverityInfo, CodeLadderDecision
	decision.Reason = fmt.Sprintf("retry ladder (%s) for %s: %s at %s -> %s %s", stage, ask.phase, d.Class, from, d.Action, d.Rung)
	o.signals.Emit(decision)
	for _, code := range d.Signals {
		warn := base
		warn.Severity, warn.Code = signalcenter.SeverityWarn, signalcenter.Code(code)
		warn.Reason = fmt.Sprintf("retry ladder (%s) for %s: %s at %s", stage, ask.phase, code, from)
		o.signals.Emit(warn)
	}
}
