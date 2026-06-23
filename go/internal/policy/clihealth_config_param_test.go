package policy_test

// CLIHealthConfig — the typed parameter gating the proactive per-cycle usage
// probe. Driven only through Policy.CLIHealthConfig() and policy.Load, never
// env. Default (absent block) is ProactiveProbe=false: the probe is opt-in, so
// nothing changes for an operator who has not enabled it.

import (
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/policy"
)

func TestCLIHealthConfig_Resolution(t *testing.T) {
	cases := []struct {
		name string
		pol  policy.Policy
		want policy.CLIHealthConfig
	}{
		{"absent-is-zero-off", policy.Policy{}, policy.CLIHealthConfig{}},
		{"present-enabled", policy.Policy{CLIHealth: &policy.CLIHealthConfig{ProactiveProbe: true}}, policy.CLIHealthConfig{ProactiveProbe: true}},
		{"present-zero-is-off", policy.Policy{CLIHealth: &policy.CLIHealthConfig{}}, policy.CLIHealthConfig{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.pol.CLIHealthConfig(); got != tc.want {
				t.Errorf("CLIHealthConfig() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestLoad_CLIHealthBlock(t *testing.T) {
	cases := []struct {
		name string
		json string
		want policy.CLIHealthConfig
	}{
		{"enabled", `{"cli_health":{"proactive_probe":true}}`, policy.CLIHealthConfig{ProactiveProbe: true}},
		{"absent-is-off", `{}`, policy.CLIHealthConfig{}},
		{"empty-block-is-off", `{"cli_health":{}}`, policy.CLIHealthConfig{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pol, err := policy.Load(writeTempPolicy(t, tc.json))
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if got := pol.CLIHealthConfig(); got != tc.want {
				t.Errorf("after Load, CLIHealthConfig() = %+v, want %+v", got, tc.want)
			}
		})
	}
}
