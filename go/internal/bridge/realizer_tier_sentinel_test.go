package bridge

// realizer_tier_sentinel_test.go — the generalized unresolved-model-token
// invariant. ADR-0044 C2/D3 (cycle-262) established that the "auto" resolve-me
// sentinel must never reach a CLI: `claude --model auto` boots into the fatal
// "There's an issue with the selected model (auto)" pane. An ABSTRACT TIER NAME
// reaching the same emit point is the identical failure for the identical
// reason — it means model_tier_map translation fell through, so the value is a
// vocabulary token, not a model.
//
// This is live today: claude-tmux.json declares no "top" entry, and
// translateV1TierKey("top") is a pass-through, so realizeScalar leaves
// resolved=="top" and the flag channel emits `claude --model top`. The tier is
// reachable — universalTierFloor (router/model_routing_clamp.go:27) has
// Max:"top" and 60+ profiles carry no envelope, so the advisor may propose it
// for any of them.
//
// The rule is glob-driven over ManifestNames() so a future *-tmux CLI added
// without full tier coverage fails here — no hardcoded CLI list.

import "testing"

// emittedModelValue returns the model value this realization delivers through
// spec's flag, or "" when none was emitted. Driven by spec.Flag rather than a
// hardcoded "--model" so a manifest using a different flag name (codex uses
// "-m") cannot pass the assertion vacuously.
//
// Flag-only by design: no embedded manifest declares channel:"repl" for
// model_tier, and the repl path's omit-on-vocabulary behavior is covered by
// realizer_modelpolicy_test.go. Adding an unreachable branch here would be
// untested code in a test helper.
func emittedModelValue(r Realization, spec ParamSpec) string {
	for i, f := range r.LaunchFlags {
		if f == spec.Flag && i+1 < len(r.LaunchFlags) {
			return r.LaunchFlags[i+1]
		}
	}
	return ""
}

// TestRealizeScalar_OmitsModelWhenResolutionLeavesATierName pins that no
// embedded tmux manifest ever hands a CLI an abstract vocabulary token as its
// model. RED before the sentinel widening: claude-tmux/top emits `--model top`.
func TestRealizeScalar_OmitsModelWhenResolutionLeavesATierName(t *testing.T) {
	injectCatalogDir(t, t.TempDir())

	for _, name := range ManifestNames() {
		m, err := LoadManifest(name)
		if err != nil {
			t.Fatalf("LoadManifest(%s): %v", name, err)
		}
		if !m.IsTmux() {
			continue
		}
		spec := m.Params["model_tier"]
		if spec.Channel != "flag" {
			continue // positional/noop/repl drivers emit no launch flag here
		}
		for _, tok := range unresolvedModelTokens {
			t.Run(name+"/"+tok, func(t *testing.T) {
				got := emittedModelValue(Realize(m, LaunchIntent{ModelTier: tok}), spec)
				if got == tok {
					t.Errorf("Realize(%s, ModelTier=%q) emitted %q as the model value — %q is an abstract vocabulary token, not a model. "+
						"Reaching the emit point means model_tier_map translation fell through; launching against it is the cycle-262 "+
						"`--model auto` fatal-boot class. Omit the param instead: the CLI's own default model always beats a fatal boot.",
						name, tok, got, tok)
				}
			})
		}
	}
}
