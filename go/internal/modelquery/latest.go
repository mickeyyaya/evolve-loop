package modelquery

// FreshnessPolicy declares how ONE CLI's "latest within a lineage" is chosen.
// The ZERO VALUE is the enumerating-CLI default: newest concrete version via
// NewestInLineage. It is data declared per CLI in the bridge manifest
// ("model_freshness") and threaded here as a plain value by the composition
// root — modelquery never imports bridge, and no CLI name appears in any
// conditional in this package.
type FreshnessPolicy struct {
	// PreferAlias marks a CLI that resolves a bare family alias ("opus") to
	// that family's newest release at LAUNCH. For such a CLI the alias is
	// strictly fresher than any concrete id a catalog could cache — caching
	// "opus-4.6" would freeze the version the alias would have moved past.
	PreferAlias bool
	// AliasIDs are the ids the CLI self-resolves, in preference order.
	AliasIDs []string
}

// Freshest picks the freshest id from ONE lineage bucket. Under PreferAlias
// the first AliasIDs member present in the bucket wins; otherwise (or when no
// alias member is present) the newest concrete version wins via
// NewestInLineage — which is composed in front of, never modified: its
// versioned-beats-unversioned rule is correct for enumerating CLIs and only
// backwards for alias-resolving ones, so the alias rule sits before it.
func (p FreshnessPolicy) Freshest(lineage []string) string {
	if p.PreferAlias {
		present := make(map[string]bool, len(lineage))
		for _, id := range lineage {
			present[id] = true
		}
		for _, alias := range p.AliasIDs {
			if present[alias] {
				return alias
			}
		}
	}
	return NewestInLineage(lineage)
}

// PromoteLatest upgrades each selected tier model to the freshest member of
// ITS OWN lineage bucket among candidates. The classifier keeps 100% of the
// qualitative tier decision (which line serves this tier); Go keeps 100% of
// the numeric one (which version of that line is newest). A selection absent
// from candidates is kept verbatim — promotion never invents or crosses
// buckets. Pure: returns a new map, inputs are not mutated.
func PromoteLatest(sel map[string]string, candidates []string, p FreshnessPolicy) map[string]string {
	buckets := GroupByLineage(candidates)
	out := make(map[string]string, len(sel))
	for tier, model := range sel {
		out[tier] = model
		lineage := buckets[LineageKey(model)]
		if len(lineage) == 0 {
			continue
		}
		if freshest := p.Freshest(lineage); freshest != "" {
			out[tier] = freshest
		}
	}
	return out
}
