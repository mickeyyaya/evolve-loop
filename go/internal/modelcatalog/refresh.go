package modelcatalog

import "time"

var CanonicalTiers = []string{"fast", "balanced", "deep", "top"}

type CLISnapshot struct {
	CLI            string
	Ready          bool
	TierModels     map[string]string
	Available      []string
	Efforts        []string
	Source         string
	CandidatesHash string
	FallbackReason string
}

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
		cat.CLIs[s.CLI] = CLIEntry{
			TierModels: tiers, Available: s.Available, Efforts: s.Efforts,
			Source: source, CandidatesHash: s.CandidatesHash, FallbackReason: s.FallbackReason,
		}
	}
	return cat
}

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
