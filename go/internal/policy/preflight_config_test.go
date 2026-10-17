package policy

import (
	"testing"
	"time"
)

func TestPreflightConfig_MinFreeGiB(t *testing.T) {
	def := Policy{}.PreflightConfig().MinFreeGiB
	if def <= 0 {
		t.Fatalf("default floor = %v, want a positive documented default", def)
	}
	cases := []struct {
		name string
		p    Policy
		want float64
	}{
		{"absent block takes the default", Policy{}, def},
		{"empty block takes the default", Policy{Preflight: &PreflightPolicy{}}, def},
		{"explicit floor honored", Policy{Preflight: &PreflightPolicy{MinFreeGiB: 5}}, 5},
		{"fractional floor honored", Policy{Preflight: &PreflightPolicy{MinFreeGiB: 0.25}}, 0.25},
		{"negative floor falls back to the default", Policy{Preflight: &PreflightPolicy{MinFreeGiB: -3}}, def},
	}
	for _, c := range cases {
		if got := c.p.PreflightConfig().MinFreeGiB; got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

func TestPreflightConfig_ReleaseLagDials(t *testing.T) {
	def := Policy{}.PreflightConfig()
	if def.ReleaseLagThreshold <= 0 || def.ReleaseCheckTTL <= 0 {
		t.Fatalf("defaults = %+v, want positive documented defaults", def)
	}
	cases := []struct {
		name          string
		p             Policy
		wantThreshold int
		wantTTL       time.Duration
	}{
		{"absent block takes the defaults", Policy{}, def.ReleaseLagThreshold, def.ReleaseCheckTTL},
		{"explicit dials honored", Policy{Preflight: &PreflightPolicy{ReleaseLagThreshold: 5, ReleaseCheckTTLHours: 6}}, 5, 6 * time.Hour},
		{"fractional hours honored", Policy{Preflight: &PreflightPolicy{ReleaseCheckTTLHours: 0.5}}, def.ReleaseLagThreshold, 30 * time.Minute},
		{"zero and negative fall back", Policy{Preflight: &PreflightPolicy{ReleaseLagThreshold: -1, ReleaseCheckTTLHours: -2}}, def.ReleaseLagThreshold, def.ReleaseCheckTTL},
	}
	for _, c := range cases {
		got := c.p.PreflightConfig()
		if got.ReleaseLagThreshold != c.wantThreshold || got.ReleaseCheckTTL != c.wantTTL {
			t.Errorf("%s: got %d/%v want %d/%v", c.name, got.ReleaseLagThreshold, got.ReleaseCheckTTL, c.wantThreshold, c.wantTTL)
		}
	}
}
