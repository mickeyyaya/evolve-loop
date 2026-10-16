package modelquery

type FreshnessPolicy struct {
	PreferAlias bool
	AliasIDs    []string
}

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

func PromoteLatest(sel map[string]string, candidates []string, p FreshnessPolicy) map[string]string {
	buckets := GroupByLineage(candidates)
	out := make(map[string]string, len(sel))
	for tier, model := range sel {
		out[tier] = model
		lineage := buckets[LineageKey(model)]
		if len(lineage) == 0 {
			continue
		}
		if freshest := p.Freshest(incumbentFirst(lineage, model)); freshest != "" {
			out[tier] = freshest
		}
	}
	return out
}

func incumbentFirst(lineage []string, sel string) []string {
	for i, id := range lineage {
		if id == sel {
			out := make([]string, 0, len(lineage))
			out = append(out, sel)
			out = append(out, lineage[:i]...)
			return append(out, lineage[i+1:]...)
		}
	}
	return lineage
}
