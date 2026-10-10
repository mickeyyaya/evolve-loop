package clihealth

import "testing"

func TestQuotaPattern_OnlyAQuotaWallCarriesAResetTime(t *testing.T) {
	for _, tc := range []struct {
		pattern string
		want    bool
	}{
		{"rate_limit", true},
		{ExhaustedPattern, true},
		{QuotaExhaustedPattern, true},
		{UsageProbePattern, true},
		{CredentialPattern, false},
		{BootTimeoutPattern, false},
		{"", false},
	} {
		if got := QuotaPattern(tc.pattern); got != tc.want {
			t.Errorf("QuotaPattern(%q)=%v, want %v: a login or boot bench lapses on a cooldown, not on a quota reset", tc.pattern, got, tc.want)
		}
	}
}
