package main

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/looppreflight"
)

const gcScopeCache = "cache"

var gcDiskFreeBytes = looppreflight.DiskFreeBytes

func (r gcRun) cacheScope(projectRoot string) int {
	projectRoot, pol, ok := r.loadPolicy(projectRoot)
	if !ok {
		return 1
	}
	if r.diskFreeAboveTrim(projectRoot, pol) {
		return 0
	}
	failed := r.step(func(s gcRun) bool { return s.goCache(pol) })
	failed = r.step(func(s gcRun) bool { return s.pipelineTemp(pol.TempTTLHours) }) || failed
	if failed {
		return 1
	}
	return 0
}

func (r gcRun) diskFreeAboveTrim(projectRoot string, pol gc.Policy) bool {
	floor := pol.CacheTrimFreeBytes()
	if floor == 0 {
		return false
	}
	free, err := gcDiskFreeBytes(projectRoot)
	if err != nil {
		fmt.Fprintf(r.stderr, "evolve gc: WARN: disk free unreadable (%v); trimming the cache anyway\n", err)
		return false
	}
	if free < floor {
		return false
	}
	fmt.Fprintf(r.stdout, "evolve gc: go build cache trim skipped: disk free %s is at or above gc.cache_trim_free_gb (%s)\n", gcSize(int64(free)), gcSize(int64(floor)))
	return true
}
