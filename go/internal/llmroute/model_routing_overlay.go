package llmroute

import (
	"cmp"

	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

type Overlay struct {
	CLI        string
	Tier       string
	LadderTier string
	Effort     string
}

func ApplySoftOverlay(in Plan, ov Overlay, prof *profiles.Profile) Plan {
	out := in
	out.Candidates = append([]string(nil), in.Candidates...)
	if ov.CLI != "" {
		primary := defaultDriverForFamily(ov.CLI)
		matched := false
		for _, c := range out.Candidates {
			if c == ov.CLI {
				primary = c
				matched = true
				break
			}
		}
		if !matched && !KnownDriver(ov.CLI) {
			for _, c := range out.Candidates {
				if Family(c) == ov.CLI {
					primary = c
					break
				}
			}
		}
		candidates := make([]string, 0, len(out.Candidates)+1)
		candidates = append(candidates, primary)
		seen := map[string]struct{}{primary: {}}
		for _, c := range out.Candidates {
			if _, dup := seen[c]; dup {
				continue
			}
			seen[c] = struct{}{}
			candidates = append(candidates, c)
		}
		out.Candidates = candidates
	}
	if tier := cmp.Or(ov.LadderTier, ov.Tier); tier != "" {
		out.Model = tier
		out.Tiers = TierChain(tier, envelopeMin(prof))
	}
	if ov.Effort != "" {
		out.Effort = ov.Effort
	}
	return out
}
