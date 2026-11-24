package core

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/acssuite"
)

const maxPredicateReroutes = 1

var (
	testDuration   = regexp.MustCompile(`\(\d+(\.\d+)?s\)`)
	packageSummary = regexp.MustCompile(`^(FAIL|ok)\t`)
)

func acsPredicateFailures(failures []string, cycle int) []string {
	own := acssuite.CyclePackage(cycle) + " "
	var out []string
	for _, f := range failures {
		if acsPredicateFailure.MatchString(f) && strings.HasPrefix(f, own) {
			out = append(out, f)
		}
	}
	return out
}

type tddOwned struct{ Predicates, Audit []string }

func tddOwnedFailures(failures []string, cycle int) tddOwned {
	owned := tddOwned{Predicates: acsPredicateFailures(failures, cycle)}
	own := strings.TrimPrefix(acssuite.CyclePackage(cycle), "./")
	for _, f := range failures {
		if pkg, ok := testAuditLinePackage(f); ok && pkg == own {
			owned.Audit = append(owned.Audit, f)
		}
	}
	return owned
}

type tddRoute struct {
	Brief  []string
	Reason string
}

const (
	auditRerouteReason  = "build floor: the test-audit ratchet has a finding in this cycle's own TDD predicate package; Build may not edit go/acs/cycle<N>/, so TDD fixes the predicate test"
	repeatRerouteReason = "build floor: this cycle's own TDD predicate failed byte-identically across a Build correction; Build may not edit go/acs/cycle<N>/, so TDD re-authors the predicate"
)

func (cr *cycleRun) rerouteOpen(next Phase, dr *dispatchResult) bool {
	return next == PhaseBuild && !dr.remediation && cr.cs.PredicateReroutes < maxPredicateReroutes
}

func (cr *cycleRun) tddReroute(next Phase, dr *dispatchResult, owned tddOwned, previous []string) (tddRoute, bool) {
	var reason string
	switch {
	case len(owned.Audit) > 0 && cr.rerouteOpen(next, dr):
		reason = auditRerouteReason
	case cr.predicateDefectRepeats(next, dr, previous, owned.Predicates):
		reason = repeatRerouteReason
	default:
		return tddRoute{}, false
	}
	return tddRoute{Brief: append(slices.Clone(owned.Predicates), owned.Audit...), Reason: reason}, true
}

func samePredicateFailures(prev, cur []string) bool {
	return slices.EqualFunc(prev, cur, func(a, b string) bool {
		return predicateFailureIdentity(a) == predicateFailureIdentity(b)
	})
}

func predicateFailureIdentity(failure string) string {
	lines := strings.Split(testDuration.ReplaceAllString(failure, ""), "\n")
	kept := lines[:1]
	for i, line := range lines[1:] {
		if (i == 0 && strings.HasPrefix(line, "…")) || packageSummary.MatchString(line) || strings.HasPrefix(line, "=== ") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

func (cr *cycleRun) predicateDefectRepeats(next Phase, dr *dispatchResult, prev, cur []string) bool {
	return cr.rerouteOpen(next, dr) && len(cur) > 0 && samePredicateFailures(prev, cur)
}

func (cr *cycleRun) routePredicateDefectToTDD(dr *dispatchResult, route tddRoute) loopAction {
	cr.cs.PredicateReroutes++
	cr.cs.PredicateRepairBrief = strings.Join(route.Brief, "\n")
	fmt.Fprintf(cr.o.errOut(), "[orchestrator] phase %s: %s\n", PhaseBuild, route.Reason)
	cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(PhaseBuild, dr.resp, dr.attemptCount, route.Reason, cr.cs.PhaseStartedAt))
	cr.scheduledNext = PhaseTDD
	return loopContinue
}

func consumeDispatchFlags(cs *CycleState, next Phase) {
	switch next {
	case PhaseAudit:
		cs.AuditRepairActive = false
	case PhaseTDD:
		cs.PredicateRepairBrief = ""
	}
}
