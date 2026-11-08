//go:build acs

package cycle2219

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	liveRunName = "cycle-1790"
	liveNotice  = "evolve gc: 1 run(s) hold a live lease (" + liveRunName + "), so the sweeps that stop processes (sessions, sockets, cycle and dispatch processes) wait for a wave boundary"
)

var processSweepLines = []string{
	"orphan session(s) would be reaped",
	"dead per-run socket(s) would be reaped",
	"finished-cycle orphan process(es) would be terminated",
	"dispatch process(es) would be stopped",
}

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func buildEvolve(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "evolve")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	build := exec.CommandContext(ctx, "go", "build", "-o", bin, "./cmd/evolve")
	build.Dir = goDir(t)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build evolve: %v\n%s", err, out)
	}
	return bin
}

type gcOutcome struct {
	stdout, stderr string
}

func runGCDryRun(t *testing.T, bin, root string, extra ...string) gcOutcome {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	args := append([]string{"gc", "--dry-run", "--project-root", root}, extra...)
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "TMUX_TMPDIR="+t.TempDir(), "GOCACHE="+t.TempDir())
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	if err := cmd.Run(); err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			t.Fatalf("run evolve gc: %v", err)
		}
	}
	return gcOutcome{stdout: so.String(), stderr: se.String()}
}

func newProject(t *testing.T) string {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Git("commit", "-q", "--allow-empty", "-m", "seed")
	if err := os.MkdirAll(filepath.Join(repo.Dir, ".evolve", "runs"), 0o755); err != nil {
		t.Fatal(err)
	}
	return repo.Dir
}

func writeLease(t *testing.T, root string, expired bool) {
	t.Helper()
	runDir := filepath.Join(root, ".evolve", "runs", liveRunName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	lease := runlease.Lease{RunID: "01LIVE", OwnerPID: os.Getpid()}
	heartbeat := time.Now()
	if expired {
		heartbeat = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	if err := runlease.Write(runDir, lease, heartbeat); err != nil {
		t.Fatal(err)
	}
}

func snapshot(t *testing.T, root string) []string {
	t.Helper()
	var entries []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return fs.SkipDir
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		entries = append(entries, rel+"|"+strconv.FormatInt(info.Size(), 10)+"|"+info.ModTime().UTC().Format(time.RFC3339Nano))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(entries)
	return entries
}

func TestC2219_001_LiveLeaseSuppressesProcessSweepPreview(t *testing.T) {
	bin := buildEvolve(t)
	root := newProject(t)
	writeLease(t, root, false)
	got := runGCDryRun(t, bin, root)
	if !strings.Contains(got.stderr, liveNotice) {
		t.Errorf("RED: stderr lacks the live lease line %q\nstderr=%s", liveNotice, got.stderr)
	}
	for _, banned := range processSweepLines {
		if strings.Contains(got.stdout, banned) {
			t.Errorf("RED: dry-run previews %q while a run holds a live lease\nstdout=%s", banned, got.stdout)
		}
	}
	if !strings.Contains(got.stdout, "workspace item(s) would be reaped") {
		t.Errorf("the live-lease preview must keep the sweeps a live run still performs\nstdout=%s", got.stdout)
	}
}

func TestC2219_002_LiveLeaseIsReportedInDryRunJSON(t *testing.T) {
	bin := buildEvolve(t)
	root := newProject(t)
	writeLease(t, root, false)
	got := runGCDryRun(t, bin, root, "--json")
	var envelope struct {
		Data struct {
			DryRun   bool     `json:"dry_run"`
			LiveRuns []string `json:"live_runs"`
			Report   []string `json:"report"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(got.stdout), &envelope); err != nil {
		t.Fatalf("unmarshal: %v\nstdout=%s", err, got.stdout)
	}
	result := envelope.Data
	if !result.DryRun || !slices.Equal(result.LiveRuns, []string{liveRunName}) {
		t.Errorf("RED: json dry_run=%v live_runs=%v, want true and [%s]\nstdout=%s", result.DryRun, result.LiveRuns, liveRunName, got.stdout)
	}
	for _, line := range result.Report {
		for _, banned := range processSweepLines {
			if strings.Contains(line, banned) {
				t.Errorf("RED: json report carries process sweep line %q", line)
			}
		}
	}
}

func TestC2219_003_NoLiveLeaseKeepsFullPreview(t *testing.T) {
	bin := buildEvolve(t)
	for name, leased := range map[string]bool{"no lease": false, "expired lease": true} {
		root := newProject(t)
		if leased {
			writeLease(t, root, true)
		}
		got := runGCDryRun(t, bin, root)
		for _, want := range processSweepLines[:3] {
			if !strings.Contains(got.stdout, want) {
				t.Errorf("%s: preview lost %q\nstdout=%s", name, want, got.stdout)
			}
		}
		if strings.Contains(got.stderr, "hold a live lease") {
			t.Errorf("%s: stderr reports a live lease: %s", name, got.stderr)
		}
	}
}

func TestC2219_004_DryRunTakesNoGCLeaseAndWritesNothing(t *testing.T) {
	bin := buildEvolve(t)
	root := newProject(t)
	writeLease(t, root, false)
	before := snapshot(t, root)
	got := runGCDryRun(t, bin, root)
	if _, err := os.Stat(filepath.Join(root, ".evolve", "gc.lock")); !os.IsNotExist(err) {
		t.Errorf("dry-run created gc.lock: err=%v", err)
	}
	if after := snapshot(t, root); !slices.Equal(before, after) {
		t.Errorf("dry-run changed the project tree\nbefore=%v\nafter=%v", before, after)
	}
	if !strings.Contains(got.stderr, liveNotice) {
		t.Errorf("RED: the write-nothing run must still report the live lease; stderr=%s", got.stderr)
	}
}

func TestC2219_005_RunGCIsFullyCovered(t *testing.T) {
	profile := filepath.Join(t.TempDir(), "cover.out")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	run := exec.CommandContext(ctx, "go", "test", "-count=1", "-run", "GC|Gc", "-coverprofile="+profile, "./cmd/evolve")
	run.Dir = goDir(t)
	if out, err := run.CombinedOutput(); err != nil {
		t.Fatalf("RED: gc tests fail: %v\n%s", err, acsassert.OutputTail(string(out), 40))
	}
	funcs, err := exec.CommandContext(ctx, "go", "tool", "cover", "-func="+profile).CombinedOutput()
	if err != nil {
		t.Fatalf("cover -func: %v\n%s", err, funcs)
	}
	pct, err := acsassert.FuncCoverage(string(funcs), "runGC")
	if err != nil {
		t.Fatal(err)
	}
	if pct < 100 {
		t.Errorf("runGC coverage = %.1f%%, want 100%%", pct)
	}
}
