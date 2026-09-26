package policy_test

import (
	"encoding/json"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const contextFillDefaultPct = 60

func TestContextFillConfig_AbsentDefaults(t *testing.T) {
	got := policy.Policy{}.ContextFillConfig()
	if got.WarnThresholdPct != contextFillDefaultPct {
		t.Errorf("absent block: WarnThresholdPct = %d, want %d", got.WarnThresholdPct, contextFillDefaultPct)
	}
}

func TestContextFillConfig_EmptyBlockDefaults(t *testing.T) {
	got := policy.Policy{ContextFill: &policy.ContextFillPolicy{}}.ContextFillConfig()
	if got.WarnThresholdPct != contextFillDefaultPct {
		t.Errorf("empty block: WarnThresholdPct = %d, want %d", got.WarnThresholdPct, contextFillDefaultPct)
	}
}

func TestContextFillConfig_ValidOverrideRespected(t *testing.T) {
	for _, want := range []int{1, 42, 85, 100} {
		got := policy.Policy{ContextFill: &policy.ContextFillPolicy{WarnThresholdPct: want}}.ContextFillConfig()
		if got.WarnThresholdPct != want {
			t.Errorf("override %d: WarnThresholdPct = %d, want %d", want, got.WarnThresholdPct, want)
		}
	}
}

func TestContextFillConfig_OutOfRangeFallsToDefault(t *testing.T) {
	for _, bad := range []int{0, -1, -60, 101, 900} {
		got := policy.Policy{ContextFill: &policy.ContextFillPolicy{WarnThresholdPct: bad}}.ContextFillConfig()
		if got.WarnThresholdPct != contextFillDefaultPct {
			t.Errorf("out-of-range %d: WarnThresholdPct = %d, want %d (operator input is never accepted verbatim)", bad, got.WarnThresholdPct, contextFillDefaultPct)
		}
	}
}

func TestContextFillConfig_JSONKeyIsContextFill(t *testing.T) {
	var pol policy.Policy
	if err := json.Unmarshal([]byte(`{"context_fill":{"warn_threshold_pct":42}}`), &pol); err != nil {
		t.Fatalf("parse policy JSON: %v", err)
	}
	if got := pol.ContextFillConfig().WarnThresholdPct; got != 42 {
		t.Errorf(`{"context_fill":{"warn_threshold_pct":42}} resolved to %d, want 42 — JSON tags must be context_fill/warn_threshold_pct`, got)
	}
}
