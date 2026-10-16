package core

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

type StateMachine struct {
	allowed map[Phase]map[Phase]bool
	specFor func(Phase) (phasespec.PhaseSpec, bool)
	spine   []Phase
}

var spineOrder = []Phase{
	PhaseIntent,
	PhaseScout,
	PhaseTriage,
	PhaseTDD,
	PhaseBuildPlanner,
	PhaseBuild,
	PhaseAudit,
	PhaseShip,
	PhaseEnd,
}

func (sm *StateMachine) effectiveSpine() []Phase {
	if len(sm.spine) > 0 {
		return sm.spine
	}
	return spineOrder
}

func (sm *StateMachine) spineNext(p Phase) (Phase, bool) {
	spine := sm.effectiveSpine()
	for i, sp := range spine {
		if sp == p && i+1 < len(spine) {
			return spine[i+1], true
		}
	}
	return "", false
}

func (sm *StateMachine) WithSpine(order []Phase) *StateMachine {
	sm.spine = order
	return sm
}

func spinePhasesFrom(names []string) []Phase {
	var out []Phase
	for _, n := range names {
		if p := phaseFromRouter(n); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func legalGraphFrom(successors map[string][]string) map[Phase]map[Phase]bool {
	if len(successors) == 0 {
		return nil
	}
	graph := make(map[Phase]map[Phase]bool, len(successors))
	for fromName, tos := range successors {
		from := phaseFromRouter(fromName)
		if from == "" {
			continue
		}
		edges := make(map[Phase]bool, len(tos))
		for _, toName := range tos {
			if to := phaseFromRouter(toName); to != "" {
				edges[to] = true
			}
		}
		graph[from] = edges
	}
	return graph
}

func (sm *StateMachine) WithLegalGraph(graph map[Phase]map[Phase]bool) *StateMachine {
	if len(graph) > 0 {
		sm.allowed = graph
	}
	return sm
}

func (sm *StateMachine) WithCatalog(specFor func(Phase) (phasespec.PhaseSpec, bool)) *StateMachine {
	sm.specFor = specFor
	return sm
}

func NewStateMachine() *StateMachine {
	a := map[Phase]map[Phase]bool{
		PhaseStart:        {PhaseIntent: true, PhaseScout: true},
		PhaseIntent:       {PhaseScout: true},
		PhaseScout:        {PhaseTriage: true, PhaseTDD: true, PhaseBuild: true, PhaseEnd: true},
		PhaseTriage:       {PhaseTDD: true, PhaseBuild: true, PhaseEnd: true},
		PhaseTDD:          {PhaseBuildPlanner: true, PhaseBuild: true},
		PhaseBuildPlanner: {PhaseBuild: true},
		PhaseBuild:        {PhaseAudit: true},
		PhaseAudit:        {PhaseShip: true, PhaseRetro: true, PhaseTDD: true, PhaseBuild: true},
		PhaseRetro:        {PhaseShip: true, PhaseTDD: true, PhaseEnd: true, PhaseAudit: true},
		PhaseShip:         {PhaseEnd: true, PhaseDebugger: true, PhaseAudit: true, PhaseBuild: true, PhaseTDD: true, PhaseShip: true},
		PhaseDebugger:     {PhaseShip: true, PhaseAudit: true, PhaseBuild: true, PhaseTDD: true, PhaseEnd: true},
		PhaseEnd:          {},
	}
	return &StateMachine{allowed: a}
}

func (sm *StateMachine) CanTerminateEarly(from Phase, shipPlanned bool) bool {
	if shipPlanned {
		return false
	}
	if sm.specFor != nil {
		if spec, ok := sm.specFor(from); ok && spec.EarlyExit != nil {
			return *spec.EarlyExit
		}
	}
	switch from {
	case PhaseScout, PhaseTriage:
		return true
	default:
		return false
	}
}

func (sm *StateMachine) CanTransition(from, to Phase) bool {
	if !from.IsValid() || !to.IsValid() {
		return false
	}
	return sm.allowed[from][to]
}

func (sm *StateMachine) NextFromStart(intentRequired bool) Phase {
	if intentRequired {
		return PhaseIntent
	}
	return PhaseScout
}

func (sm *StateMachine) Next(current Phase, verdict string) (Phase, error) {
	if !current.IsValid() {
		return "", fmt.Errorf("%w: %s", ErrPhaseInvalid, current)
	}
	if sm.specFor != nil {
		if spec, ok := sm.specFor(current); ok && spec.OnPass != "" && spec.OnFail != "" {
			switch verdict {
			case VerdictPASS, VerdictWARN:
				if next := phaseFromRouter(spec.OnPass); next != "" {
					return next, nil
				}
				return "", fmt.Errorf("%w: %s on_pass %q resolves to no known phase", ErrTransitionInvalid, current, spec.OnPass)
			case VerdictFAIL:
				if next := phaseFromRouter(spec.OnFail); next != "" {
					return next, nil
				}
				return "", fmt.Errorf("%w: %s on_fail %q resolves to no known phase", ErrTransitionInvalid, current, spec.OnFail)
			default:
				return "", fmt.Errorf("%w: %s verdict %q", ErrTransitionInvalid, current, verdict)
			}
		}
	}
	switch current {
	case PhaseStart:
		return PhaseScout, nil
	case PhaseAudit:
		switch verdict {
		case VerdictPASS, VerdictWARN:
			return PhaseShip, nil
		case VerdictFAIL:
			return PhaseRetro, nil
		default:
			return "", fmt.Errorf("%w: audit verdict %q", ErrTransitionInvalid, verdict)
		}
	case PhaseDebugger, PhaseRetro:
		return PhaseEnd, nil
	case PhaseEnd:
		return "", fmt.Errorf("%w: end is terminal", ErrTransitionInvalid)
	}
	if next, ok := sm.spineNext(current); ok {
		return next, nil
	}
	return "", fmt.Errorf("%w: no successor for %s", ErrTransitionInvalid, current)
}

func mandatoryAnchorsFor(cfg config.RoutingConfig) []Phase {
	var anchors []Phase
	for _, name := range effectiveOrder(cfg) {
		if !isConfiguredMandatory(cfg, name) {
			continue
		}
		if p := phaseFromRouter(name); p != "" {
			anchors = append(anchors, p)
		}
	}
	return anchors
}

func effectiveOrder(cfg config.RoutingConfig) []string {
	if len(cfg.Order) > 0 {
		return cfg.Order
	}
	return cfg.Mandatory
}

func (sm *StateMachine) SpineSatisfiedUpTo(target Phase, sig router.RoutingSignals, cfg config.RoutingConfig) bool {
	_, unsatisfied := sm.UnsatisfiedSpineAnchor(target, sig, cfg)
	return !unsatisfied
}

func (sm *StateMachine) UnsatisfiedSpineAnchor(target Phase, sig router.RoutingSignals, cfg config.RoutingConfig) (Phase, bool) {
	anchors := mandatoryAnchorsFor(cfg)
	ti := -1
	for i, a := range anchors {
		if a == target {
			ti = i
			break
		}
	}
	if ti < 0 {
		return "", false
	}
	for i := 0; i < ti; i++ {
		if !sm.gateSatisfied(anchors[i], sig) {
			return anchors[i], true
		}
	}
	return "", false
}

func (sm *StateMachine) gateSatisfied(anchor Phase, sig router.RoutingSignals) bool {
	if sm.specFor != nil {
		if spec, ok := sm.specFor(anchor); ok && spec.Gate != nil {
			present, verdict := digestSignalFor(anchor, sig)
			if spec.Gate.RequiresPresent && !present {
				return false
			}
			if len(spec.Gate.VerdictIn) > 0 && !containsString(spec.Gate.VerdictIn, verdict) {
				return false
			}
			return true
		}
	}
	return anchorArtifactPresent(anchor, sig)
}

func digestSignalFor(anchor Phase, sig router.RoutingSignals) (present bool, verdict string) {
	switch anchor {
	case PhaseScout:
		return sig.Scout.Present, ""
	case PhaseBuild:
		return sig.Build.Present, ""
	case PhaseAudit:
		return sig.Audit.Present, sig.Audit.Verdict
	}
	return false, ""
}

func anchorArtifactPresent(anchor Phase, sig router.RoutingSignals) bool {
	switch anchor {
	case PhaseScout:
		return sig.Scout.Present
	case PhaseBuild:
		return sig.Build.Present
	case PhaseAudit:
		return sig.Audit.Present && (sig.Audit.Verdict == VerdictPASS || sig.Audit.Verdict == VerdictWARN)
	case PhaseShip:
		return true
	}
	return true
}

func containsString(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func isConfiguredMandatory(cfg config.RoutingConfig, phase string) bool {
	for _, m := range cfg.Mandatory {
		if m == phase {
			return true
		}
	}
	return false
}
