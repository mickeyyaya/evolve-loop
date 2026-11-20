package policy

import (
	"errors"
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

func TestPreflightConfig_DiskPauseMax(t *testing.T) {
	ten := 10
	cases := []struct {
		name string
		p    Policy
		want time.Duration
	}{
		{"absent block takes the default", Policy{}, 30 * time.Minute},
		{"empty block takes the default", Policy{Preflight: &PreflightPolicy{}}, 30 * time.Minute},
		{"empty disk guard takes the default", Policy{Preflight: &PreflightPolicy{DiskGuard: &DiskGuardPolicy{}}}, 30 * time.Minute},
		{"explicit bound honored", Policy{Preflight: &PreflightPolicy{DiskGuard: &DiskGuardPolicy{MaxPauseSeconds: &ten}}}, 10 * time.Second},
	}
	for _, c := range cases {
		if got := c.p.PreflightConfig().DiskPauseMax; got != c.want {
			t.Errorf("%s: DiskPauseMax = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestParse_PreflightDiskGuardMaxPause(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		wantErr bool
		want    time.Duration
	}{
		{"valid bound", `{"preflight":{"disk_guard":{"max_pause_s":45}}}`, false, 45 * time.Second},
		{"absent bound", `{"preflight":{"disk_guard":{}}}`, false, 30 * time.Minute},
		{"zero is refused", `{"preflight":{"disk_guard":{"max_pause_s":0}}}`, true, 0},
		{"negative is refused", `{"preflight":{"disk_guard":{"max_pause_s":-5}}}`, true, 0},
		{"unknown disk guard key is refused", `{"preflight":{"disk_guard":{"max_pause":5}}}`, true, 0},
	}
	for _, c := range cases {
		p, err := Parse([]byte(c.raw))
		if c.wantErr {
			if err == nil {
				t.Errorf("%s: Parse accepted %s", c.name, c.raw)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: Parse = %v", c.name, err)
			continue
		}
		if got := p.PreflightConfig().DiskPauseMax; got != c.want {
			t.Errorf("%s: DiskPauseMax = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestParse_UnknownDiskGuardKeyIsErrUnknownKey(t *testing.T) {
	_, err := Parse([]byte(`{"preflight":{"disk_guard":{"max_pause":5}}}`))
	if !errors.Is(err, ErrUnknownKey) {
		t.Fatalf("Parse = %v, want ErrUnknownKey", err)
	}
}
