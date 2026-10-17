package cliroute

import (
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func FamilyChains(block *policy.CLIRouting) [][]string {
	if block == nil {
		return nil
	}
	var chains [][]string
	add := func(entries []string) {
		if fams := familiesOf(entries); len(fams) > 0 {
			chains = append(chains, fams)
		}
	}
	add(block.Default)
	for _, role := range sortedKeys(block.Work) {
		add(block.Work[role])
	}
	for _, agent := range sortedKeys(block.Agents) {
		add(block.Agents[agent].CLI)
	}
	for _, tier := range sortedKeys(block.Tiers) {
		add(block.Tiers[tier].CLIs)
	}
	return chains
}

func familiesOf(entries []string) []string {
	var fams []string
	for _, entry := range entries {
		name := strings.TrimSpace(entry)
		if name == "" {
			continue
		}
		if fam := familyOf(name); !slices.Contains(fams, fam) {
			fams = append(fams, fam)
		}
	}
	return fams
}
