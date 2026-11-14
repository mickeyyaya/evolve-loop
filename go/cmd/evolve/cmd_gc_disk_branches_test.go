package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
)

func TestDiskSignals_ATempDirThatIsTheProjectRootIsCheckedOnce(t *testing.T) {
	calls := 0
	prev := gcDiskFreeBytes
	gcDiskFreeBytes = func(string) (uint64, error) { calls++; return 1e9, nil }
	t.Cleanup(func() { gcDiskFreeBytes = prev })
	var stdout, stderr bytes.Buffer

	newGCRun(context.Background(), true, &stdout, &stderr).diskSignals(os.TempDir(), gc.Policy{})

	if calls != 1 || strings.Count(stderr.String(), "gc.disk_floor_gb") != 1 {
		t.Errorf("free disk read %d time(s), stderr %q; want one check of the shared path", calls, stderr.String())
	}
}

func TestCacheScope_AnUnresolvableRootAndAFailedCacheStepEachExitOne(t *testing.T) {
	withDiskFree(t, 1e9, nil)
	var stdout, stderr bytes.Buffer
	r := newGCRun(context.Background(), true, &stdout, &stderr)
	r.abs = func(string) (string, error) { return "", errors.New("no cwd") }
	if code := r.cacheScope("rel"); code != 1 || !strings.Contains(stderr.String(), "resolve --project-root: no cwd") {
		t.Errorf("unresolvable root: exit %d, stderr %q; want 1 and the named error", code, stderr.String())
	}

	projectRoot, _, _ := gcWorktreeEnv(t, "")
	stale, _ := gcFakeGoCache(t)
	gcWritePolicy(t, projectRoot, `{"go_cache_ttl_hours":24}`)
	stdout.Reset()
	stderr.Reset()
	r = newGCRun(context.Background(), false, &stdout, &stderr)
	r.remove = func(string) error { return errors.New("read-only cache") }
	if code := r.cacheScope(projectRoot); code != 1 || !strings.Contains(stderr.String(), "go build cache error") {
		t.Errorf("failed cache step: exit %d, stdout %q; want 1 and the cache error", code, stdout.String())
	}
	if _, err := os.Stat(stale); err != nil {
		t.Errorf("the stale entry is gone although remove failed: %v", err)
	}
}

func TestPipelineTemp_SparesAHeldArtifactAndNamesARemoveError(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	old := time.Now().Add(-48 * time.Hour)
	held, stuck := filepath.Join(tmp, "go-build123456"), filepath.Join(tmp, "go-build654321")
	for _, dir := range []string{held, stuck} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(dir, old, old); err != nil {
			t.Fatal(err)
		}
	}
	var stdout, stderr bytes.Buffer
	r := newGCRun(context.Background(), false, &stdout, &stderr)
	r.exec = func(_ context.Context, name, _ string, _, _ []string, _ io.Reader, out, _ io.Writer) (int, error) {
		if name == "lsof" {
			_, _ = io.WriteString(out, "p4242\nn"+held+"\n")
		}
		return 0, nil
	}
	r.removeAll = func(string) error { return errors.New("directory busy") }

	failed := r.pipelineTemp(1)

	if !failed || !strings.Contains(stderr.String(), "evolve gc: temp error:") || !strings.Contains(stderr.String(), "directory busy") {
		t.Errorf("failed=%v stderr %q; want a failed sweep that names the remove error", failed, stderr.String())
	}
	if !strings.Contains(stdout.String(), "1 stale pipeline temp artifact(s) spared: a live process holds them") {
		t.Errorf("stdout %q does not name the spared artifact", stdout.String())
	}
	for _, dir := range []string{held, stuck} {
		if _, err := os.Stat(dir); err != nil {
			t.Errorf("%s was removed: %v", dir, err)
		}
	}
}
