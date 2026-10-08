package policy

import (
	"fmt"
	"slices"
	"strings"
)

const effortSourceDefault = "default"

type EffortTable struct {
	agents map[string]string
	tiers  map[string]string
}

func (p Policy) Efforts() EffortTable {
	if p.CLIRouting == nil {
		return EffortTable{}
	}
	t := EffortTable{agents: map[string]string{}, tiers: map[string]string{}}
	for key, rule := range p.CLIRouting.Agents {
		if rule.Effort != "" {
			t.agents[key] = rule.Effort
		}
	}
	for tier, rule := range p.CLIRouting.Tiers {
		if rule.Effort != "" {
			t.tiers[tier] = rule.Effort
		}
	}
	return t
}

func (t EffortTable) Resolve(tier string, agents ...string) (effort, source string) {
	for _, agent := range agents {
		if e := t.agents[agent]; e != "" {
			return e, "cli_routing.agents." + agent
		}
	}
	if e := t.tiers[tier]; e != "" {
		return e, "cli_routing.tiers." + tier
	}
	if e := defaultTierEffort[tier]; e != "" {
		return e, effortSourceDefault
	}
	return "", ""
}

func ValidateEffort(level string) error {
	if slices.Contains(effortLevels, level) {
		return nil
	}
	return fmt.Errorf("unknown effort %q (levels: %s)", level, strings.Join(effortLevels, ", "))
}
