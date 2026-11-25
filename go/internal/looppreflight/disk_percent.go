package looppreflight

import (
	"fmt"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/diskfloor"
)

const (
	defaultDiskWarnPercent  = 10.0
	defaultDiskPausePercent = 5.0
)

type DiskGuardConfig struct {
	WarnPercent  float64
	PausePercent float64
	FloorBytes   uint64
}

func DefaultDiskGuardConfig() DiskGuardConfig {
	return DiskGuardConfig{
		WarnPercent:  defaultDiskWarnPercent,
		PausePercent: defaultDiskPausePercent,
	}
}

type DiskGuard struct {
	dir     string
	cfg     DiskGuardConfig
	usage   func(string) (DiskUsage, error)
	warn    io.Writer
	paused  bool
	freePct float64
}

type DiskStatus struct {
	Paused   bool
	FreePct  float64
	PausePct float64
	Path     string
}

func NewDiskGuard(dir string, cfg DiskGuardConfig, usage func(string) (DiskUsage, error), warn io.Writer) *DiskGuard {
	if cfg.WarnPercent <= 0 {
		cfg.WarnPercent = defaultDiskWarnPercent
	}
	if cfg.PausePercent <= 0 {
		cfg.PausePercent = defaultDiskPausePercent
	}
	if usage == nil {
		usage = DiskUsageOf
	}
	if warn == nil {
		warn = io.Discard
	}
	return &DiskGuard{dir: dir, cfg: cfg, usage: usage, warn: warn}
}

func (g *DiskGuard) Check() bool {
	return g.Measure().Paused
}

func (g *DiskGuard) Measure() DiskStatus {
	paused := g.probe()
	return DiskStatus{Paused: paused, FreePct: g.freePct, PausePct: g.cfg.PausePercent, Path: g.dir}
}

func (g *DiskGuard) probe() bool {
	u, err := g.usage(g.dir)
	if err != nil {
		fmt.Fprintf(g.warn, "[disk-guard] WARN: free disk space at %s unreadable: %v\n", g.dir, err)
		return g.paused
	}
	if u.TotalBytes == 0 {
		fmt.Fprintf(g.warn, "[disk-guard] WARN: free disk space at %s unmeasurable (total bytes is 0)\n", g.dir)
		return g.paused
	}
	pct := (float64(u.FreeBytes) / float64(u.TotalBytes)) * 100
	g.freePct = pct
	underFloor := g.cfg.FloorBytes > 0 && u.FreeBytes < g.cfg.FloorBytes
	if pct < g.cfg.PausePercent || underFloor {
		g.paused = true
		if pct < g.cfg.PausePercent {
			fmt.Fprintf(g.warn, "[disk-guard] WARN: free disk space at %s is %.1f%% (below %.0f%% pause threshold) — pausing new dispatches\n", g.dir, pct, g.cfg.PausePercent)
		}
		if underFloor {
			fmt.Fprintf(g.warn, "[disk-guard] WARN: free disk space at %s is %.2f GB (below %.2f GB %s) — pausing new dispatches\n", g.dir, float64(u.FreeBytes)/float64(diskfloor.BytesPerGB), float64(g.cfg.FloorBytes)/float64(diskfloor.BytesPerGB), diskfloor.KeyFloorGB)
		}
		return true
	}
	g.paused = false
	if pct < g.cfg.WarnPercent {
		fmt.Fprintf(g.warn, "[disk-guard] WARN: free disk space at %s is %.1f%% (below %.0f%% warn threshold)\n", g.dir, pct, g.cfg.WarnPercent)
	}
	return false
}
