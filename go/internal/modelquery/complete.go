package modelquery

import "github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"

// CompleteTiers fills any canonical tier the classifier omitted from a
// documented nearest-neighbour ladder over modelcatalog.CanonicalTiers
// (fast→top order): ascending distance, preferring the MORE capable side on
// ties — a deep task degrades better to the frontier model than to a mid one.
// It never invents an id: every fill reuses a model sanitizeTierMap already
// validated against the offered list. An empty input has nothing to borrow
// and stays empty (the caller's fallback path handles that). Pure — returns a
// new map.
//
// This is what keeps the reuse gate's full-coverage condition satisfiable:
// entries written by the current pipeline always carry every canonical tier,
// so an unchanged offering can be reused without a classifier call.
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

// neighbourOrder returns the other indices of a length-n tier vocabulary
// sorted by ascending distance from i, higher (more capable) index first on
// equal distance.
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
