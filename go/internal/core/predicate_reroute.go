package core

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

const maxPredicateReroutes = 1

var testDuration = regexp.MustCompile(`\(\d+(\.\d+)?s\)`)

func acsPredicateFailures(failures []string) []string {
	var out []string
	for _, f := range failures {
		if acsPredicateFailure.MatchString(f) {
			out = append(out, f)
		}
	}
	return out
}

func samePredicateFailures(prev, cur []string) bool {
	return slices.EqualFunc(prev, cur, func(a, b string) bool {
		return testDuration.ReplaceAllString(a, "") == testDuration.ReplaceAllString(b, "")
	})
}

func (cr *cycleRun) predicateDefectRepeats(next Phase, dr *dispatchResult, prev, cur []string) bool {
	return next == PhaseBuild && !dr.remediation && len(cur) > 0 && samePredicateFailures(prev, cur) &&
		cr.cs.PredicateReroutes < maxPredicateReroutes
}

func (cr *cycleRun) routePredicateDefectToTDD(dr *dispatchResult, predicates []string) loopAction {
	cr.cs.PredicateReroutes++
	cr.cs.PredicateRepairBrief = strings.Join(predicates, "\n")
	reason := "build floor: this cycle's own TDD predicate failed byte-identically across a Build correction; Build may not edit go/acs/cycle<N>/, so TDD re-authors the predicate"
	fmt.Fprintf(cr.o.errOut(), "[orchestrator] phase %s: %s\n", PhaseBuild, reason)
	cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, phaseOutcomeFrom(PhaseBuild, dr.resp, dr.attemptCount, reason, cr.cs.PhaseStartedAt))
	cr.scheduledNext = PhaseTDD
	return loopContinue
}
