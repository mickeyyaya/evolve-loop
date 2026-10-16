// Package modelcatalog is the live, CLI-queried tier→model map.
package modelcatalog

import "time"

const DefaultTTL = 24 * time.Hour

type Catalog struct {
	FetchedAt time.Time           `json:"fetched_at"`
	CLIs      map[string]CLIEntry `json:"clis"`
}

type CLIEntry struct {
	TierModels     map[string]string   `json:"tier_models"`
	Available      []string            `json:"available,omitempty"`
	Efforts        []string            `json:"efforts,omitempty"`
	Source         string              `json:"source,omitempty"`
	TierFallbacks  map[string][]string `json:"tier_fallbacks,omitempty"`
	CandidatesHash string              `json:"candidates_hash,omitempty"`
	FallbackReason string              `json:"fallback_reason,omitempty"`
}

func (e CLIEntry) modelForTier(tier string) (model string, ok bool) {
	if m := e.TierModels[tier]; m != "" {
		return m, true
	}
	for _, m := range e.TierFallbacks[tier] {
		if m != "" {
			return m, true
		}
	}
	return "", false
}

const SourceLive = "live"

const SourceDetect = "detect"

func (c Catalog) DispatchModel(cli, tier string) (model string, ok bool) {
	entry, found := c.CLIs[cli]
	if !found || entry.Source != SourceLive {
		return "", false
	}
	return entry.modelForTier(tier)
}

func (c Catalog) Lookup(cli, tier string) (model string, ok bool) {
	entry, found := c.CLIs[cli]
	if !found {
		return "", false
	}
	return entry.modelForTier(tier)
}

func (c Catalog) IsStale(now time.Time, ttl time.Duration) bool {
	if c.FetchedAt.IsZero() {
		return true
	}
	return !now.Before(c.FetchedAt.Add(ttl))
}

func (c Catalog) Empty() bool {
	return len(c.CLIs) == 0
}
