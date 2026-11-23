package main

import (
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/looppreflight"
)

const (
	gcScopeCache   = "cache"
	gcScopeSalvage = "salvage"
)

var gcDiskFreeBytes = looppreflight.DiskFreeBytes

var gcScopes = map[string]func(gcRun, string) int{
	gcScopeCache:   gcRun.cacheScope,
	gcScopeSalvage: gcRun.salvageScope,
}

func gcScopeNames() string {
	return strings.Join(slices.Sorted(maps.Keys(gcScopes)), ", ")
}

func (r gcRun) cacheScope(projectRoot string) int {
	projectRoot, pol, ok := r.loadPolicy(projectRoot)
	if !ok {
		return 1
	}
	r.diskSignals(projectRoot, pol)
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

func (r gcRun) salvageScope(projectRoot string) int {
	projectRoot, pol, ok := r.loadPolicy(projectRoot)
	if !ok {
		return 1
	}
	opts := worktreeGCOptions(projectRoot, filepath.Join(projectRoot, ".evolve"), pol.Worktrees, r.stdout)
	opts.Exec = r.exec
	items, err := gc.PlanSalvageRepack(opts)
	if err != nil {
		fmt.Fprintf(r.stderr, "evolve gc: salvage plan failed: %v\n", err)
		return 1
	}
	r.salvageReport(items)
	if r.dryRun {
		return 0
	}
	freed, err := gc.ApplySalvageRepack(opts, items)
	fmt.Fprintf(r.stdout, "evolve gc: salvage freed %s\n", gcSize(freed))
	if err != nil {
		fmt.Fprintf(r.stderr, "evolve gc: salvage apply failed: %v\n", err)
		return 1
	}
	return 0
}

func (r gcRun) salvageReport(items []gc.SalvageRepackItem) {
	var reclaimable int64
	for _, it := range items {
		fmt.Fprintf(r.stdout, "  %s %s %s (%s)\n", strings.ToUpper(string(it.Action)), it.Leaf, gcSize(it.Bytes), it.Reason)
		if it.Action != gc.SalvageKeep {
			reclaimable += it.Bytes
		}
	}
	r.summary("%d salvage bundle(s) listed, at most %s reclaimable", "%d salvage bundle(s) listed, at most %s reclaimable", len(items), gcSize(reclaimable))
}
