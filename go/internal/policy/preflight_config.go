package policy

import (
	"errors"
	"io"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/diskfloor"
)

const (
	defaultPreflightReleaseLagPatches  = 2
	defaultPreflightReleaseCheckTTLHrs = 24.0
	defaultDiskGuardMaxPauseSeconds    = 1800
)

type PreflightPolicy struct {
	MinFreeGiB           float64          `json:"min_free_gib"`
	ReleaseLagThreshold  int              `json:"release_lag_threshold"`
	ReleaseCheckTTLHours float64          `json:"release_check_ttl_hours"`
	DiskGuard            *DiskGuardPolicy `json:"disk_guard,omitempty"`
}

type DiskGuardPolicy struct {
	MaxPauseSeconds *int `json:"max_pause_s,omitempty"`
}

func (p *PreflightPolicy) validate() error {
	if p == nil || p.DiskGuard == nil || p.DiskGuard.MaxPauseSeconds == nil {
		return nil
	}
	if *p.DiskGuard.MaxPauseSeconds <= 0 {
		return errors.New("disk_guard.max_pause_s must be greater than zero")
	}
	return nil
}

type PreflightConfig struct {
	MinFreeGiB          float64
	resolvedFloorBytes  uint64
	ReleaseLagThreshold int
	ReleaseCheckTTL     time.Duration
	DiskPauseMax        time.Duration
}

func (p Policy) PreflightConfig() PreflightConfig {
	c := PreflightConfig{
		ReleaseLagThreshold: defaultPreflightReleaseLagPatches,
		ReleaseCheckTTL:     hoursToDuration(defaultPreflightReleaseCheckTTLHrs),
		DiskPauseMax:        defaultDiskGuardMaxPauseSeconds * time.Second,
	}
	if n, err := diskfloor.ApplyToPreflight(p.Disk, p.GC, p.Preflight, &c, nil); err == nil {
		c.resolvedFloorBytes = n
	}
	if p.Preflight == nil {
		return c
	}
	if p.Preflight.ReleaseLagThreshold > 0 {
		c.ReleaseLagThreshold = p.Preflight.ReleaseLagThreshold
	}
	if p.Preflight.ReleaseCheckTTLHours > 0 {
		c.ReleaseCheckTTL = hoursToDuration(p.Preflight.ReleaseCheckTTLHours)
	}
	if g := p.Preflight.DiskGuard; g != nil && g.MaxPauseSeconds != nil {
		c.DiskPauseMax = time.Duration(*g.MaxPauseSeconds) * time.Second
	}
	return c
}

func (p Policy) DiskFloor(warn io.Writer) (diskfloor.Resolution, error) {
	return diskfloor.Resolve(p.Disk, p.GC, p.Preflight, diskfloor.DefaultPreflightFloorBytes, warn)
}

func (p Policy) GCDiskFloor(warn io.Writer) (diskfloor.Resolution, error) {
	return diskfloor.Resolve(p.Disk, p.GC, p.Preflight, diskfloor.DefaultGCFloorBytes, warn)
}

func (p Policy) validateDiskFloor() error {
	_, err := p.DiskFloor(nil)
	return err
}

func hoursToDuration(h float64) time.Duration {
	return time.Duration(h * float64(time.Hour))
}

func (c PreflightConfig) MinFreeBytes() uint64 {
	if c.resolvedFloorBytes > 0 {
		return c.resolvedFloorBytes
	}
	return diskfloor.PreflightBytes(c)
}
