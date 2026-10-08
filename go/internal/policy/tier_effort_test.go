package policy_test

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestBridgeConfig_TierEffortsDefaultsAndOverrides(t *testing.T) {
	compiled := map[string]string{"fast": "low", "balanced": "medium", "deep": "high", "top": "xhigh"}
	cases := []struct {
		name     string
		override map[string]string
		want     map[string]string
	}{
		{"absent-gives-compiled", nil, compiled},
		{"deep-override-wins", map[string]string{"deep": "medium"}, map[string]string{"fast": "low", "balanced": "medium", "deep": "medium", "top": "xhigh"}},
		{"max-is-accepted", map[string]string{"top": "max"}, map[string]string{"fast": "low", "balanced": "medium", "deep": "high", "top": "max"}},
		{"unknown-effort-keeps-compiled", map[string]string{"deep": "hihg"}, compiled},
		{"empty-effort-keeps-compiled", map[string]string{"deep": ""}, compiled},
		{"unknown-tier-is-dropped", map[string]string{"opus": "low"}, compiled},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := policy.BridgePolicy{TierEffort: tc.override}.TierEfforts()
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("TierEfforts() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestBridgeConfig_TierEffortsReturnsAFreshMap(t *testing.T) {
	first := policy.BridgePolicy{}.TierEfforts()
	first["deep"] = "low"
	if got := (policy.BridgePolicy{}).TierEfforts()["deep"]; got != "high" {
		t.Fatalf("a caller edit leaked into the compiled table: deep = %q, want %q", got, "high")
	}
}

func TestLoad_BridgeTierEffort(t *testing.T) {
	pol, err := policy.Load(writeTempPolicy(t, `{"bridge":{"tier_effort":{"deep":"medium"}}}`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := pol.BridgeConfig().TierEfforts()["deep"]; got != "medium" {
		t.Errorf("after Load, deep effort = %q, want %q", got, "medium")
	}
}

func TestBridgeConfig_TierEffortWarningsNameEachIgnoredEntry(t *testing.T) {
	got := policy.BridgePolicy{TierEffort: map[string]string{"deep": "hihg", "opus": "low", "top": "max", "balanced": ""}}.TierEffortWarnings()
	want := []string{
		`bridge.tier_effort.balanced: unknown effort "", keeping "medium"`,
		`bridge.tier_effort.deep: unknown effort "hihg", keeping "high"`,
		`bridge.tier_effort.opus: unknown tier, ignored`,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("TierEffortWarnings() =\n%q\nwant\n%q", got, want)
	}
	if w := (policy.BridgePolicy{}).TierEffortWarnings(); len(w) != 0 {
		t.Errorf("an absent block warns %q, want nothing", w)
	}
}

func TestEffortLevels_AreTheOrderedVocabularyAndACopy(t *testing.T) {
	want := []string{"low", "medium", "high", "xhigh", "max"}
	got := policy.EffortLevels()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("EffortLevels() = %v, want %v", got, want)
	}
	got[0] = "edited"
	if policy.EffortLevels()[0] != "low" {
		t.Fatal("a caller edit leaked into the effort vocabulary")
	}
}
