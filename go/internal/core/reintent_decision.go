package core

import (
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/failurelog"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

const auditDeferReasonPrefix = "audit-fail-defer: "

type deferDecision struct {
	category string
	fixType  string
	source   string
}

func (o *Orchestrator) auditDeclineDefers(cs CycleState) (deferDecision, bool) {
	if cs.AuditDeclineReason == "" {
		return deferDecision{}, false
	}
	return o.deferDecision(cs)
}

func (o *Orchestrator) deferDecision(cs CycleState) (deferDecision, bool) {
	if fb, declared := phasecontract.ReadFailureBlock(cs.WorkspacePath, string(PhaseAudit)); declared {
		category := policyCategoryFor(failurelog.NormalizeLegacy(fb.Class))
		if cat, known := o.failurePolicy.RetryPolicyFor(category); known && cat.Action == policy.ActionDeferOrQuarantine {
			return deferDecision{category: category, fixType: cat.FixType, source: "audit"}, true
		}
	}
	if bd := o.failureDossier(cs, VerdictFAIL).BuildDeclared; bd != nil {
		return deferDecision{category: bd.Class, fixType: policy.FixTypePipelineRepair, source: "build"}, true
	}
	if dec := trustedFailureDecision(cs); dec != nil && dec.Action == policy.ActionDeferOrQuarantine {
		return deferDecision{category: dec.Category, fixType: dec.FixType, source: "failure-decision.json"}, true
	}
	return deferDecision{}, false
}

func (d deferDecision) reason() string {
	return auditDeferReasonPrefix + d.category + " maps to " + policy.ActionDeferOrQuarantine + " (" + d.fixType +
		"), per " + d.source + "; the cycle ends and the item is deferred, with no further tdd, build or audit round"
}

func (d deferDecision) routingDecision() router.RouterDecision {
	return router.RouterDecision{
		NextPhase: router.PhaseEnd,
		Reason:    d.reason(),
		Evidence:  map[string]interface{}{"category": d.category, "fix_type": d.fixType, "source": d.source},
	}
}

type RecordedDefer struct {
	Category string
	FixType  string
}

func DeferOf(result CycleResult) (RecordedDefer, bool) {
	d := RecordedDefer{Category: result.DeferCategory, FixType: result.DeferFixType}
	return d, d != RecordedDefer{}
}

func (o *Orchestrator) carryDefer(cs CycleState, reason string, result *CycleResult) {
	if !strings.Contains(reason, auditDeferReasonPrefix) {
		return
	}
	d, ok := o.auditDeclineDefers(cs)
	if !ok {
		fmt.Fprintf(o.errOut(), "[orchestrator] WARN cycle %d: the retro decision is a defer, but no defer decision resolves; the closeout releases the items\n", cs.CycleID)
		return
	}
	result.DeferCategory, result.DeferFixType = d.category, d.fixType
}
