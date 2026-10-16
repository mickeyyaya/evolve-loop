package bridge

import "testing"

func emittedModelValue(r Realization, spec ParamSpec) string {
	for i, f := range r.LaunchFlags {
		if f == spec.Flag && i+1 < len(r.LaunchFlags) {
			return r.LaunchFlags[i+1]
		}
	}
	return ""
}

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
			continue
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
