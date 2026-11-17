package policy_test

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestUsageBrake_TheHoldPercentComesFromTheCLIHealthBlockAndAbsentMeansOff(t *testing.T) {
	cases := []struct {
		name string
		json string
		want float64
	}{
		{"absent block is off", `{}`, 0},
		{"block without the key is off", `{"cli_health":{"proactive_probe":true}}`, 0},
		{"a whole percent", `{"cli_health":{"usage_hold_percent":90}}`, 90},
		{"a fractional percent", `{"cli_health":{"usage_hold_percent":87.5}}`, 87.5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pol, err := policy.Load(writeTempPolicy(t, tc.json))
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if got := pol.CLIHealthConfig().UsageHoldPercent; got != tc.want {
				t.Errorf("UsageHoldPercent = %v, want %v", got, tc.want)
			}
		})
	}
}
