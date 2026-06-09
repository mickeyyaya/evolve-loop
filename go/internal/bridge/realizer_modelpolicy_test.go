package bridge

// realizer_modelpolicy_test.go — ADR-0044 C2/D3 (Slice 2) RED tests: the
// ModelFlagPolicy "omit-on-auto" guard at the realizer chokepoint.
//
// cycle-262 (2026-06-09): retro was dispatched with model "auto" — the loop's
// resolve-me sentinel, never a valid concrete model for ANY CLI. The
// claude-tmux manifest realizes model_tier as `--model <value>` with
// ModelTierMap pass-through for unmapped values, so the sentinel sailed
// straight into `claude --model auto`, which boots into the fatal
// "There's an issue with the selected model (auto)" pane (verified in the
// cycle-262 tmux-final-scrollback). The headless codex driver already guards
// this (omit -m on auto, pinned in coverage_batch7_test.go); the realizer is
// the single emit point for every flag/repl-channel CLI, so ONE guard here
// covers claude-tmux, codex-tmux, and any future manifest (matrix-wide fix,
// not a per-driver patch).
//
// Contract: when post-resolution model_tier is still the "auto" sentinel, the
// realizer emits NO model param at all — the CLI's own default model is
// always preferable to a fatal boot. Concrete tiers and raw model names are
// unaffected.

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

// TestRealize_ModelAutoOmitsFlag is the D3 reproduction: the "auto" sentinel
// must never be emitted as a concrete model value.
func TestRealize_ModelAutoOmitsFlag(t *testing.T) {
	t.Parallel()
	flags := launchFlagsForModel(t, modelPolicyManifest(), "auto")
	if containsFlag(flags, "--model") || containsFlag(flags, "auto") {
		t.Fatalf("model_tier=auto must omit the model param entirely (cycle-262: `claude --model auto` is a fatal boot); got flags=%v", flags)
	}
}

// TestRealize_ConcreteTiersStillEmit pins the non-regression half of the
// policy: mapped tiers and raw model names keep flowing exactly as before.
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

// TestRealize_ReplChannelAutoOmitted covers the repl-template channel with a
// synthetic manifest: the same sentinel guard must apply before ANY channel
// emits (the realizer is the single chokepoint for both).
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
