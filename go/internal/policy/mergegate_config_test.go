package policy_test

// MergeGatePolicy / MergeGateConfig — the typed parameter block for the
// merge-to-main gate (config-as-code, no flags). MergeGateConfig() encodes the
// non-obvious default rules, mirroring RouterConfig/SwarmConfig: Stage→"shadow"
// (byte-neutral first deploy over the riskiest action), BatchWaveCount→1,
// BatchChurnLOC→800, BlockSeverity→"HIGH", CarryoverStallCycles→8 — each numeric
// override applies only when > 0, each string override only when non-empty, so a
// partial or absent block can never silently produce an unsafe zero threshold.
// Black-box: drives only the exported accessor + explicit inputs, zero env.

import (
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/policy"
)

func TestMergeGateConfig_Resolution(t *testing.T) {
	defaults := policy.MergeGateConfig{
		Stage:                "shadow",
		BatchWaveCount:       1,
		BatchChurnLOC:        800,
		BlockSeverity:        "HIGH",
		CarryoverStallCycles: 8,
	}
	cases := []struct {
		name string
		pol  policy.Policy
		want policy.MergeGateConfig
	}{
		{"absent-defaults", policy.Policy{}, defaults},
		{"empty-block-defaults", policy.Policy{MergeGate: &policy.MergeGatePolicy{}}, defaults},
		{"stage-override", policy.Policy{MergeGate: &policy.MergeGatePolicy{Stage: "enforce"}},
			policy.MergeGateConfig{Stage: "enforce", BatchWaveCount: 1, BatchChurnLOC: 800, BlockSeverity: "HIGH", CarryoverStallCycles: 8}},
		{"batch-wave-zero-falls-to-default", policy.Policy{MergeGate: &policy.MergeGatePolicy{BatchWaveCount: 0}}, defaults},
		{"batch-wave-negative-falls-to-default", policy.Policy{MergeGate: &policy.MergeGatePolicy{BatchWaveCount: -3}}, defaults},
		{"batch-wave-override", policy.Policy{MergeGate: &policy.MergeGatePolicy{BatchWaveCount: 3}},
			policy.MergeGateConfig{Stage: "shadow", BatchWaveCount: 3, BatchChurnLOC: 800, BlockSeverity: "HIGH", CarryoverStallCycles: 8}},
		{"batch-churn-zero-falls-to-default", policy.Policy{MergeGate: &policy.MergeGatePolicy{BatchChurnLOC: 0}}, defaults},
		{"batch-churn-override", policy.Policy{MergeGate: &policy.MergeGatePolicy{BatchChurnLOC: 1500}},
			policy.MergeGateConfig{Stage: "shadow", BatchWaveCount: 1, BatchChurnLOC: 1500, BlockSeverity: "HIGH", CarryoverStallCycles: 8}},
		{"block-severity-override", policy.Policy{MergeGate: &policy.MergeGatePolicy{BlockSeverity: "CRITICAL"}},
			policy.MergeGateConfig{Stage: "shadow", BatchWaveCount: 1, BatchChurnLOC: 800, BlockSeverity: "CRITICAL", CarryoverStallCycles: 8}},
		{"carryover-stall-zero-falls-to-default", policy.Policy{MergeGate: &policy.MergeGatePolicy{CarryoverStallCycles: 0}}, defaults},
		{"carryover-stall-override", policy.Policy{MergeGate: &policy.MergeGatePolicy{CarryoverStallCycles: 12}},
			policy.MergeGateConfig{Stage: "shadow", BatchWaveCount: 1, BatchChurnLOC: 800, BlockSeverity: "HIGH", CarryoverStallCycles: 12}},
		{"full-override", policy.Policy{MergeGate: &policy.MergeGatePolicy{
			Stage: "advisory", BatchWaveCount: 5, BatchChurnLOC: 1200, BlockSeverity: "MEDIUM", CarryoverStallCycles: 20,
		}}, policy.MergeGateConfig{
			Stage: "advisory", BatchWaveCount: 5, BatchChurnLOC: 1200, BlockSeverity: "MEDIUM", CarryoverStallCycles: 20,
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.pol.MergeGateConfig(); got != tc.want {
				t.Errorf("MergeGateConfig() = %+v, want %+v", got, tc.want)
			}
		})
	}
}
