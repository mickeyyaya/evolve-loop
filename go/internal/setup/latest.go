package setup

// latest.go — the pure core of `evolve setup latest`: given what dispatch
// would use today (the catalog-first tier map) and what the CLI bridge
// LIVE-reports as available, decide whether a fresher model exists in the SAME
// lineage. Operator directive: /evo:setup must surface the newest model each
// bridge actually offers as an option, not silently trust a possibly-stale
// tier map — across ALL ready CLIs, probed in parallel.
//
// Pins stay abstract tiers (policy.ValidatePin rejects native ids), so "adopt
// the latest" is never a pin — it is a catalog refresh (`evolve models
// refresh`), and this probe is the read-only staleness check that justifies
// offering one.

import "github.com/mickeyyaya/evolve-loop/go/internal/modelquery"

// FamilyLatest is one CLI family's live-latest verdict.
type FamilyLatest struct {
	CLI string `json:"cli"`
	// CurrentDeepModel is what dispatch resolves for the deep tier today —
	// catalog when present (the hot-reloading dispatch authority), manifest
	// tier map otherwise. The display anchor; staleness covers every tier.
	CurrentDeepModel string `json:"current_deep_model"`
	// LatestModel is the freshest member of CurrentDeepModel's OWN lineage
	// among the live candidates (promotion never crosses lineages).
	LatestModel string `json:"latest_model,omitempty"`
	// CurrentSeenLive reports whether the deep model (or a lineage sibling)
	// actually appeared in the live capture. False means the probe can say
	// NOTHING about freshness — a "freshest in its line" verdict over a
	// never-seen model would be fabricated (review finding: the identity-
	// fallback tier word and a vanished model both hit this).
	CurrentSeenLive bool `json:"current_seen_live"`
	// StaleTiers carries every stale tier's operator-quotable evidence —
	// staleness is judged across the WHOLE tier map, not only deep (most
	// phases dispatch balanced), and a bare tier name proved unquotable: the
	// deep-anchored display fields degenerated to "dispatch uses X, bridge
	// offers X" for a balanced-only staleness (review finding).
	StaleTiers []TierStale `json:"stale_tiers,omitempty"`
	// UnverifiedTiers names tiers whose mapped model never appeared in the
	// live capture (nor any lineage sibling) — per-tier honesty for the
	// vanished-model case CurrentSeenLive (deep-anchored) cannot express.
	UnverifiedTiers []string `json:"unverified_tiers,omitempty"`
	// MapStale is len(StaleTiers) > 0 — the condition under which the setup
	// skill offers a catalog refresh.
	MapStale bool `json:"map_stale"`
	// Candidates counts the live-captured ids (0 with Error set = probe failed).
	Candidates int    `json:"candidates"`
	Error      string `json:"error,omitempty"`
}

// TierStale is one stale tier's evidence: the pair the setup skill quotes
// verbatim in its adopt-latest question.
type TierStale struct {
	Tier    string `json:"tier"`
	Current string `json:"current"`
	Latest  string `json:"latest"`
}

// LatestReport is the digest `evolve setup latest --json` emits.
type LatestReport struct {
	Source string         `json:"source"` // always "live" — that is the point
	CLIs   []FamilyLatest `json:"clis"`
}

// ComputeLatest picks the freshest model in current's lineage among the live
// candidates, under the CLI's freshness policy (alias-resolving CLIs prefer
// the alias). Pure. stale is true only when the pick differs from current — a
// fresher model in a DIFFERENT lineage never marks the map stale. observed
// reports whether current (or a lineage sibling) appeared in candidates at
// all: when false, latest degenerates to current and stale to false, and the
// caller must present "not seen live", never "confirmed freshest".
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
