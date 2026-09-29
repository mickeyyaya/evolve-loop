package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func gcFakeGoCache(t *testing.T) (stale, fresh string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("GOCACHE", dir)
	stale = filepath.Join(dir, "0a", "0a1b2c-d")
	fresh = filepath.Join(dir, "0b", "0b1b2c-d")
	for p, age := range map[string]time.Duration{stale: 30 * time.Hour, fresh: time.Hour} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("cached output"), 0o644); err != nil {
			t.Fatal(err)
		}
		mtime := time.Now().Add(-age)
		if err := os.Chtimes(p, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
	return stale, fresh
}

func gcWritePolicy(t *testing.T, projectRoot, gcBlock string) {
	t.Helper()
	p := filepath.Join(projectRoot, ".evolve", "policy.json")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(`{"gc":`+gcBlock+`}`), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRunGC_TrimsGoBuildCacheEntriesUnusedPastThePolicyTTL(t *testing.T) {
	projectRoot, _, _ := gcWorktreeEnv(t, "")
	stale, fresh := gcFakeGoCache(t)
	gcWritePolicy(t, projectRoot, `{"go_cache_ttl_hours":24}`)

	var stdout, stderr bytes.Buffer
	runGC([]string{"--dry-run", "--project-root", projectRoot}, nil, &stdout, &stderr)
	if !strings.Contains(stdout.String(), "1 go build cache file(s) would be trimmed") {
		t.Errorf("--dry-run does not preview the stale entry:\n%s\nstderr=%s", stdout.String(), stderr.String())
	}
	if _, err := os.Stat(stale); err != nil {
		t.Fatalf("--dry-run removed the stale entry: %v", err)
	}

	stdout.Reset()
	runGC([]string{"--project-root", projectRoot}, nil, &stdout, &stderr)
	if !strings.Contains(stdout.String(), "trimmed 1 go build cache file(s)") {
		t.Errorf("the explicit run does not report the trim:\n%s\nstderr=%s", stdout.String(), stderr.String())
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("stale entry survived `evolve gc`: %v", err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Errorf("an entry used an hour ago was trimmed: %v", err)
	}
}

func TestRunGC_LeavesTheGoBuildCacheAloneWithoutAPolicyTTL(t *testing.T) {
	projectRoot, _, _ := gcWorktreeEnv(t, "")
	stale, _ := gcFakeGoCache(t)

	var stdout, stderr bytes.Buffer
	runGC([]string{"--project-root", projectRoot}, nil, &stdout, &stderr)

	if _, err := os.Stat(stale); err != nil {
		t.Errorf("the cache was trimmed with gc.go_cache_ttl_hours unset: %v", err)
	}
	if !strings.Contains(stdout.String(), "go build cache trim off") {
		t.Errorf("the skipped trim is not reported:\n%s", stdout.String())
	}
}

func TestRunGC_ReportsFreeDiskBeforeAndAfterARealRunOnly(t *testing.T) {
	projectRoot, _, _ := gcWorktreeEnv(t, "")
	gcFakeGoCache(t)

	var stdout, stderr bytes.Buffer
	runGC([]string{"--dry-run", "--project-root", projectRoot}, nil, &stdout, &stderr)
	if strings.Contains(stdout.String(), "disk free") {
		t.Errorf("a preview released nothing, so it must not report a disk delta:\n%s", stdout.String())
	}

	stdout.Reset()
	runGC([]string{"--project-root", projectRoot}, nil, &stdout, &stderr)
	if !strings.Contains(stdout.String(), "evolve gc: disk free ") || !strings.Contains(stdout.String(), "released") {
		t.Errorf("the real run does not report the disk it freed:\n%s\nstderr=%s", stdout.String(), stderr.String())
	}
}
