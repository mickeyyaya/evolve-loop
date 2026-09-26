package core

import (
	"fmt"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

var kindReportPhases = []Phase{PhaseScout, PhaseTriage}

func kindDeclaringPhase(next Phase) bool {
	for _, p := range kindReportPhases {
		if p == next {
			return true
		}
	}
	return false
}

func kindSignals(workspace string) router.RoutingSignals {
	phases := make([]string, len(kindReportPhases))
	for i, p := range kindReportPhases {
		phases[i] = string(p)
	}
	sig, err := router.Digest(workspace, phases)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN deliverable-kind digest failed: %v — reading the conservative kind\n", err)
	}
	for _, reason := range sig.DigestDegraded {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN deliverable-kind digest degraded (%s) — reading the conservative kind\n", reason)
	}
	return sig
}

func dispatchSignals(next Phase, workspace, projectRoot string) map[string]string {
	sig := kindSignals(workspace)
	kind, declared := sig.DeclaredDeliverableKind()
	if !declared {
		kind = config.DeliverableKindCode
		if kindDeclaringPhase(next) && len(sig.DigestDegraded) == 0 {
			if d, ok := domainDefaultKind(projectRoot); ok {
				kind = d
			}
		}
	}
	out := map[string]string{config.SignalDeliverableKind: kind}
	if gt := sig.Scout.GoalType; gt != "" {
		out[config.SignalGoalType] = gt
	}
	return out
}

func domainDefaultKind(projectRoot string) (string, bool) {
	d, ok, err := config.LoadDomain(projectRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN .evolve/domain.json unreadable — no default deliverable kind: %v\n", err)
		return "", false
	}
	if !ok {
		return "", false
	}
	return d.DefaultDeliverableKind(), true
}
