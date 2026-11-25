package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

const gcSalvageLeaf = "cycle-cccc-31"

type gcSalvageEnv struct {
	root   string
	repo   *gittest.Repo
	bundle string
}

func newGCSalvageEnv(t *testing.T) *gcSalvageEnv {
	t.Helper()
	root, _, _ := gcWorktreeEnv(t, "")
	repo := gittest.At(t, root)
	bulk := make([]byte, 3<<20)
	if _, err := rand.Read(bulk); err != nil {
		t.Fatal(err)
	}
	repo.Write("bulk.bin", string(bulk))
	repo.Git("add", "bulk.bin")
	repo.Git("commit", "-q", "-m", "bulk history")
	repo.Git("update-ref", "refs/remotes/origin/main", "refs/heads/main")
	repo.Git("checkout", "-q", "-b", "lane/salvage")
	repo.Write("lane.txt", "lane\n")
	repo.Git("add", "lane.txt")
	repo.Git("commit", "-q", "-m", "lane work")
	tip := repo.Git("rev-parse", "HEAD")
	repo.Git("checkout", "-q", "main")
	leaf := filepath.Join(root, ".evolve", "operator-salvage", gcSalvageLeaf)
	if err := os.MkdirAll(leaf, 0o755); err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(leaf, "refs.bundle")
	repo.Git("bundle", "create", bundle, "refs/heads/lane/salvage")
	if err := os.WriteFile(filepath.Join(leaf, "HEAD"), []byte(tip+" lane/salvage\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-3 * time.Hour)
	for _, p := range []string{bundle, filepath.Join(leaf, "HEAD"), leaf} {
		if err := os.Chtimes(p, old, old); err != nil {
			t.Fatal(err)
		}
	}
	return &gcSalvageEnv{root: root, repo: repo, bundle: bundle}
}

func (e *gcSalvageEnv) run(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := runGC(append([]string{"--only", "salvage", "--project-root", e.root}, args...), nil, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func gcSalvageStamp(t *testing.T, p string) (int64, time.Time) {
	t.Helper()
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	return info.Size(), info.ModTime()
}

func TestRunGC_OnlySalvageDryRunListsTheRepackAndChangesNothing(t *testing.T) {
	e := newGCSalvageEnv(t)
	size, mtime := gcSalvageStamp(t, e.bundle)

	code, stdout, stderr := e.run(t, "--dry-run")

	if code != 0 {
		t.Fatalf("exit %d\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "REPACK "+gcSalvageLeaf) || !strings.Contains(stdout, "evolve gc --dry-run: ") {
		t.Errorf("stdout = %q, want a REPACK line for the leaf and the dry-run prefix", stdout)
	}
	gotSize, gotTime := gcSalvageStamp(t, e.bundle)
	if gotSize != size || !gotTime.Equal(mtime) {
		t.Errorf("the dry-run changed the bundle: %d %v, want %d %v", gotSize, gotTime, size, mtime)
	}
	if _, err := os.Stat(filepath.Join(e.root, ".evolve", "archive", "salvage-repack.jsonl")); !os.IsNotExist(err) {
		t.Errorf("the dry-run wrote an audit row: %v", err)
	}
}

func TestRunGC_OnlySalvageRepacksTheBundleAndASecondRunFindsNothingToDo(t *testing.T) {
	e := newGCSalvageEnv(t)
	size, _ := gcSalvageStamp(t, e.bundle)
	headsBefore := e.repo.Git("bundle", "list-heads", e.bundle)

	code, stdout, stderr := e.run(t)

	if code != 0 {
		t.Fatalf("exit %d\nstdout=%s\nstderr=%s", code, stdout, stderr)
	}
	after, _ := gcSalvageStamp(t, e.bundle)
	if after >= 1<<20 || after >= size {
		t.Errorf("bundle is %d bytes, was %d, want a thin bundle under 1 MiB", after, size)
	}
	if got := e.repo.Git("bundle", "list-heads", e.bundle); got != headsBefore {
		t.Errorf("heads = %q, want %q", got, headsBefore)
	}
	if !strings.Contains(stdout, "REPACK "+gcSalvageLeaf) || !strings.Contains(stdout, "freed") {
		t.Errorf("stdout = %q", stdout)
	}
	raw, err := os.ReadFile(filepath.Join(e.root, ".evolve", "archive", "salvage-repack.jsonl"))
	if err != nil || strings.Count(string(raw), "\n") != 1 {
		t.Errorf("audit = %q, %v, want one row", raw, err)
	}

	_, again, _ := e.run(t, "--dry-run")
	if !strings.Contains(again, "KEEP "+gcSalvageLeaf) || strings.Contains(again, "REPACK") {
		t.Errorf("second run = %q, want KEEP only", again)
	}
}

func TestRunGC_OnlySalvageJSONNamesTheScope(t *testing.T) {
	e := newGCSalvageEnv(t)

	code, stdout, stderr := e.run(t, "--dry-run", "--json")

	var got gcResult
	if err := json.Unmarshal([]byte(stdout), &got); code != 0 || err != nil || got.Scope != "salvage" || !got.DryRun {
		t.Errorf("exit %d, err %v, result %+v\nstderr=%s", code, err, got, stderr)
	}
}

func TestRunGC_OnlySalvageRefusesWithoutOriginMain(t *testing.T) {
	e := newGCSalvageEnv(t)
	e.repo.Git("update-ref", "-d", "refs/remotes/origin/main")

	code, _, stderr := e.run(t, "--dry-run")

	if code != 1 || !strings.Contains(stderr, "salvage plan failed") || !strings.Contains(stderr, "origin/main") {
		t.Errorf("exit %d, stderr = %q", code, stderr)
	}
}

func TestRunGC_OnlySalvageReportsAnApplyFailureAndExitsOne(t *testing.T) {
	e := newGCSalvageEnv(t)
	if err := os.WriteFile(filepath.Join(e.root, ".evolve", "archive"), []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	size, _ := gcSalvageStamp(t, e.bundle)

	code, _, stderr := e.run(t)

	if code != 1 || !strings.Contains(stderr, "salvage apply failed") {
		t.Errorf("exit %d, stderr = %q", code, stderr)
	}
	if got, _ := gcSalvageStamp(t, e.bundle); got != size {
		t.Errorf("bundle changed to %d bytes after a failed apply", got)
	}
}

func TestGCRun_SalvageScopeStopsWhenTheProjectRootCannotBeResolved(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	r := newGCRun(context.Background(), true, &stdout, &stderr)
	r.abs = func(string) (string, error) { return "", errors.New("no cwd") }

	if rc := r.salvageScope("x"); rc != 1 {
		t.Errorf("rc = %d, want 1", rc)
	}
}

func TestRunGC_OnlySalvageRetentionDryRunAndApplyHonorCap(t *testing.T) {
	e := newGCSalvageEnv(t)
	e.repo.Write(".evolve/policy.json", `{"gc":{"salvage_max_mb":1}}`)
	e.repo.Git("checkout", "-q", "-b", "lane/landed")
	e.repo.Write("landed.txt", "landed\n")
	e.repo.Git("add", "landed.txt")
	e.repo.Git("commit", "-q", "-m", "landed")
	landedTip := e.repo.Git("rev-parse", "HEAD")
	e.repo.Git("checkout", "-q", "main")
	e.repo.Git("merge", "--ff-only", "lane/landed")
	e.repo.Git("update-ref", "refs/remotes/origin/main", "refs/heads/main")

	landedLeaf := filepath.Join(e.root, ".evolve", "operator-salvage", "cycle-cccc-32")
	if err := os.MkdirAll(landedLeaf, 0o755); err != nil {
		t.Fatal(err)
	}
	pad := make([]byte, 2<<20)
	if _, err := rand.Read(pad); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(landedLeaf, "HEAD"), []byte(landedTip+" lane/landed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(landedLeaf, "refs.bundle"), pad, 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-10 * 24 * time.Hour)
	for _, p := range []string{filepath.Join(landedLeaf, "HEAD"), filepath.Join(landedLeaf, "refs.bundle"), landedLeaf} {
		if err := os.Chtimes(p, old, old); err != nil {
			t.Fatal(err)
		}
	}

	code, stdout, stderr := e.run(t, "--dry-run")
	if code != 0 || !strings.Contains(stdout, "EVICT cycle-cccc-32") {
		t.Fatalf("dry-run code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if _, err := os.Stat(landedLeaf); err != nil {
		t.Fatalf("dry-run deleted leaf: %v", err)
	}

	code, stdout, stderr = e.run(t)
	if code != 0 || !strings.Contains(stdout, "salvage retention evicted") {
		t.Fatalf("apply code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if _, err := os.Stat(landedLeaf); !os.IsNotExist(err) {
		t.Errorf("landed leaf still present after apply: %v", err)
	}
}

func TestGCRun_SalvageRetentionErrors(t *testing.T) {
	e := newGCSalvageEnv(t)
	e.repo.Git("checkout", "-q", "-b", "lane/landed-no-bundle")
	e.repo.Write("nb.txt", "nb\n")
	e.repo.Git("add", "nb.txt")
	e.repo.Git("commit", "-q", "-m", "nb")
	landedTip := e.repo.Git("rev-parse", "HEAD")
	e.repo.Git("checkout", "-q", "main")
	e.repo.Git("merge", "--ff-only", "lane/landed-no-bundle")
	e.repo.Git("update-ref", "refs/remotes/origin/main", "refs/heads/main")
	e.repo.Write(".evolve/policy.json", `{"gc":{"salvage_max_mb":1}}`)

	landedLeaf := filepath.Join(e.root, ".evolve", "operator-salvage", "cycle-cccc-33")
	if err := os.MkdirAll(landedLeaf, 0o755); err != nil {
		t.Fatal(err)
	}
	pad := make([]byte, 2<<20)
	if _, err := rand.Read(pad); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(landedLeaf, "HEAD"), []byte(landedTip+" lane/landed-no-bundle\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(landedLeaf, "data.bin"), pad, 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-10 * 24 * time.Hour)
	for _, p := range []string{filepath.Join(landedLeaf, "HEAD"), filepath.Join(landedLeaf, "data.bin"), landedLeaf} {
		if err := os.Chtimes(p, old, old); err != nil {
			t.Fatal(err)
		}
	}

	e.repo.Git("update-ref", "-d", "refs/remotes/origin/main")
	var stdout, stderr bytes.Buffer
	r := newGCRun(context.Background(), true, &stdout, &stderr)
	opts := gc.WorktreeOptions{ProjectRoot: e.root, EvolveDir: filepath.Join(e.root, ".evolve"), Exec: r.exec}
	if rc := r.salvageDryRetention(opts, 1<<20); rc != 1 || !strings.Contains(stderr.String(), "salvage retention plan failed") {
		t.Errorf("dry retention rc=%d stderr=%q", rc, stderr.String())
	}
	stderr.Reset()
	if rc := r.salvageApplyRetention(opts, 1<<20); rc != 1 || !strings.Contains(stderr.String(), "salvage retention plan failed") {
		t.Errorf("apply retention rc=%d stderr=%q", rc, stderr.String())
	}

	e.repo.Git("update-ref", "refs/remotes/origin/main", "refs/heads/main")
	if err := os.RemoveAll(filepath.Join(e.root, ".evolve", "operator-salvage", gcSalvageLeaf)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(e.root, ".evolve", "archive"), []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, errOut := e.run(t)
	if code != 1 || !strings.Contains(errOut, "salvage retention apply failed") {
		t.Errorf("apply error code=%d stderr=%q", code, errOut)
	}
}
