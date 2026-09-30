package core

import (
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func (o *Orchestrator) unavailablePhaseReason(name string) error {
	r, ok := o.runners[Phase(name)]
	if !ok {
		return nil
	}
	p, ok := r.(PersonaProber)
	if !ok {
		return nil
	}
	err := p.PersonaAvailable()
	if err == nil {
		return nil
	}
	if !errors.Is(err, ErrAgentDocMissing) {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN plan: persona probe for %s failed (%v) — the phase stays selectable; dispatch reports the real cause\n", name, err)
		return nil
	}
	return err
}

func (o *Orchestrator) unavailableOptionalPhases() []string {
	var out []string
	for _, spec := range o.catalog.All() {
		if !o.optionalInfraSkip(Phase(spec.Name), ErrAgentDocMissing) {
			continue
		}
		if r, ok := o.runners[Phase(spec.Name)]; ok {
			if _, probeable := r.(PersonaProber); !probeable {
				fmt.Fprintf(os.Stderr, "[orchestrator] WARN plan: optional phase %s cannot be probed for its persona (runner is not a PersonaProber) — it stays selectable; a missing doc surfaces only at dispatch\n", spec.Name)
			}
		}
		if err := o.unavailablePhaseReason(spec.Name); err != nil {
			fmt.Fprintf(os.Stderr, "[orchestrator] WARN plan: optional phase %s persona doc missing (%v) — excluded from the selectable set this cycle; provide the persona or take the phase off the menu\n", spec.Name, err)
			out = append(out, spec.Name)
		}
	}
	slices.Sort(out)
	return out
}

func withoutCards(cards []router.PhaseCard, unavailable []string) []router.PhaseCard {
	if len(unavailable) == 0 {
		return cards
	}
	out := make([]router.PhaseCard, 0, len(cards))
	for _, c := range cards {
		if !slices.Contains(unavailable, c.Name) {
			out = append(out, c)
		}
	}
	return out
}

func withoutNames(names, unavailable []string) []string {
	if len(unavailable) == 0 {
		return names
	}
	out := make([]string, 0, len(names))
	for _, n := range names {
		if !slices.Contains(unavailable, n) {
			out = append(out, n)
		}
	}
	return out
}
