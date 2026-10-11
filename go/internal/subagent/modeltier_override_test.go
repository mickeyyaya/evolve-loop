package subagent

import "testing"

func TestApplyModelTierOverride(t *testing.T) {
	const lowGoal = `"model_tier_overrides":{"cycle_1_or_low_goal":"opus"}`
	cases := []struct {
		name    string
		base    string
		profile string
		cycle   int
		want    string
	}{
		{"unparseable profile keeps the base", "sonnet", `{"model_tier_overrides":`, 1, "sonnet"},
		{"no overrides keeps the base", "sonnet", `{}`, 1, "sonnet"},
		{"cycle 1 escalates", "sonnet", `{` + lowGoal + `}`, 1, "opus"},
		{"cycle 0 counts as early", "sonnet", `{` + lowGoal + `}`, 0, "opus"},
		{"cycle 2 has no active situation", "sonnet", `{` + lowGoal + `}`, 2, "sonnet"},
		{"blank override keeps the base", "sonnet", `{"model_tier_overrides":{"cycle_1_or_low_goal":"  "}}`, 1, "sonnet"},
		{"other situation does not apply", "sonnet", `{"model_tier_overrides":{"later":"opus"}}`, 1, "sonnet"},
		{"override never downgrades", "opus", `{"model_tier_overrides":{"cycle_1_or_low_goal":"haiku"}}`, 1, "opus"},
		{"equal tier keeps the base spelling", "deep", `{` + lowGoal + `}`, 1, "deep"},
		{"envelope max clamps the override", "haiku", `{"model_tier_envelope":{"max":"sonnet"},` + lowGoal + `}`, 1, "sonnet"},
		{"envelope max above the override is no clamp", "haiku", `{"model_tier_envelope":{"max":"top"},` + lowGoal + `}`, 1, "opus"},
		{"envelope max equal to the override is no clamp", "haiku", `{"model_tier_envelope":{"max":"opus"},` + lowGoal + `}`, 1, "opus"},
		{"same-rank envelope max keeps the override spelling", "haiku", `{"model_tier_envelope":{"max":"opus"},"model_tier_overrides":{"cycle_1_or_low_goal":"deep"}}`, 1, "deep"},
		{"empty envelope max is ignored", "haiku", `{"model_tier_envelope":{"min":"haiku"},` + lowGoal + `}`, 1, "opus"},
		{"unranked envelope max is ignored", "haiku", `{"model_tier_envelope":{"max":"mystery"},` + lowGoal + `}`, 1, "opus"},
		{"clamped override at or below the base keeps the base", "sonnet", `{"model_tier_envelope":{"max":"sonnet"},` + lowGoal + `}`, 1, "sonnet"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := applyModelTierOverride(tc.base, tc.profile, ResolveModelTierRequest{Cycle: tc.cycle})

			if got != tc.want {
				t.Errorf("applyModelTierOverride(%q, cycle %d) = %q, want %q", tc.base, tc.cycle, got, tc.want)
			}
		})
	}
}

func TestResolveModelTier_FirstCycleOverrideReachesTheResolvedTier(t *testing.T) {
	profile := `{"role":"builder","model_tier_default":"sonnet","model_tier_overrides":{"cycle_1_or_low_goal":"opus"}}`
	read := func(string) (string, error) { return profile, nil }

	first, err := ResolveModelTier(ResolveModelTierRequest{ProfilePath: "p", Cycle: 1}, ResolveModelTierOptions{ReadProfile: read})
	if err != nil {
		t.Fatal(err)
	}
	later, err := ResolveModelTier(ResolveModelTierRequest{ProfilePath: "p", Cycle: 5}, ResolveModelTierOptions{ReadProfile: read})
	if err != nil {
		t.Fatal(err)
	}

	if first != TierOpus || later != TierSonnet {
		t.Errorf("tiers = (cycle1 %q, cycle5 %q), want (opus, sonnet)", first, later)
	}
}
