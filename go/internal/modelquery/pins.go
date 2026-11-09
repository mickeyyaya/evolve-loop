package modelquery

import (
	"slices"
	"sort"
)

func pinTiers(tiers, pins map[string]string, offered []string) (map[string]string, []string) {
	if len(pins) == 0 {
		return tiers, nil
	}
	pinned := make(map[string]string, len(tiers)+len(pins))
	for tier, model := range tiers {
		pinned[tier] = model
	}
	var unmet []string
	for _, tier := range sortedKeys(pins) {
		if !slices.Contains(offered, pins[tier]) {
			unmet = append(unmet, tier+"="+pins[tier])
			continue
		}
		pinned[tier] = pins[tier]
	}
	return pinned, unmet
}

func pinPairs(pins map[string]string) []string {
	pairs := make([]string, 0, len(pins))
	for _, tier := range sortedKeys(pins) {
		pairs = append(pairs, tier+"="+pins[tier])
	}
	return pairs
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
