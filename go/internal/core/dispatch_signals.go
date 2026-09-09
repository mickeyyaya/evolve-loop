package core

import (
	"fmt"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

// dispatch_signals.go — ADR-0099 slice 3: core is the ONE digester feeding the
// dispatch-time selectors (the skill-overlay `when` rule). The runner copies
// PhaseRequest.Signals onto policy.OverlayDispatch and never re-reads the
// workspace, so overlay selection and the kernel's own classification
// (DocumentCycle) read the same digest and degrade the same way.

// kindReportPhases lists the phases whose report headers declare the
// deliverable kind — the ONE list every kind read (DocumentCycle,
// dispatchSignals) digests and the prompt seeds gate on.
var kindReportPhases = []Phase{PhaseScout, PhaseTriage}

// kindDeclaringPhase reports whether next is a phase that declares the kind:
// its own dispatch may carry the project's default, while a later phase reads
// only what those reports declared.
func kindDeclaringPhase(next Phase) bool {
	for _, p := range kindReportPhases {
		if p == next {
			return true
		}
	}
	return false
}

// kindSignals is the single digest of the kind-declaring reports. A read that
// fails for a reason other than absence is reported here — once per read,
// with the kernel's own reason — and the signals still carry the conservative
// side; a clean absence (no report yet) is silent.
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

// dispatchSignals projects the cycle's objective signals for one dispatch of
// next. Keys are the kernel's routable field names (config.Signal*), so a
// `when` clause reads the same word a conditional_mandatory clause does. The
// kind is the DECLARED one when a report spoke (triage > scout). Before any
// declaration — a kind-declaring phase's own dispatch, on a clean digest —
// the project default (.evolve/domain.json) stands in, so a document-domain
// project's very first scout dispatch already carries the solution persona.
// Every other case is code: a later phase whose reports declared nothing
// reads exactly what the integrity floor (DocumentCycle, the tdd pin) reads,
// and a degraded digest never lets the default reclassify a torn cycle.
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

// domainDefaultKind reads the project's default deliverable kind from
// .evolve/domain.json — the ONE reader the prompt seed (seedDomainDefault) and
// the dispatch projection (dispatchSignals) share. No file ⇒ ("", false); a
// file that exists but cannot be parsed is reported loudly and counts as
// absent (the code side), never a silent reclassification.
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
