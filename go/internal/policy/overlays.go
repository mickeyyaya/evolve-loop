// overlays.go — skill-overlay resolver (cycle-609 skill-overlays-bridge-layer).
//
// The nil-able-pointer + resolver idiom mirrors ObserverPolicy/ObserverConfig
// (policy.go): Policy.Overlays == nil ⇒ the compiled default applies; a non-nil
// block with an empty Rules slice is an explicit operator opt-out (zero
// overlays, NOT the default); a non-nil block with rules resolves the UNION of
// every matching rule's skills, deduped, in stable (first-seen) order.
//
// This is the policy-layer resolver only — a pure, side-effect-free mapping
// from a dispatch descriptor to an ordered skill list. It carries no producer:
// nothing constructs an OverlayDispatch from a live BridgeRequest yet, so the
// audit-F1 protected-surface gap (skill dirs referenced by overlays) stays
// dormant. Wiring Engine.Launch to inject these skills — the Tier producer that
// makes F1 reachable — MUST land in the same slice that extends
// protectedSurfaceFragments (guards/integrity_surface.go), and that file is on
// the control-plane boundary (no autonomous --class cycle may edit it). So the
// injection seam is intentionally deferred to an out-of-cycle manual ship; see
// build-report.md § Deferred.
package policy

import "path"

// OverlaysPolicy is the operator-configurable skill-overlay block. Rules are
// evaluated in order; a dispatch may match several (their skills union).
type OverlaysPolicy struct {
	Rules []OverlayRule `json:"rules,omitempty"`
}

// OverlayRule matches a dispatch when EVERY non-empty selector dimension
// matches. An empty dimension is a wildcard. Patterns are glob (path.Match), so
// a literal like "audit" matches only itself while "gpt-*" matches "gpt-5".
type OverlayRule struct {
	Phases []string `json:"phases,omitempty"`
	CLIs   []string `json:"clis,omitempty"`
	Models []string `json:"models,omitempty"`
	Tiers  []string `json:"tiers,omitempty"`
	Skills []string `json:"skills,omitempty"`
}

// OverlayDispatch is the descriptor a resolver call is keyed on — the phase,
// driver (cli), model, and capability tier of a single agent launch.
type OverlayDispatch struct {
	Phase string
	CLI   string
	Model string
	Tier  string
}

// compiledDefaultOverlays is the single source of the built-in overlay set —
// the ONLY Go literal tier→skills mapping in this package. Operators override
// it wholesale via policy.json's overlays block (absent ⇒ this default; empty
// rules ⇒ opt out).
func compiledDefaultOverlays() []OverlayRule {
	return []OverlayRule{
		{Tiers: []string{"deep", "top"}, Skills: []string{"fable-mode"}},
	}
}

// ResolveOverlays returns the ordered, deduped skill list that applies to the
// given dispatch. See the file header for the nil/empty/rules contract.
func (p Policy) ResolveOverlays(d OverlayDispatch) []string {
	rules := compiledDefaultOverlays()
	if p.Overlays != nil {
		rules = p.Overlays.Rules // explicit block (possibly empty ⇒ opt-out)
	}
	var out []string
	seen := map[string]struct{}{}
	for _, r := range rules {
		if !r.matches(d) {
			continue
		}
		for _, s := range r.Skills {
			if s == "" {
				continue
			}
			if _, dup := seen[s]; dup {
				continue
			}
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}
	return out
}

// matches reports whether every non-empty selector dimension of the rule is
// satisfied by the dispatch.
func (r OverlayRule) matches(d OverlayDispatch) bool {
	return matchDim(r.Phases, d.Phase) &&
		matchDim(r.CLIs, d.CLI) &&
		matchDim(r.Models, d.Model) &&
		matchDim(r.Tiers, d.Tier)
}

// matchDim reports whether value satisfies a selector dimension. An empty
// patterns slice is a wildcard (matches anything); otherwise value must
// glob-match at least one pattern.
func matchDim(patterns []string, value string) bool {
	if len(patterns) == 0 {
		return true
	}
	for _, pat := range patterns {
		if ok, err := path.Match(pat, value); err == nil && ok {
			return true
		}
	}
	return false
}
