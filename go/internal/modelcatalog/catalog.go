// Package modelcatalog is the live, CLI-queried tier→model map.
//
// At dispatch a phase carries an abstract model tier ("fast" | "balanced" |
// "deep"); that tier must be translated to a CLI-native concrete model id.
// Today that translation comes from the static, embedded bridge manifest
// (bridge.Manifest.ModelTierMap). This package holds the same shape but
// sourced LIVE from each installed CLI and cached to
// .evolve/model-catalog.json with a fetched_at timestamp, refreshed at cycle
// start when older than the TTL (1 day).
//
// This file is the pure data layer (slice 10a): schema + lookup + staleness,
// zero side effects and no dependency on the bridge. The dispatch read-path
// deliberately keeps the manifest fallback at the CALL SITE — having this
// package import bridge would create an import cycle (the realizer that needs
// the lookup lives in bridge). Lookup returns (model, ok); an empty/missing
// catalog yields ok=false everywhere, so the caller falls back to the static
// manifest and behavior is unchanged until a catalog is populated.
package modelcatalog

import "time"

// DefaultTTL is the cache lifetime. Within the window the cached catalog is
// reused verbatim (reproducibility across a cycle); past it, the cycle-start
// hook refreshes once at the boundary.
const DefaultTTL = 24 * time.Hour

// Catalog is the cached per-CLI tier→model map. Keyed by BASE cli name
// ("claude" | "codex" | "agy") — driver suffixes (-tmux, -p) are normalized
// away by the caller before Lookup so one entry serves every driver variant.
type Catalog struct {
	// FetchedAt is when this catalog was last refreshed. A zero value means
	// "never fetched" and is always stale.
	FetchedAt time.Time `json:"fetched_at"`
	// CLIs maps a base CLI name to its tier table.
	CLIs map[string]CLIEntry `json:"clis"`
}

// CLIEntry is one CLI's model information.
type CLIEntry struct {
	// TierModels translates the abstract tier (fast|balanced|deep) to this
	// CLI's concrete model id — the live analogue of
	// bridge.Manifest.ModelTierMap.
	TierModels map[string]string `json:"tier_models"`
	// Available is the raw enumerated model-id list as offered by the CLI, kept
	// as an audit/debug trail. Not consumed at dispatch.
	Available []string `json:"available,omitempty"`
	// Source records provenance: "live" (queried from the CLI itself) vs
	// "detect" (derived from the static, possibly-degenerate manifest map).
	// Only "live" entries are trustworthy enough to OVERRIDE the manifest at
	// dispatch; "detect" entries are informational (shown by `models list`).
	Source string `json:"source,omitempty"`
	// TierFallbacks is the operator-authored per-tier fallback chain: when
	// TierModels[tier] is empty, the first non-empty model in
	// TierFallbacks[tier] is used instead. Absent or exhausted chains degrade
	// to the single-shot behavior (ok=false → manifest fallback at the caller).
	TierFallbacks map[string][]string `json:"tier_fallbacks,omitempty"`
}

// modelForTier resolves the entry's model for tier: the primary TierModels
// mapping when non-empty, else the first non-empty model in the tier's
// TierFallbacks chain. ok=false when both are empty/absent.
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

// SourceLive marks a tier map queried from the CLI itself (authoritative).
const SourceLive = "live"

// SourceDetect marks a tier map derived from the static manifest (informational).
const SourceDetect = "detect"

// DispatchModel returns the concrete model for (cli, tier) ONLY when the entry
// is live-sourced — the gate that keeps a degenerate detect-derived catalog
// from overriding the proven static manifest at dispatch. Non-live or missing
// entries return ok=false so the caller falls back to the manifest.
func (c Catalog) DispatchModel(cli, tier string) (model string, ok bool) {
	entry, found := c.CLIs[cli]
	if !found || entry.Source != SourceLive {
		return "", false
	}
	return entry.modelForTier(tier)
}

// Lookup returns the concrete model for (cli, tier), consulting the tier's
// fallback chain when the primary mapping is empty or absent. ok is false when
// the CLI is unknown or neither the primary nor the chain yields a model — in
// every such case the caller must fall back to the static manifest. cli must
// already be a base name (no -tmux/-p suffix).
func (c Catalog) Lookup(cli, tier string) (model string, ok bool) {
	entry, found := c.CLIs[cli]
	if !found {
		return "", false
	}
	return entry.modelForTier(tier)
}

// IsStale reports whether the catalog must be refreshed: it was never fetched
// (zero FetchedAt) or now is at/after FetchedAt+ttl. A future FetchedAt (clock
// skew) is treated as fresh rather than refreshing on every cycle.
func (c Catalog) IsStale(now time.Time, ttl time.Duration) bool {
	if c.FetchedAt.IsZero() {
		return true
	}
	return !now.Before(c.FetchedAt.Add(ttl))
}

// Empty reports whether the catalog carries no CLI entries.
func (c Catalog) Empty() bool {
	return len(c.CLIs) == 0
}
