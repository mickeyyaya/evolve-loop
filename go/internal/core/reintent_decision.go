package core

import (
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
	return deferDecisionIn(cs.WorkspacePath, o.failurePolicy)
}

func deferDecisionIn(workspace string, pol policy.SystemFailurePolicy) (deferDecision, bool) {
	if fb, declared := phasecontract.ReadFailureBlock(workspace, string(PhaseAudit)); declared {
		category := policyCategoryFor(failurelog.NormalizeLegacy(fb.Class))
		if cat, known := pol.RetryPolicyFor(category); known && cat.Action == policy.ActionDeferOrQuarantine {
			return deferDecision{category: category, fixType: cat.FixType, source: "audit"}, true
		}
	}
	if bd := readBuildDeclared(workspace); bd != nil {
		return deferDecision{category: bd.Class, fixType: policy.FixTypePipelineRepair, source: "build"}, true
	}
	if dec, _ := readFailureDecision(workspace); dec != nil && dec.Action == policy.ActionDeferOrQuarantine {
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

func RecordedDeferDecision(workspace string, pol policy.SystemFailurePolicy) (RecordedDefer, bool) {
	d, ok := deferDecisionIn(workspace, pol)
	return RecordedDefer{Category: d.category, FixType: d.fixType}, ok
}

func IsAuditDeferResult(result CycleResult) bool {
	return strings.Contains(result.RetroDecision, auditDeferReasonPrefix)
}
