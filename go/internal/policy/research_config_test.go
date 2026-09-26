package policy_test

import (
	"encoding/json"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestResearchConfig_AbsentDefaults(t *testing.T) {
	got := policy.Policy{}.ResearchConfig()
	want := policy.ResearchConfig{RecallK: 5, NoveltyThreshold: 0.9}
	if got != want {
		t.Errorf("zero-value Policy{}.ResearchConfig() = %+v, want %+v", got, want)
	}
}

func TestResearchConfig_EmptyBlockDefaults(t *testing.T) {
	got := policy.Policy{Research: &policy.ResearchPolicy{}}.ResearchConfig()
	want := policy.ResearchConfig{RecallK: 5, NoveltyThreshold: 0.9}
	if got != want {
		t.Errorf("empty block ResearchConfig() = %+v, want %+v", got, want)
	}
}

func TestResearchConfig_RecallKRangeResolution(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   int
		want int
	}{
		{"zero", 0, 5},
		{"negative", -1, 5},
		{"absurdly-large", 100000, 5},
		{"just-past-cap", 51, 5},
		{"cap", 50, 50},
		{"in-range-3", 3, 3},
		{"in-range-8", 8, 8},
	} {
		got := policy.Policy{Research: &policy.ResearchPolicy{RecallK: tc.in}}.ResearchConfig().RecallK
		if got != tc.want {
			t.Errorf("%s: RecallK(%d) = %d, want %d", tc.name, tc.in, got, tc.want)
		}
	}
}

func TestResearchConfig_NoveltyThresholdRangeResolution(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   float64
		want float64
	}{
		{"zero", 0, 0.9},
		{"negative", -0.5, 0.9},
		{"above-one", 1.5, 0.9},
		{"in-range", 0.75, 0.75},
		{"exactly-one", 1, 1},
	} {
		got := policy.Policy{Research: &policy.ResearchPolicy{NoveltyThreshold: tc.in}}.ResearchConfig().NoveltyThreshold
		if got != tc.want {
			t.Errorf("%s: NoveltyThreshold(%v) = %v, want %v", tc.name, tc.in, got, tc.want)
		}
	}
}

func TestResearchConfig_JSONTagsBindOperatorBlock(t *testing.T) {
	var p policy.Policy
	if err := json.Unmarshal([]byte(`{"research":{"recall_k":7,"novelty_threshold":0.8}}`), &p); err != nil {
		t.Fatalf("unmarshal policy: %v", err)
	}
	if p.Research == nil {
		t.Fatal("research block did not bind — check the Policy.Research json tag")
	}
	got := p.ResearchConfig()
	want := policy.ResearchConfig{RecallK: 7, NoveltyThreshold: 0.8}
	if got != want {
		t.Errorf("ResearchConfig() from JSON = %+v, want %+v", got, want)
	}
}
