package modelcatalog

import "time"

// CanonicalTiers are the abstract model tiers a catalog entry may carry, in
// fast→top order. A snapshot's tier keys outside this set are dropped so the
// catalog never carries a tier dispatch can't ask for. "top" is the frontier
// tier (default when a profile/advisor is silent); "high" is an input alias of
// "deep" (see bridge.translateV1TierKey), not a canonical tier of its own.
var CanonicalTiers = []string{"fast", "balanced", "deep", "top"}

// CLISnapshot is the minimal per-CLI input BuildFromSnapshots needs. It is
// deliberately decoupled from setup.CLIStatus so this package stays a leaf
// (no setup → bridge import chain): the command layer adapts CLIStatus into
// CLISnapshot at the call site.
type CLISnapshot struct {
	// CLI is the base CLI name (claude|codex|agy|…), already normalized.
	CLI string
	// Ready reports whether the CLI is installed AND usable (authed). Only
	// ready CLIs are cataloged — a blocked/misconfigured CLI's model list is
	// not something dispatch should resolve against.
	Ready bool
	// TierModels maps each tier to the CLI's concrete model id, as the source
	// currently reports it. Empty models and non-canonical tiers are ignored.
	TierModels map[string]string
	// Available is the raw enumerated model-id list (audit trail), passed
	// through to CLIEntry.Available. Optional; nil for detect-derived snapshots.
	Available []string
	// Source is the provenance written to CLIEntry.Source (SourceLive /
	// SourceDetect). Empty defaults to SourceDetect — a snapshot of unknown
	// provenance is treated as the non-authoritative kind.
	Source string
}

// BuildFromSnapshots assembles a Catalog from per-CLI snapshots, stamping
// fetchedAt. It is pure and deterministic (Rule 5: the ready/tier filtering is
// mechanical, not a judgment call): a CLI is included only when it is ready,
// named, and contributes at least one canonical-tier model with a non-empty id.
//
// This is the bootstrap source — it mirrors whatever the caller's detector
// reports (today: setup.Detect's manifest-derived tier_models). The live
// `/model`-queried source is a future, higher-fidelity producer of the same
// Catalog shape; this function is unaffected by that swap.
func BuildFromSnapshots(snaps []CLISnapshot, fetchedAt time.Time) Catalog {
	cat := Catalog{FetchedAt: fetchedAt, CLIs: make(map[string]CLIEntry)}
	for _, s := range snaps {
		if !s.Ready || s.CLI == "" {
			continue
		}
		tiers := make(map[string]string)
		for _, tier := range CanonicalTiers {
			if model := s.TierModels[tier]; model != "" {
				tiers[tier] = model
			}
		}
		if len(tiers) == 0 {
			continue
		}
		source := s.Source
		if source == "" {
			source = SourceDetect
		}
		cat.CLIs[s.CLI] = CLIEntry{TierModels: tiers, Available: s.Available, Source: source}
	}
	return cat
}

// MergeFallbacks carries operator-authored TierFallbacks from a prior catalog
// into a freshly built one, so `evolve models refresh` (which rebuilds and
// rewrites the cache wholesale) does not silently destroy fallback chains.
// It fills gaps only: a fresh entry that already carries its own chain keeps
// it, and a CLI absent from next is never resurrected just to preserve its old
// chain. Pure — neither input is mutated; the returned catalog is next with a
// copied CLIs map and, where filled, a copied fallbacks map.
func MergeFallbacks(prior, next Catalog) Catalog {
	merged := next
	merged.CLIs = make(map[string]CLIEntry, len(next.CLIs))
	for name, entry := range next.CLIs {
		if len(entry.TierFallbacks) == 0 {
			if chains := prior.CLIs[name].TierFallbacks; len(chains) > 0 {
				copied := make(map[string][]string, len(chains))
				for tier, chain := range chains {
					copied[tier] = append([]string(nil), chain...)
				}
				entry.TierFallbacks = copied
			}
		}
		merged.CLIs[name] = entry
	}
	return merged
}
