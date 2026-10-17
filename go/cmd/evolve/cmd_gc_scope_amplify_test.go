package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
)

func TestRunGC_OnlyRefusesMalformedScopes(t *testing.T) {
	t.Parallel()
	for _, scope := range []string{"CACHE", "cache,temp", " cache", "all"} {
		scope := scope
		t.Run("scope="+scope, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			code := runGC([]string{"--only", scope, "--project-root", t.TempDir()}, nil, &stdout, &stderr)
			if code == 0 {
				t.Errorf("--only %q exited 0, want a refusal\nstdout=%s", scope, stdout.String())
			}
			if stdout.Len() != 0 {
				t.Errorf("--only %q wrote to stdout before refusing: %q", scope, stdout.String())
			}
		})
	}
}

func TestRunGC_OnlyCacheDryRunTrimsNothing(t *testing.T) {
	projectRoot, _, _ := gcWorktreeEnv(t, "")
	stale, fresh := gcFakeGoCache(t)
	gcWritePolicy(t, projectRoot, `{"go_cache_ttl_hours":24,"cache_trim_free_gb":50}`)
	withDiskFree(t, 10e9, nil)
	var stdout, stderr bytes.Buffer
	if code := runGC([]string{"--only", "cache", "--dry-run", "--project-root", projectRoot}, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
	}
	for _, path := range []string{stale, fresh} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("dry-run removed %s: %v", path, err)
		}
	}
}

func TestRunGC_OnlyCacheWithoutFloorTrimsStaleEntriesAndKeepsFreshOnes(t *testing.T) {
	projectRoot, _, _ := gcWorktreeEnv(t, "")
	stale, fresh := gcFakeGoCache(t)
	gcWritePolicy(t, projectRoot, `{"go_cache_ttl_hours":24}`)
	withDiskFree(t, 1e12, nil)
	var stdout, stderr bytes.Buffer
	if code := runGC([]string{"--only", "cache", "--project-root", projectRoot}, nil, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("stale entry survived an unset floor: %v", err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Errorf("fresh entry removed: %v", err)
	}
	if strings.Contains(stdout.String(), "skipped") {
		t.Errorf("unset floor must not skip the trim: %q", stdout.String())
	}
}

func TestCacheTrimFreeBytes_ALargeFloorNeverWrapsToASmallerOne(t *testing.T) {
	t.Parallel()
	small := (gcpolicy.Policy{CacheTrimFreeGB: 50}).CacheTrimFreeBytes()
	for _, gb := range []int{1 << 20, 1 << 40, int(^uint(0) >> 1)} {
		if got := (gcpolicy.Policy{CacheTrimFreeGB: gb}).CacheTrimFreeBytes(); got < small {
			t.Errorf("cache_trim_free_gb=%d gave %d bytes, below the 50 GB value %d: the product overflowed", gb, got, small)
		}
	}
}
