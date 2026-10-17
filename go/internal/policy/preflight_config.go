package policy

import "time"

const (
	defaultPreflightMinFreeGiB         = 1.0
	defaultPreflightReleaseLagPatches  = 2
	defaultPreflightReleaseCheckTTLHrs = 24.0
)

type PreflightPolicy struct {
	MinFreeGiB           float64 `json:"min_free_gib"`
	ReleaseLagThreshold  int     `json:"release_lag_threshold"`
	ReleaseCheckTTLHours float64 `json:"release_check_ttl_hours"`
}

type PreflightConfig struct {
	MinFreeGiB          float64
	ReleaseLagThreshold int
	ReleaseCheckTTL     time.Duration
}

func (p Policy) PreflightConfig() PreflightConfig {
	c := PreflightConfig{
		MinFreeGiB:          defaultPreflightMinFreeGiB,
		ReleaseLagThreshold: defaultPreflightReleaseLagPatches,
		ReleaseCheckTTL:     hoursToDuration(defaultPreflightReleaseCheckTTLHrs),
	}
	if p.Preflight == nil {
		return c
	}
	if p.Preflight.MinFreeGiB > 0 {
		c.MinFreeGiB = p.Preflight.MinFreeGiB
	}
	if p.Preflight.ReleaseLagThreshold > 0 {
		c.ReleaseLagThreshold = p.Preflight.ReleaseLagThreshold
	}
	if p.Preflight.ReleaseCheckTTLHours > 0 {
		c.ReleaseCheckTTL = hoursToDuration(p.Preflight.ReleaseCheckTTLHours)
	}
	return c
}

func hoursToDuration(h float64) time.Duration {
	return time.Duration(h * float64(time.Hour))
}

func (c PreflightConfig) MinFreeBytes() uint64 {
	return uint64(c.MinFreeGiB * (1 << 30))
}
