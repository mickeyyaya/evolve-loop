package policy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func zeroShipThreshold(t *testing.T, th FailureThresholds) (int, bool) {
	t.Helper()
	raw, err := json.Marshal(th)
	if err != nil {
		t.Fatalf("marshal thresholds: %v", err)
	}
	var m map[string]int
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal thresholds: %v", err)
	}
	v, ok := m["zero_ship_halt"]
	return v, ok
}

func loadFailureThresholds(t *testing.T, policyJSON string) FailureThresholds {
	t.Helper()
	path := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(path, []byte(policyJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := Load(path)
	if err != nil {
		t.Fatalf("Load(%s): %v", policyJSON, err)
	}
	fp, err := p.FailurePolicyConfig()
	if err != nil {
		t.Fatalf("FailurePolicyConfig: %v", err)
	}
	return fp.Thresholds
}

func TestZeroShipHalt_CompiledDefaultIsTwo(t *testing.T) {
	got, ok := zeroShipThreshold(t, DefaultSystemFailurePolicy().Thresholds)
	if !ok || got != 2 {
		t.Fatalf("compiled default failure_policy.thresholds.zero_ship_halt = %d (present=%v), want 2", got, ok)
	}
}

func TestZeroShipHalt_AbsentPolicyBlockResolvesToTheDefault(t *testing.T) {
	got, ok := zeroShipThreshold(t, loadFailureThresholds(t, `{}`))
	if !ok || got != 2 {
		t.Fatalf("absent failure_policy block: zero_ship_halt = %d (present=%v), want the default 2", got, ok)
	}
}

func TestZeroShipHalt_ReadsFromPolicyJSON(t *testing.T) {
	cases := []struct {
		name   string
		policy string
		want   int
	}{
		{"an operator override", `{"failure_policy":{"thresholds":{"zero_ship_halt":4}}}`, 4},
		{"an override of one", `{"failure_policy":{"thresholds":{"zero_ship_halt":1}}}`, 1},
		{"zero falls back to the default", `{"failure_policy":{"thresholds":{"zero_ship_halt":0}}}`, 2},
		{"negative falls back to the default", `{"failure_policy":{"thresholds":{"zero_ship_halt":-3}}}`, 2},
		{"a sibling threshold leaves the default", `{"failure_policy":{"thresholds":{"lane_deferral_halt_ceiling":5}}}`, 2},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, ok := zeroShipThreshold(t, loadFailureThresholds(t, tc.policy))
			if !ok || got != tc.want {
				t.Fatalf("zero_ship_halt = %d (present=%v), want %d", got, ok, tc.want)
			}
		})
	}
}

func TestZeroShipHalt_OverrideLeavesSiblingThresholdsAtTheirDefaults(t *testing.T) {
	th := loadFailureThresholds(t, `{"failure_policy":{"thresholds":{"zero_ship_halt":5}}}`)
	if th.ConsecutiveFailuresHaltCeiling != 3 || th.LaneDeferralHaltCeiling != 3 {
		t.Errorf("siblings drifted: consecutive=%d lane_deferral=%d, want 3 and 3", th.ConsecutiveFailuresHaltCeiling, th.LaneDeferralHaltCeiling)
	}
}
