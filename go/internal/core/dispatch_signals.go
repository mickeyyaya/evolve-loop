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
	declared, _ := sig.DeclaredDeliverableKind()
	domainDefault := noDomainDefault
	if kindDeclaringPhase(next) && len(sig.DigestDegraded) == 0 {
		domainDefault = projectDomainDefault(projectRoot)
	}
	out := map[string]string{config.SignalDeliverableKind: resolveDeliverableKind(declared, domainDefault)}
	if gt := sig.Scout.GoalType; gt != "" {
		out[config.SignalGoalType] = gt
	}
	return out
}

func resolveDeliverableKind(declared string, domainDefault func() (string, bool)) string {
	if kind := router.NormalizeDeliverableKind(declared); kind != "" {
		return kind
	}
	if kind, ok := domainDefault(); ok {
		return kind
	}
	return config.DeliverableKindCode
}

func projectDomainDefault(projectRoot string) func() (string, bool) {
	return func() (string, bool) { return domainDefaultKind(projectRoot) }
}

func noDomainDefault() (string, bool) { return "", false }

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
