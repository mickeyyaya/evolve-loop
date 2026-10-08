package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

func TestRunGC_RemovesOnlyEmptyClaimDirsOfStaleCycles(t *testing.T) {
	projectRoot, _, _ := gcWorktreeEnv(t, "")
	gcFakeGoCache(t)
	processing := filepath.Join(projectRoot, ".evolve", "inbox", "processing")
	stale := filepath.Join(processing, "cycle-1700")
	live := filepath.Join(processing, "cycle-1837")
	holding := filepath.Join(processing, "cycle-1828")
	for _, d := range []string{stale, live, holding} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(holding, "item.json"), []byte(`{"id":"held"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	runDir := filepath.Join(projectRoot, ".evolve", "runs", "cycle-1837")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runlease.Write(runDir, runlease.Lease{RunID: "r", OwnerPID: os.Getpid()}, time.Now()); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	runGC([]string{"--dry-run", "--project-root", projectRoot}, nil, &stdout, &stderr)
	if !strings.Contains(stdout.String(), "WOULD-REMOVE "+stale) || strings.Contains(stdout.String(), "REMOVE "+live) || strings.Contains(stdout.String(), "REMOVE "+holding) {
		t.Errorf("--dry-run must list only the empty stale claim dir:\n%s", stdout.String())
	}
	if _, err := os.Stat(stale); err != nil {
		t.Fatalf("--dry-run removed %s", stale)
	}

	stdout.Reset()
	runGC([]string{"--project-root", projectRoot}, nil, &stdout, &stderr)
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("the empty stale claim dir must be gone: %v\n%s", err, stdout.String())
	}
	for _, d := range []string{live, holding} {
		if _, err := os.Stat(d); err != nil {
			t.Errorf("%s must stay: %v", d, err)
		}
	}
}

func TestRunGC_ReportsAClaimDirItCannotRemoveOrAnInboxItCannotRead(t *testing.T) {
	projectRoot, _, _ := gcWorktreeEnv(t, "")
	gcFakeGoCache(t)
	processing := filepath.Join(projectRoot, ".evolve", "inbox", "processing")
	stale := filepath.Join(processing, "cycle-1700")
	if err := os.MkdirAll(stale, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(processing, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(processing, 0o755) })
	var stdout, stderr bytes.Buffer
	if rc := runGC([]string{"--project-root", projectRoot}, nil, &stdout, &stderr); rc != 1 || !strings.Contains(stderr.String(), "evolve gc: claim dir "+stale+":") {
		t.Errorf("rc = %d stderr = %q; want exit 1 naming the dir", rc, stderr.String())
	}
	if _, err := os.Stat(stale); err != nil {
		t.Errorf("the dir stays: %v", err)
	}
	_ = os.Chmod(processing, 0o755)
	if err := os.RemoveAll(filepath.Join(projectRoot, ".evolve", "inbox")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, ".evolve", "inbox"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	stderr.Reset()
	if rc := runGC([]string{"--project-root", projectRoot}, nil, &stdout, &stderr); rc != 1 || !strings.Contains(stderr.String(), "evolve gc: claim dir survey failed:") {
		t.Errorf("rc = %d stderr = %q; want the survey failure", rc, stderr.String())
	}
}

func TestGCProject_RefusesAnUnresolvedRootAndWarnsOnABadPolicy(t *testing.T) {
	var stdout, stderr bytes.Buffer
	r := newGCRun(context.Background(), false, &stdout, &stderr)
	if rc := r.project(""); rc != 1 || stderr.String() != "evolve gc: mutating run refused: --project-root must be explicitly set\n" {
		t.Errorf("project(\"\") = %d, stderr %q; want the refusal", rc, stderr.String())
	}
	stderr.Reset()
	r.abs = func(string) (string, error) { return "", errors.New("cwd is gone") }
	if rc := r.project("relative-root"); rc != 1 || stderr.String() != "evolve gc: resolve --project-root: cwd is gone\n" {
		t.Errorf("project with an unresolvable root = %d, stderr %q; want exit 1 naming the fault", rc, stderr.String())
	}
	projectRoot, _, _ := gcWorktreeEnv(t, "")
	gcFakeGoCache(t)
	gcWritePolicy(t, projectRoot, `not json`)
	stderr.Reset()
	r = newGCRun(context.Background(), true, &stdout, &stderr)
	r.project(projectRoot)
	if !strings.Contains(stderr.String(), "evolve gc: WARN: policy load failed:") || !strings.Contains(stderr.String(), "using zero-value gc policy") {
		t.Errorf("stderr = %q; want the policy WARN", stderr.String())
	}
}
