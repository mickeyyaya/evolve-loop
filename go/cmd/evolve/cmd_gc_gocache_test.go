package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
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

func TestRunGC_WithoutAPolicyTTLOnlyTheDefaultCapAppliesAndASmallCacheIsLeftAlone(t *testing.T) {
	projectRoot, _, _ := gcWorktreeEnv(t, "")
	stale, _ := gcFakeGoCache(t)

	var stdout, stderr bytes.Buffer
	runGC([]string{"--project-root", projectRoot}, nil, &stdout, &stderr)

	if _, err := os.Stat(stale); err != nil {
		t.Errorf("the cache was trimmed with gc.go_cache_ttl_hours unset and the cache far under its cap: %v", err)
	}
	for _, want := range []string{"go build cache age trim off", "trimmed 0 go build cache file(s) to fit the 20.00 GB cap"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("stdout does not report %q:\n%s\nstderr=%s", want, stdout.String(), stderr.String())
		}
	}
}

func gcSparseCacheEntry(t *testing.T, dir, rel string, size int64, age time.Duration) string {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(size); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	mtime := time.Now().Add(-age)
	if err := os.Chtimes(p, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRunGC_TrimsTheGoBuildCacheToItsCapLeastRecentlyUsedFirst(t *testing.T) {
	projectRoot, _, _ := gcWorktreeEnv(t, "")
	cache := t.TempDir()
	t.Setenv("GOCACHE", cache)
	oldest := gcSparseCacheEntry(t, cache, "0a/0a01-d", 1e9, 20*time.Hour)
	older := gcSparseCacheEntry(t, cache, "0b/0b01-d", 1e9, 10*time.Hour)
	recent := gcSparseCacheEntry(t, cache, "0c/0c01-a", 1e9, 5*time.Hour)
	gcWritePolicy(t, projectRoot, `{"go_cache_ttl_hours":24,"go_cache_max_gb":2}`)

	var stdout, stderr bytes.Buffer
	runGC([]string{"--dry-run", "--project-root", projectRoot}, nil, &stdout, &stderr)
	if want := "1 go build cache file(s) would be trimmed to fit the 2.00 GB cap (1.00 GB, least recently used first); 2.00 GB would remain"; !strings.Contains(stdout.String(), want) {
		t.Errorf("--dry-run does not preview the cap trim %q:\n%s\nstderr=%s", want, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(oldest); err != nil {
		t.Fatalf("--dry-run removed the least recently used entry: %v", err)
	}

	stdout.Reset()
	runGC([]string{"--project-root", projectRoot}, nil, &stdout, &stderr)
	if want := "trimmed 1 go build cache file(s) to fit the 2.00 GB cap (1.00 GB, least recently used first); 2.00 GB remains"; !strings.Contains(stdout.String(), want) {
		t.Errorf("the explicit run does not report the bytes the cap freed %q:\n%s\nstderr=%s", want, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(oldest); !os.IsNotExist(err) {
		t.Errorf("the least recently used entry survived the cap: %v", err)
	}
	for _, kept := range []string{older, recent} {
		if _, err := os.Stat(kept); err != nil {
			t.Errorf("%s was trimmed although the cache already fit its cap without it: %v", kept, err)
		}
	}
}

func TestRunGC_WarnsWhenEntriesInUseKeepTheCacheOverItsCap(t *testing.T) {
	projectRoot, _, _ := gcWorktreeEnv(t, "")
	cache := t.TempDir()
	t.Setenv("GOCACHE", cache)
	inUse := gcSparseCacheEntry(t, cache, "0a/0a01-d", 3e9, 10*time.Minute)
	gcWritePolicy(t, projectRoot, `{"go_cache_max_gb":1}`)

	var stdout, stderr bytes.Buffer
	runGC([]string{"--project-root", projectRoot}, nil, &stdout, &stderr)

	if _, err := os.Stat(inUse); err != nil {
		t.Fatalf("an entry used 10 minutes ago was trimmed: %v", err)
	}
	if want := "go build cache still holds 3.00 GB, over its 1.00 GB cap"; !strings.Contains(stderr.String(), want) {
		t.Errorf("stderr does not warn %q:\n%s", want, stderr.String())
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

func TestGCGoCache_AHostWithoutGoHasNoCacheToTrimWhileAnUnusableCacheFailsTheStep(t *testing.T) {
	cases := []struct {
		name       string
		env        [2]string
		wantFailed bool
		want       string
	}{
		{"no go toolchain on PATH", [2]string{"PATH", t.TempDir()}, false, "no go toolchain on PATH"},
		{"the cache is switched off", [2]string{"GOCACHE", "off"}, true, `GOCACHE is "off"`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv(c.env[0], c.env[1])
			var stdout, stderr bytes.Buffer
			r := newGCRun(context.Background(), true, &stdout, &stderr)

			failed := r.goCache(gc.Policy{})

			if failed != c.wantFailed || !strings.Contains(stdout.String()+stderr.String(), c.want) {
				t.Errorf("goCache failed=%v, want %v naming %q\nstdout=%s\nstderr=%s", failed, c.wantFailed, c.want, stdout.String(), stderr.String())
			}
		})
	}
}
