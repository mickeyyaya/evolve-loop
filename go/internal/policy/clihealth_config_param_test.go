package policy_test

import (
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
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

func TestCLIHealthConfig_UsageEvidenceTimeoutAndTTLComeFromTheBlockWithCompiledDefaults(t *testing.T) {
	for name, tc := range map[string]struct {
		json         string
		timeout, ttl time.Duration
	}{
		"absent block":    {`{}`, 2 * time.Minute, 10 * time.Minute},
		"block without":   {`{"cli_health":{"proactive_probe":true}}`, 2 * time.Minute, 10 * time.Minute},
		"block with both": {`{"cli_health":{"usage_evidence_timeout_s":45,"usage_evidence_ttl_s":90}}`, 45 * time.Second, 90 * time.Second},
		"non-positive":    {`{"cli_health":{"usage_evidence_timeout_s":-1,"usage_evidence_ttl_s":0}}`, 2 * time.Minute, 10 * time.Minute},
	} {
		pol, err := policy.Load(writeTempPolicy(t, tc.json))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		c := pol.CLIHealthConfig()
		if c.UsageEvidenceTimeout() != tc.timeout || c.UsageEvidenceTTL() != tc.ttl {
			t.Errorf("%s: timeout %v ttl %v, want %v and %v", name, c.UsageEvidenceTimeout(), c.UsageEvidenceTTL(), tc.timeout, tc.ttl)
		}
	}
}
