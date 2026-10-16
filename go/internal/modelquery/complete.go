package modelquery

import "github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"

func CompleteTiers(tiers map[string]string) map[string]string {
	out := make(map[string]string, len(modelcatalog.CanonicalTiers))
	for tier, model := range tiers {
		out[tier] = model
	}
	if len(out) == 0 {
		return out
	}
	order := modelcatalog.CanonicalTiers
	for i, tier := range order {
		if out[tier] != "" {
			continue
		}
		for _, j := range neighbourOrder(i, len(order)) {
			if model := tiers[order[j]]; model != "" {
				out[tier] = model
				break
			}
		}
	}
	return out
}

func neighbourOrder(i, n int) []int {
	out := make([]int, 0, n-1)
	for d := 1; d < n; d++ {
		if i+d < n {
			out = append(out, i+d)
		}
		if i-d >= 0 {
			out = append(out, i-d)
		}
	}
	return out
}
