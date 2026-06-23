package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/looppreflight"
	"github.com/mickeyyaya/evolveloop/go/test/fixtures"
)

// forcedHalt is a looppreflight.Result that always halts — the seam value tests
// inject to drive the gate's abort path without a real environment probe.
func forcedHalt() looppreflight.Result {
	return looppreflight.Result{
		Checks:       []looppreflight.CheckResult{{Name: "bridge-boot", Level: looppreflight.LevelHalt, Message: "forced", Detail: "rc=80"}},
		ChecksTotal:  1,
		OverallLevel: looppreflight.LevelHalt,
		GeneratedAt:  "1970-01-01T00:00:00Z",
	}
}

func TestLoopPreflightHalts_SeamHalt_WritesFileAndHalts(t *testing.T) {
	dir := t.TempDir()
	evolveDir := filepath.Join(dir, ".evolve")

	prev := runLoopPreflightFn
	defer func() { runLoopPreflightFn = prev }()
	runLoopPreflightFn = func(loopConfig, io.Writer) looppreflight.Result { return forcedHalt() }

	var stderr bytes.Buffer
	if !loopPreflightHalts(loopConfig{ProjectRoot: dir, EvolveDir: evolveDir}, &stderr) {
		t.Fatalf("expected the gate to halt")
	}
	if _, err := os.Stat(filepath.Join(evolveDir, "loop-preflight.json")); err != nil {
		t.Fatalf("expected .evolve/loop-preflight.json to be persisted: %v", err)
	}
	if !strings.Contains(stderr.String(), "bridge-boot") {
		t.Fatalf("expected the readiness summary on stderr; got %q", stderr.String())
	}
}

func TestLoopPreflightHalts_SkipEnv_BypassesSeam(t *testing.T) {
	called := false
	prev := runLoopPreflightFn
	defer func() { runLoopPreflightFn = prev }()
	runLoopPreflightFn = func(loopConfig, io.Writer) looppreflight.Result {
		called = true
		return forcedHalt()
	}

	var stderr bytes.Buffer
	if loopPreflightHalts(loopConfig{ProjectRoot: t.TempDir(), SkipPreflight: true}, &stderr) {
		t.Fatalf("--skip-preflight must bypass the gate (no halt)")
	}
	if called {
		t.Fatalf("--skip-preflight must not invoke the preflight seam")
	}
}

// End-to-end: a halting gate aborts runLoop BEFORE any cycle, with rc=2 and
// stop_reason=preflight_failed in the stdout JSON.
func TestRunLoop_PreflightHalt_AbortsBeforeCycle(t *testing.T) {
	projectRoot := t.TempDir()
	evolveDir := filepath.Join(projectRoot, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	prevDeps := wireOrchestratorDepsFn
	defer func() { wireOrchestratorDepsFn = prevDeps }()
	wireOrchestratorDepsFn = func(string, string) orchDeps {
		return orchDeps{Storage: &fixtures.FakeStorage{}, Ledger: newFakeLedger()}
	}
	prevPf := runLoopPreflightFn
	defer func() { runLoopPreflightFn = prevPf }()
	runLoopPreflightFn = func(loopConfig, io.Writer) looppreflight.Result { return forcedHalt() }

	var stdout, stderr bytes.Buffer
	rc := runLoop([]string{
		"--project-root", projectRoot,
		"--evolve-dir", evolveDir,
		"--goal-text", "anything",
		"--cycles", "1",
		"--force-fresh", // skip the unfinished-cycle guard; isolate the preflight gate
	}, nil, &stdout, &stderr)

	if rc != 2 {
		t.Fatalf("rc=%d want 2; stderr=%q stdout=%q", rc, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), `"stop_reason": "preflight_failed"`) {
		t.Fatalf("stdout should carry stop_reason=preflight_failed; got %s", stdout.String())
	}
}
