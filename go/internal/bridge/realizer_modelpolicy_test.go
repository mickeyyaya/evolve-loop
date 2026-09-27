package bridge

import "testing"

func modelPolicyManifest() Manifest {
	return Manifest{
		CLI:    "claude-tmux",
		Binary: "claude",
		ModelTierMap: map[string]string{
			"fast":     "haiku",
			"balanced": "sonnet",
			"deep":     "opus",
		},
		Params: map[string]ParamSpec{
			"model_tier": {Channel: "flag", Flag: "--model", From: "model_tier_map"},
		},
	}
}

func launchFlagsForModel(t *testing.T, m Manifest, modelTier string) []string {
	t.Helper()
	r := Realize(m, LaunchIntent{ModelTier: modelTier})
	return r.LaunchFlags
}

func containsFlag(flags []string, want string) bool {
	for _, f := range flags {
		if f == want {
			return true
		}
	}
	return false
}

func TestRealize_ModelAutoOmitsFlag(t *testing.T) {
	t.Parallel()
	flags := launchFlagsForModel(t, modelPolicyManifest(), "auto")
	if containsFlag(flags, "--model") || containsFlag(flags, "auto") {
		t.Fatalf("model_tier=auto must omit the model param entirely (cycle-262: `claude --model auto` is a fatal boot); got flags=%v", flags)
	}
}

func TestRealize_ConcreteTiersStillEmit(t *testing.T) {
	t.Parallel()
	cases := []struct {
		tier string
		want string // concrete value expected after --model
	}{
		{"balanced", "sonnet"}, // canonical map key
		{"deep", "opus"},
		{"sonnet", "sonnet"},                   // legacy alias resolves via translateV1TierKey → balanced → sonnet
		{"claude-opus-4-8", "claude-opus-4-8"}, // raw model identifier passes through
	}
	for _, tc := range cases {
		flags := launchFlagsForModel(t, modelPolicyManifest(), tc.tier)
		if !containsFlag(flags, "--model") || !containsFlag(flags, tc.want) {
			t.Errorf("model_tier=%q: want --model %s in flags; got %v", tc.tier, tc.want, flags)
		}
	}
}

func TestRealize_ReplChannelAutoOmitted(t *testing.T) {
	t.Parallel()
	m := Manifest{
		CLI: "synthetic-tmux",
		Params: map[string]ParamSpec{
			"model_tier": {Channel: "repl", Template: "/model {alias}"},
		},
	}
	r := Realize(m, LaunchIntent{ModelTier: "auto"})
	if len(r.REPLInput) != 0 {
		t.Fatalf("repl-channel model_tier=auto must emit no /model command; got %v", r.REPLInput)
	}
}
