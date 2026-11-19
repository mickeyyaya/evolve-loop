package main

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/testverdict"
)

func trimTestCache(evolveDir string, pol gc.Policy, remove func(string) error) testverdict.TrimReport {
	dir := filepath.Join(evolveDir, "landing", "testcache")
	return testverdict.Trim(dir, time.Now(), pol.TestCacheTTL(), pol.TestCacheMaxBytes(), remove)
}

func (r gcRun) testCache(evolveDir string, pol gc.Policy) bool {
	dir := filepath.Join(evolveDir, "landing", "testcache")
	ttl, capBytes := pol.TestCacheTTL(), pol.TestCacheMaxBytes()
	rep := trimTestCache(evolveDir, pol, r.remove)
	r.summary("%d scan test cache file(s) would be trimmed (%s, older than %s or over the %s cap in %s)", "trimmed %d scan test cache file(s) (%s, older than %s or over the %s cap in %s)", rep.Files, gcSize(rep.Bytes), ttl, gcSize(capBytes), dir)
	for _, e := range rep.Errors {
		fmt.Fprintf(r.stderr, "evolve gc: scan test cache error: %s\n", e)
	}
	return len(rep.Errors) > 0
}
