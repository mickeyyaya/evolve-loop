package policy_test

import (
	"encoding/json"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// TestResearchConfig_AbsentDefaults pins the behaviour-preservation contract:
// an install with no "research" block must resolve to the values the compiled
// constants carry today, so introducing the knobs cannot narrow advisor recall
// or arm the novelty gate differently anywhere.
func TestResearchConfig_AbsentDefaults(t *testing.T) {
	got := policy.Policy{}.ResearchConfig()
	want := policy.ResearchConfig{RecallK: 5, NoveltyThreshold: 0.9}
	if got != want {
		t.Errorf("zero-value Policy{}.ResearchConfig() = %+v, want %+v", got, want)
	}
}

// TestResearchConfig_EmptyBlockDefaults covers the operator who writes
// "research": {} — an empty block is an absent block, not zero-valued config.
func TestResearchConfig_EmptyBlockDefaults(t *testing.T) {
	got := policy.Policy{Research: &policy.ResearchPolicy{}}.ResearchConfig()
	want := policy.ResearchConfig{RecallK: 5, NoveltyThreshold: 0.9}
	if got != want {
		t.Errorf("empty block ResearchConfig() = %+v, want %+v", got, want)
	}
}

// TestResearchConfig_RecallKRangeResolution is the load-bearing negative half:
// out-of-range recall must fall back to the visible built-in. 0/-1 would
// otherwise silently DISABLE the advisor's recall memory, and an absurd value
// would flood the plan prompt with weak matches.
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

// TestResearchConfig_NoveltyThresholdRangeResolution pins the second knob's
// range. 0 would suppress every lesson write (evidence loss) and >1 would
// disarm the gate; both must resolve to the built-in instead.
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

// TestResearchConfig_JSONTagsBindOperatorBlock proves the operator-facing wire
// shape actually reaches the resolver: a policy.json "research" block with
// snake_case keys must populate the typed fields (a renamed tag would leave the
// operator's tuning silently at the default).
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
