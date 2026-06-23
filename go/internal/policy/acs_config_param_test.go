package policy_test

// ACSConfig — the ACS timeout config that replaced EVOLVE_ACS_GO_TIMEOUT_S.
// GoTimeoutS defaults 0 (absent block → use DefaultTimeout in acssuite).

import (
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/policy"
)

func TestACSTimeoutConfig_Resolution(t *testing.T) {
	cases := []struct {
		name string
		pol  policy.Policy
		want policy.ACSConfig
	}{
		{
			"absent-defaults-zero",
			policy.Policy{},
			policy.ACSConfig{GoTimeoutS: 0},
		},
		{
			"empty-block-defaults-zero",
			policy.Policy{ACS: &policy.ACSConfig{}},
			policy.ACSConfig{GoTimeoutS: 0},
		},
		{
			"go-timeout-s-set",
			policy.Policy{ACS: &policy.ACSConfig{GoTimeoutS: 120}},
			policy.ACSConfig{GoTimeoutS: 120},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.pol.ACSTimeoutConfig()
			if got != tc.want {
				t.Errorf("ACSTimeoutConfig() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestLoad_ACSBlock(t *testing.T) {
	cases := []struct {
		name string
		json string
		want policy.ACSConfig
	}{
		{
			"absent-block-zero",
			`{}`,
			policy.ACSConfig{GoTimeoutS: 0},
		},
		{
			"go-timeout-s-120",
			`{"acs":{"go_timeout_s":120}}`,
			policy.ACSConfig{GoTimeoutS: 120},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pol, err := policy.Load(writeTempPolicy(t, tc.json))
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if got := pol.ACSTimeoutConfig(); got != tc.want {
				t.Errorf("after Load, ACSTimeoutConfig() = %+v, want %+v", got, tc.want)
			}
		})
	}
}
