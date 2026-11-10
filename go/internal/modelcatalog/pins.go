package modelcatalog

import (
	"maps"
	"slices"
)

func PinTiers(tiers, pins map[string]string, offered []string) (map[string]string, []string) {
	if len(pins) == 0 {
		return tiers, nil
	}
	pinned := maps.Clone(tiers)
	if pinned == nil {
		pinned = make(map[string]string, len(pins))
	}
	unmet := make(map[string]string)
	for tier, model := range pins {
		if !slices.Contains(offered, model) {
			unmet[tier] = model
			continue
		}
		pinned[tier] = model
	}
	if len(unmet) == 0 {
		return pinned, nil
	}
	return pinned, PinPairs(unmet)
}

func PinPairs(pins map[string]string) []string {
	pairs := make([]string, 0, len(pins))
	for _, tier := range slices.Sorted(maps.Keys(pins)) {
		pairs = append(pairs, tier+"="+pins[tier])
	}
	return pairs
}
