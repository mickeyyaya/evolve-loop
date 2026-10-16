package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func gcTestCacheFile(t *testing.T, evolveDir, rel string, size int, age time.Duration) string {
	t.Helper()
	p := filepath.Join(evolveDir, "landing", "testcache", filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, bytes.Repeat([]byte("x"), size), 0o644); err != nil {
		t.Fatal(err)
	}
	mtime := time.Now().Add(-age)
	if err := os.Chtimes(p, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRunGC_TrimsTheScanTestCachePastItsTTLAndPreviewsFirst(t *testing.T) {
	projectRoot, evolveDir, _ := gcWorktreeEnv(t, "")
	stale := gcTestCacheFile(t, evolveDir, "aa/old.json", 100, 30*24*time.Hour)
	fresh := gcTestCacheFile(t, evolveDir, "bb/new.json", 100, time.Hour)
	gcWritePolicy(t, projectRoot, `{"testcache_ttl_days":7}`)

	var stdout, stderr bytes.Buffer
	runGC([]string{"--dry-run", "--project-root", projectRoot}, nil, &stdout, &stderr)
	if !strings.Contains(stdout.String(), "1 scan test cache file(s) would be trimmed") {
		t.Errorf("the preview does not name the stale entry:\n%s\nstderr=%s", stdout.String(), stderr.String())
	}
	if _, err := os.Stat(stale); err != nil {
		t.Fatalf("--dry-run removed the stale entry: %v", err)
	}

	stdout.Reset()
	runGC([]string{"--project-root", projectRoot}, nil, &stdout, &stderr)
	if !strings.Contains(stdout.String(), "trimmed 1 scan test cache file(s)") {
		t.Errorf("the real run does not report the trim:\n%s\nstderr=%s", stdout.String(), stderr.String())
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("the stale entry survived: %v", err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Errorf("a fresh entry was trimmed: %v", err)
	}
}

func TestRunGC_TheScanTestCacheSizeCapComesFromPolicy(t *testing.T) {
	projectRoot, evolveDir, _ := gcWorktreeEnv(t, "")
	big := gcTestCacheFile(t, evolveDir, "aa/big.json", 2<<20, time.Hour)
	gcWritePolicy(t, projectRoot, `{"testcache_max_mb":1}`)

	var stdout, stderr bytes.Buffer
	runGC([]string{"--project-root", projectRoot}, nil, &stdout, &stderr)

	if _, err := os.Stat(big); !os.IsNotExist(err) {
		t.Errorf("a file over the policy cap must go: %v\nstdout=%s", err, stdout.String())
	}
}

func TestRunGC_AScanTestCacheThatCannotBeListedFailsTheRunLoudly(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads every directory")
	}
	projectRoot, evolveDir, _ := gcWorktreeEnv(t, "")
	file := gcTestCacheFile(t, evolveDir, "aa/x.json", 10, time.Hour)
	locked := filepath.Dir(file)
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	var stdout, stderr bytes.Buffer
	code := runGC([]string{"--project-root", projectRoot}, nil, &stdout, &stderr)

	if code == 0 || !strings.Contains(stderr.String(), "scan test cache error") {
		t.Errorf("a listing fault must show and fail the run; code=%d stderr=%s", code, stderr.String())
	}
}
