package setup

import "github.com/mickeyyaya/evolve-loop/go/internal/modelquery"

type FamilyLatest struct {
	CLI              string      `json:"cli"`
	CurrentDeepModel string      `json:"current_deep_model"`
	LatestModel      string      `json:"latest_model,omitempty"`
	CurrentSeenLive  bool        `json:"current_seen_live"`
	StaleTiers       []TierStale `json:"stale_tiers,omitempty"`
	UnverifiedTiers  []string    `json:"unverified_tiers,omitempty"`
	MapStale         bool        `json:"map_stale"`
	Candidates       int         `json:"candidates"`
	Error            string      `json:"error,omitempty"`
}

type TierStale struct {
	Tier    string `json:"tier"`
	Current string `json:"current"`
	Latest  string `json:"latest"`
}

type LatestReport struct {
	Source string         `json:"source"`
	CLIs   []FamilyLatest `json:"clis"`
}

func ComputeLatest(current string, candidates []string, fp modelquery.FreshnessPolicy) (latest string, stale, observed bool) {
	if current == "" {
		return "", false, false
	}
	key := modelquery.LineageKey(current)
	bucket := []string{current}
	for _, id := range candidates {
		if id == current {
			observed = true
			continue
		}
		if modelquery.LineageKey(id) == key {
			bucket = append(bucket, id)
			observed = true
		}
	}
	latest = fp.Freshest(bucket)
	if latest == "" {
		latest = current
	}
	return latest, latest != current, observed
}
