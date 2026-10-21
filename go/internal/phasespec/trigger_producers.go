package phasespec

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

func UnproducedTriggerFields(cfg config.RoutingConfig, specs []PhaseSpec, harnessProduces func(string) bool) []string {
	declared := declaredOutputSignals(specs)
	var v []string
	for _, phase := range slices.Sorted(maps.Keys(cfg.Triggers)) {
		earlier := signalsDeclaredBefore(cfg.Order, phase, declared)
		produced := func(field string) bool { return harnessProduces(field) || earlier[field] }
		block := cfg.Triggers[phase]
		v = append(v, unproducedConditions(phase, "insert_when", block.InsertWhen, produced)...)
		v = append(v, unproducedConditions(phase, "skip_when", block.SkipWhen, produced)...)
	}
	return v
}

func declaredOutputSignals(specs []PhaseSpec) map[string][]string {
	out := make(map[string][]string, len(specs))
	for _, s := range specs {
		for _, signal := range s.Outputs.Signals {
			if !strings.Contains(signal, ".") {
				signal = s.Name + "." + signal
			}
			out[s.Name] = append(out[s.Name], signal)
		}
	}
	return out
}

func signalsDeclaredBefore(order []string, phase string, declared map[string][]string) map[string]bool {
	at := slices.Index(order, phase)
	set := map[string]bool{}
	for _, earlier := range order[:max(at, 0)] {
		for _, signal := range declared[earlier] {
			set[signal] = true
		}
	}
	return set
}

func unproducedConditions(phase, key string, conds []config.Condition, produced func(string) bool) []string {
	var v []string
	for _, c := range conds {
		if !produced(c.Field) {
			v = append(v, fmt.Sprintf("phase %s %s field %s has no producer: no typed signal, and no phase before %s declares it in outputs.signals", phase, key, c.Field, phase))
		}
	}
	return v
}
