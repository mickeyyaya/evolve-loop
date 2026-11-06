package core

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
)

type BlockerOutcome int

const (
	NoClaim BlockerOutcome = iota
	Routable
	Unroutable
	Incomplete
	Refused
)

type blockerRounds struct {
	End   bool
	Claim bool
}

func blockerRoundsDecision(outcome BlockerOutcome) blockerRounds {
	switch outcome {
	case Routable, Unroutable, Incomplete:
		return blockerRounds{End: true}
	case Refused:
		return blockerRounds{Claim: true}
	}
	return blockerRounds{}
}

type blockerEnd struct {
	outcome BlockerOutcome
	diag    Diagnostic
}

func (e blockerEnd) Error() string { return e.diag.Message }

func endedBlockerOutcome(next Phase, resp PhaseResponse, attempts int, end blockerEnd, startedAt string) recovery.PhaseOutcome {
	resp.Verdict = VerdictFAIL
	resp.Diagnostics = append([]Diagnostic{end.diag}, resp.Diagnostics...)
	return phaseOutcomeFrom(next, resp, attempts, end.Error(), startedAt)
}

func (cr *cycleRun) endOnBlocker(next Phase, dr *dispatchResult, outcome BlockerOutcome, diag Diagnostic) (loopAction, error) {
	if outcome == Routable {
		return cr.endOnProvenBlocker(next, dr, diag)
	}
	end := blockerEnd{outcome: outcome, diag: diag}
	cr.o.recordPhaseOutcome(&cr.result, &cr.phaseTimings, cr.cs.WorkspacePath, endedBlockerOutcome(next, dr.resp, dr.attemptCount, end, cr.cs.PhaseStartedAt))
	cr.recordFailureLearning(next, end, max(dr.attemptCount, 1))
	return loopAbort, wrapCycleLevelError(next, end)
}

type blockerStatement struct{ fact, step string }

func (s blockerStatement) text() string { return s.fact + ". Next: " + s.step }

func (s blockerStatement) lastRound() string {
	return s.fact + ". This was the last correction, so this cycle ends. Next: the item returns to the queue. A quarantined item needs evolve inbox quarantine release <id>."
}

func (s blockerStatement) delivered() string {
	return s.fact + ". This was the last correction, and the audit_fix escalation delivers the build. Next: the audit reads the build."
}

func (s blockerStatement) empty() bool { return s == blockerStatement{} }

const routeStep = "at the boundary, the console routes each item that needs this path with evolve inbox route-console <id> <reason> <cycle>. A quarantined item needs evolve inbox quarantine release <id> first."

type routeGap struct{ clause, step string }

func (g routeGap) unroutable() blockerStatement {
	return blockerStatement{fact: "the blocker is proven, and " + g.clause + ", so this cycle ends with no route", step: g.step}
}

func (g routeGap) handoff(cycle int, reason string) string {
	return fmt.Sprintf("cycle %d handed off. No item routes to the console: %s. Next: %s The hand-off reason: %s", cycle, g.clause, g.step, reason)
}

func consoleRouteChoice(workspace string) (string, routeGap, bool) {
	committed, source := committedset.CommittedWithSource(workspace)
	decisionPath := filepath.Join(workspace, committedset.DecisionFile)
	switch {
	case source == committedset.SourceUnknown:
		pinPath := filepath.Join(workspace, committedset.LanePinFile)
		return "", routeGap{clause: "neither " + pinPath + " nor " + decisionPath + " gives a commitment", step: "the console reads both paths."}, false
	case len(committed) == 0:
		return "", routeGap{clause: "the triage decision commits no item", step: "the console reads " + decisionPath + "."}, false
	case len(committed) > 1 && source == committedset.SourcePin:
		return "", routeGap{clause: "this lane is pinned on items " + strings.Join(committed, ", ") + ", and a console route needs exactly one", step: routeStep}, false
	case len(committed) > 1:
		return "", routeGap{clause: "the triage decision commits items " + strings.Join(committed, ", ") + ", and a console route needs exactly one", step: routeStep}, false
	}
	return committed[0], routeGap{}, true
}

func NewBlockerEnd(outcome BlockerOutcome, diag Diagnostic) error {
	return blockerEnd{outcome: outcome, diag: diag}
}
