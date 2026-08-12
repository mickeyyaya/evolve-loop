package policy_test

// context_fill_config_test.go — RED contract for cycle-1444 task
// `context-fill-warn-threshold`. Mirrors parallel_evaluate_config_test.go, the
// canonical sub-block resolution shape in this package.
//
// RED: policy.ContextFillPolicy, policy.ContextFillConfig and
// Policy.ContextFillConfig() do not exist yet; this file fails to COMPILE until
// Builder adds them (compile-fail = RED evidence).
//
// The invariant every case below defends: operator input is never accepted
// verbatim. Absent, empty, and out-of-range all resolve to the built-in 60%
// default — a typo must never silence the WARN (0) nor arm it on every launch.

import (
	"encoding/json"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// contextFillDefaultPct is the built-in default warn threshold (percent of the
// effective window) — the inbox item's stated default.
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

// TestContextFillConfig_OutOfRangeFallsToDefault is the load-bearing negative
// test: a percentage outside 1–100 is meaningless. Accepting 0 would disable the
// instrument silently (0 < every reading ⇒ warn always, or ≥ every reading ⇒
// warn never, depending on the comparison) and accepting 900 would disable it
// permanently. Both must fall back to the visible built-in default.
func TestContextFillConfig_OutOfRangeFallsToDefault(t *testing.T) {
	for _, bad := range []int{0, -1, -60, 101, 900} {
		got := policy.Policy{ContextFill: &policy.ContextFillPolicy{WarnThresholdPct: bad}}.ContextFillConfig()
		if got.WarnThresholdPct != contextFillDefaultPct {
			t.Errorf("out-of-range %d: WarnThresholdPct = %d, want %d (operator input is never accepted verbatim)", bad, got.WarnThresholdPct, contextFillDefaultPct)
		}
	}
}

// TestContextFillConfig_JSONKeyIsContextFill pins the on-disk policy.json key so
// an operator block written per the docs actually binds to the field.
func TestContextFillConfig_JSONKeyIsContextFill(t *testing.T) {
	var pol policy.Policy
	if err := json.Unmarshal([]byte(`{"context_fill":{"warn_threshold_pct":42}}`), &pol); err != nil {
		t.Fatalf("parse policy JSON: %v", err)
	}
	if got := pol.ContextFillConfig().WarnThresholdPct; got != 42 {
		t.Errorf(`{"context_fill":{"warn_threshold_pct":42}} resolved to %d, want 42 — JSON tags must be context_fill/warn_threshold_pct`, got)
	}
}
