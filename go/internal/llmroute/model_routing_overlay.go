package llmroute

// Overlay is the cycle-440 MR4 SOFT dispatch adjustment: unlike a policy.Pin
// (ABSOLUTE — can collapse the chain to a single candidate), an Overlay only
// reorders the EXISTING chain. CLI (if non-empty) is promoted to primary but
// every prior candidate, including the old primary, survives — so a benched
// or failing overlay CLI still falls back via the ordinary cli-health chain
// (model_routing=auto "proposes", it never "pins"). Tier (if non-empty)
// replaces Plan.Model outright; concrete-model translation still happens
// later at bridge dispatch via the manifest's ModelTierMap. A zero-value
// Overlay is a noop.
type Overlay struct {
	CLI  string
	Tier string
}

// ApplySoftOverlay returns a NEW Plan with ov applied over in; in is never
// mutated. ov.CLI is normalized like a pin primary (defaultDriverForFamily) so
// a bare family ("codex") promotes to its registered driver ("codex-tmux");
// an already driver-qualified or unregistered name passes through unchanged.
func ApplySoftOverlay(in Plan, ov Overlay) Plan {
	out := in
	out.Candidates = append([]string(nil), in.Candidates...)
	if ov.CLI != "" {
		primary := defaultDriverForFamily(ov.CLI)
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
	}
	return out
}
