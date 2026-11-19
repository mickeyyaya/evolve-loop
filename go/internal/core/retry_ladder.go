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
	CodeLadderRedesign   signalcenter.Code = "LADDER_REDESIGN"

	originContractCorrection = "cycleRun.reviewWithCorrections"
	originResumedCorrection  = "Orchestrator.reviewResumedDeliverable"
	originAuditRepair        = "cycleRun.applyDispatchPolicy"
	originNextItem           = "cycleRun.escalatedBuildTier"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleLadder, CodeLadderRedesign, "the ladder ended (R3 failed, or a spec stop) and routed the subject to the redesign rung of ADR-0126 once (ADR-0164 ruling 1). fields.fingerprint names the failed check; fields.origin names the retry path. A second ladder end goes to the operator with no second redesign")
	signalcenter.RegisterCode(signalcenter.ModuleLadder, CodeLadderDecision, "the retry ladder chose the rung of the next phase try (ADR-0142). fields.stage is shadow (dispatch does not change) or enforce. fields.origin is the retry path, fields.rung is the rung before, and fields.next is the chosen rung. fields.action is raise, retry, or stop. fields.class and fields.fingerprint give the failure. fields.tier and fields.effort give the target")
	signalcenter.RegisterCode(signalcenter.ModuleLadder, CodeLadderStopSpec, "two tiers failed on the same check, so the ladder reclassified the failure as spec and stopped; fields.fingerprint names the check")
	signalcenter.RegisterCode(signalcenter.ModuleLadder, CodeLadderAuditFix, "the audit_fix rule ran after an audit FAIL or on residual build-floor findings. fields.source is audit or build_floor; fields.audit_fix is `would` (shadow), chosen (enforce), or decline; fields.audit_fix_rule names the rule. Other fields hold findings, prescriptions, files, lines, code lines, tokens, and handler registration")
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
		facts: retryladder.Facts{ContractRejected: true, Reason: rr.Reason}}
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
	d := retryladder.Next(cr.ladderInput(ask.phase, prior, verdict))
	cr.cs.Ladder = nextLadderState(prior, d, verdict)
	cr.o.emitLadderDecision(cr.cs, ask, prior.Rung, d, verdict)
	cr.routeToRedesign(ask, prior, d, verdict)
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

func endsTheLadder(prior *LadderState, d retryladder.Decision) bool {
	if d.Action != retryladder.ActionStop {
		return false
	}
	return d.Class == retryladder.Spec || (d.Class == retryladder.Capability && prior.Rung == string(retryladder.R3))
}

func (cr *cycleRun) routeToRedesign(ask ladderAsk, prior *LadderState, d retryladder.Decision, v retryladder.Verdict) {
	if !endsTheLadder(prior, d) || prior.Redesign != "" {
		return
	}
	cr.cs.Ladder.Redesign = cmp.Or(v.Fingerprint, string(d.Class))
	cr.o.signals.Emit(signalcenter.Event{
		Cycle: cr.cs.CycleID, RunID: cr.o.signalRunID(), Phase: string(ask.phase),
		Module: signalcenter.ModuleLadder, Origin: ask.origin, Kind: signalcenter.KindLadderDecision,
		Severity: signalcenter.SeverityWarn, Code: CodeLadderRedesign,
		Reason: fmt.Sprintf("retry ladder for %s: %s ended the ladder at %s, so the subject goes to the redesign rung", ask.phase, d.Class, prior.Rung),
		Fields: map[string]string{"fingerprint": v.Fingerprint, "origin": ask.origin, "class": string(d.Class), "rung": prior.Rung},
	})
}

func nextLadderState(prior *LadderState, d retryladder.Decision, v retryladder.Verdict) *LadderState {
	next := &LadderState{
		Rung: prior.Rung, Tier: prior.Tier, Effort: prior.Effort, RepairRound: prior.RepairRound, AuditFix: prior.AuditFix, Redesign: prior.Redesign,
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
	var fp string
	if len(st.Fingerprints) > 0 {
		fp = st.Fingerprints[len(st.Fingerprints)-1]
	}
	req.Ladder = &retryladder.Try{Rung: retryladder.Rung(st.Rung), Fingerprint: fp, Stage: retryladder.StageEnforce, Tier: st.Tier}
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
	next, raised := cr.askLadder(cr.auditLadderAsk(phase), req)
	cr.cs.Ladder = withRepairRound(cr.cs.Ladder, round)
	if raised {
		return next
	}
	return cr.keepLadderRung(phase, next)
}

func withRepairRound(st *LadderState, round int) *LadderState {
	stamped := ladderCopy(st)
	stamped.RepairRound = round
	return &stamped
}

func withAuditFix(st *LadderState, mark string) *LadderState {
	stamped := ladderCopy(st)
	stamped.AuditFix = cmp.Or(mark, stamped.AuditFix)
	return &stamped
}

func ladderCopy(st *LadderState) LadderState {
	if st == nil {
		return LadderState{Rung: string(retryladder.R0)}
	}
	return *st
}

func (cr *cycleRun) auditLadderAsk(phase Phase) ladderAsk {
	_, fb := cr.o.auditFailEnvelope(cr.cs, "")
	return ladderAsk{
		phase: phase, origin: originAuditRepair, worktree: cr.cs.ActiveWorktree, failures: cr.cs.AuditFailReasons,
		findings: composeRepairBrief(cr.cs, cr.o.errOut()),
		facts:    retryladder.Facts{AuditClass: fb.Class, Reason: firstNonEmpty(cr.cs.AuditFailReasons...)},
	}
}

func (cr *cycleRun) ladderInput(phase Phase, prior *LadderState, verdict retryladder.Verdict) retryladder.Input {
	base, chain, headroom := ladderRoute(cr.ladderPlan(phase))
	return retryladder.Input{
		Rung: retryladder.Rung(prior.Rung), Earlier: prior.Fingerprints, Verdict: verdict, Base: base, Chain: chain, Headroom: headroom,
	}
}

func worstScopedClass(read func(string) string, ids []string) retryladder.Class {
	var worst retryladder.Class
	if read == nil {
		return worst
	}
	for _, id := range ids {
		class := retryladder.Class(read(id))
		if class == retryladder.Capability {
			return class
		}
		worst = cmp.Or(worst, class)
	}
	return worst
}

func (cr *cycleRun) nextItemDecision(class retryladder.Class) (retryladder.Decision, retryladder.Verdict) {
	verdict := retryladder.VerdictFromClass(class)
	return retryladder.Next(cr.ladderInput(PhaseBuild, startPrior(), verdict)), verdict
}

func startPrior() *LadderState {
	return &LadderState{Rung: string(retryladder.R0)}
}

func (cr *cycleRun) recordStartRung(d retryladder.Decision, verdict retryladder.Verdict) {
	if cr.cs.Ladder != nil {
		return
	}
	prior := startPrior()
	cr.cs.Ladder = nextLadderState(prior, d, verdict)
	cr.o.emitLadderDecision(cr.cs, ladderAsk{phase: PhaseBuild, origin: originNextItem}, prior.Rung, d, verdict)
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
