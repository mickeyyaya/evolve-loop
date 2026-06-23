package policy_test

// QuotaResetConfig — the typed parameter that replaced EVOLVE_QUOTA_RESET_AT /
// EVOLVE_QUOTA_RESET_HOURS. Driven only through Policy.QuotaResetConfig() and
// policy.Load (explicit path), never env.

import (
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/policy"
)

func TestQuotaResetConfig_Resolution(t *testing.T) {
	cases := []struct {
		name string
		pol  policy.Policy
		want policy.QuotaResetConfig
	}{
		{"absent-is-zero", policy.Policy{}, policy.QuotaResetConfig{}},
		{"present-verbatim", policy.Policy{QuotaReset: &policy.QuotaResetConfig{ResetAt: "2026-06-21T09:00:00Z", DefaultHours: 5.5}}, policy.QuotaResetConfig{ResetAt: "2026-06-21T09:00:00Z", DefaultHours: 5.5}},
		{"present-zero-is-zero", policy.Policy{QuotaReset: &policy.QuotaResetConfig{}}, policy.QuotaResetConfig{}},
		{"negative-hours-passthrough", policy.Policy{QuotaReset: &policy.QuotaResetConfig{DefaultHours: -3}}, policy.QuotaResetConfig{DefaultHours: -3}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got policy.QuotaResetConfig = tc.pol.QuotaResetConfig()
			if got != tc.want {
				t.Errorf("QuotaResetConfig() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestLoad_QuotaResetBlock(t *testing.T) {
	cases := []struct {
		name string
		json string
		want policy.QuotaResetConfig
	}{
		{"full-block", `{"quota_reset":{"reset_at":"2026-06-21T09:00:00Z","default_hours":3.5}}`, policy.QuotaResetConfig{ResetAt: "2026-06-21T09:00:00Z", DefaultHours: 3.5}},
		{"absent-block-is-zero", `{}`, policy.QuotaResetConfig{}},
		{"empty-block-is-zero", `{"quota_reset":{}}`, policy.QuotaResetConfig{}},
		{"reset-at-only", `{"quota_reset":{"reset_at":"X"}}`, policy.QuotaResetConfig{ResetAt: "X"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pol, err := policy.Load(writeTempPolicy(t, tc.json))
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if got := pol.QuotaResetConfig(); got != tc.want {
				t.Errorf("after Load, QuotaResetConfig() = %+v, want %+v", got, tc.want)
			}
		})
	}
}
