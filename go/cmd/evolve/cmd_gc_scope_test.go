package main

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
)

func withDiskFree(t *testing.T, free uint64, err error) {
	t.Helper()
	prev := gcDiskFreeBytes
	gcDiskFreeBytes = func(string) (uint64, error) { return free, err }
	t.Cleanup(func() { gcDiskFreeBytes = prev })
}

func TestRunGC_OnlyCacheTrimsWhileDiskFreeIsUnderTheFloor(t *testing.T) {
	cases := []struct {
		name      string
		free      uint64
		freeErr   error
		wantTrim  bool
		wantInOut string
	}{
		{"under the floor", 10e9, nil, true, "trimmed 1 go build cache file(s)"},
		{"at the floor", 50e9, nil, false, "go build cache trim skipped: disk free 50.00 GB is at or above gc.cache_trim_free_gb (50.00 GB)"},
		{"disk free unreadable", 0, errors.New("statfs failed"), true, "trimmed 1 go build cache file(s)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			projectRoot, _, _ := gcWorktreeEnv(t, "")
			stale, fresh := gcFakeGoCache(t)
			gcWritePolicy(t, projectRoot, `{"go_cache_ttl_hours":24,"cache_trim_free_gb":50}`)
			withDiskFree(t, tc.free, tc.freeErr)
			var stdout, stderr bytes.Buffer
			if code := runGC([]string{"--only", "cache", "--project-root", projectRoot}, nil, &stdout, &stderr); code != 0 {
				t.Fatalf("exit %d\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
			}
			_, staleErr := os.Stat(stale)
			if trimmed := os.IsNotExist(staleErr); trimmed != tc.wantTrim {
				t.Errorf("stale cache entry trimmed=%v, want %v\nstdout=%s", trimmed, tc.wantTrim, stdout.String())
			}
			if _, err := os.Stat(fresh); err != nil {
				t.Errorf("fresh cache entry removed: %v", err)
			}
			if !strings.Contains(stdout.String(), tc.wantInOut) {
				t.Errorf("stdout = %q, want it to hold %q", stdout.String(), tc.wantInOut)
			}
			if tc.freeErr != nil && !strings.Contains(stderr.String(), "statfs failed") {
				t.Errorf("stderr = %q, want the disk free error named", stderr.String())
			}
		})
	}
}

func TestRunGC_OnlyRefusesAnUnknownScopeBeforeAnySweep(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if code := runGC([]string{"--only", "worktrees", "--project-root", t.TempDir()}, nil, &stdout, &stderr); code != 10 {
		t.Errorf("exit %d, want 10", code)
	}
	if !strings.Contains(stderr.String(), `unknown --only scope "worktrees" (valid: cache)`) || stdout.Len() != 0 {
		t.Errorf("stdout=%q stderr=%q, want only the refusal on stderr", stdout.String(), stderr.String())
	}
}
