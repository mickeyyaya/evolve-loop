package core

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

const maxPredicateReroutes = 1

var (
	testDuration   = regexp.MustCompile(`\(\d+(\.\d+)?s\)`)
	packageSummary = regexp.MustCompile(`^(FAIL|ok)\t`)
)

func acsPredicateFailures(failures []string, cycle int) []string {
	own := fmt.Sprintf("./acs/cycle%d ", cycle)
	var out []string
	for _, f := range failures {
		if acsPredicateFailure.MatchString(f) && strings.HasPrefix(f, own) {
			out = append(out, f)
		}
	}
	return out
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

func consumeDispatchFlags(cs *CycleState, next Phase) {
	switch next {
	case PhaseAudit:
		cs.AuditRepairActive = false
	case PhaseTDD:
		cs.PredicateRepairBrief = ""
	}
}
