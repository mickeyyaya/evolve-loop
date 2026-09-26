package llmroute

import (
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

// Overlay is a soft dispatch adjustment: unlike a policy.Pin it only reorders the chain; zero value is a no-op.
type Overlay struct {
	CLI  string
	Tier string
}

// ApplySoftOverlay returns a new Plan with ov applied; ov.CLI promotes an exact entry, then a same-family entry, then its default driver.
func ApplySoftOverlay(in Plan, ov Overlay, prof *profiles.Profile) Plan {
	out := in
	out.Candidates = append([]string(nil), in.Candidates...)
	if ov.CLI != "" {
		// Promoting an entry the chain already holds keeps the overlay from crossing transport.
		primary := defaultDriverForFamily(ov.CLI)
		matched := false
		for _, c := range out.Candidates {
			if c == ov.CLI {
				primary = c
				matched = true
				break
			}
		}
		if !matched && !strings.Contains(ov.CLI, "-") {
			for _, c := range out.Candidates {
				if strings.HasPrefix(c, ov.CLI+"-") {
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
	if ov.Tier != "" {
		out.Model = ov.Tier
		// Dispatch launches at Tiers[0], and the router's clamp does not carry the envelope Min into the fallback floor.
		out.Tiers = TierChain(ov.Tier, envelopeMin(prof))
	}
	return out
}
