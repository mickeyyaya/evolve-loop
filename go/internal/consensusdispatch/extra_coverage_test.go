package consensusdispatch

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/capability"
)

// TestRun_WorkspaceMkdirFails covers Run's workspace-create error branch: the
// workspace path lives under a regular file, so MkdirAll fails after the input
// validation passes.
func TestRun_WorkspaceMkdirFails(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	prof := filepath.Join(dir, "prof.json")
	prompt := filepath.Join(dir, "prompt.md")
	writeProfile(t, prof, map[string]any{"enabled": true, "cli_voters": []string{"claude", "gemini"}, "quorum": 2})
	writeFile(t, prompt, "audit")
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	rc := Run(Inputs{
		Cycle:         "1",
		WorkspacePath: filepath.Join(blocker, "ws"), // under a file → mkdir fails
		ProfilePath:   prof,
		PromptFile:    prompt,
	}, &stdout, &stderr)
	if rc != ExitRuntimeErr {
		t.Errorf("workspace mkdir failure should rc=%d, got %d (stderr=%s)", ExitRuntimeErr, rc, stderr.String())
	}
}

// TestRun_QuorumReducedThenNoDrivers covers the quorum-reduction WARN block
// (eligible < declared quorum) followed by the post-build "workers ready < 2"
// branch. Post-bridge cutover a worker is ready only if its CLI projects onto a
// REGISTERED bridge driver, so this uses two driverless voter names:
// require_min_tier=none admits both regardless of tier, quorum=5 forces the
// reduction WARN, then BuildCommandsTSV skips both (no driver) → zero ready workers.
func TestRun_QuorumReducedThenNoDrivers(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	prof := filepath.Join(dir, "prof.json")
	prompt := filepath.Join(dir, "prompt.md")
	writeProfile(t, prof, map[string]any{
		"enabled":          true,
		"cli_voters":       []string{"no-driver-a", "no-driver-b"},
		"quorum":           5, // > eligible(2) → reduction WARN
		"require_min_tier": "none",
	})
	writeFile(t, prompt, "audit")
	var stdout, stderr bytes.Buffer
	rc := Run(Inputs{
		Cycle: "1", WorkspacePath: dir, ProfilePath: prof, PromptFile: prompt,
		AdaptersDir: filepath.Join(dir, "empty-adapters"), // no capability manifests → unknown tier
		DispatchDir: filepath.Join(dir, "fake-dispatch"),
	}, &stdout, &stderr)
	if rc != ExitRuntimeErr {
		t.Errorf("no-adapters should rc=%d, got %d (stderr=%s)", ExitRuntimeErr, rc, stderr.String())
	}
	if !strings.Contains(stderr.String(), "reducing quorum") {
		t.Errorf("missing quorum-reduction WARN: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "workers ready") {
		t.Errorf("missing post-build workers-ready FAIL: %s", stderr.String())
	}
}

// TestFilterEligible_MissingManifestExcludedUnderHybrid pins the deliberate
// post-migration behavior (see filterEligible doc): a voter whose
// <cli>.capabilities.json is absent resolves to "unknown" via the default
// tierFor (capability.QualityTier) and is EXCLUDED when require_min_tier is
// hybrid or above. The old shell path's "checker script absent → include all"
// global bail-out is intentionally gone (the checker is compiled in now).
func TestFilterEligible_MissingManifestExcludedUnderHybrid(t *testing.T) {
	t.Parallel()
	emptyAdapters := t.TempDir() // no *.capabilities.json manifests
	tierFor := func(cli string) (string, error) {
		return capability.QualityTier(emptyAdapters, cli, nil)
	}
	var stderr bytes.Buffer
	elig, declared := filterEligible([]string{"claude", "gemini"}, "hybrid", tierFor, &stderr)
	if declared != 2 {
		t.Errorf("declared=%d, want 2", declared)
	}
	if len(elig) != 0 {
		t.Errorf("eligible=%v, want none (missing manifests → unknown → excluded under hybrid)", elig)
	}
	if !strings.Contains(stderr.String(), "tier=unknown, require>=hybrid") {
		t.Errorf("missing exclusion log line: %s", stderr.String())
	}
}

// TestResolveEvolveBin_EnvOverride covers the EVOLVE_GO_BIN branch.
func TestResolveEvolveBin_EnvOverride(t *testing.T) {
	// Serial (no t.Parallel): t.Setenv mutates process-global EVOLVE_GO_BIN/PATH.
	dir := t.TempDir()
	bin := filepath.Join(dir, "evolve")
	writeExec(t, bin, "#!/bin/sh\nexit 0\n")
	t.Setenv("EVOLVE_GO_BIN", bin)
	if got := resolveEvolveBin(dir); got != bin {
		t.Errorf("env override: got %q, want %q", got, bin)
	}
}

// TestResolveEvolveBin_EnvNonExecutableFallsThrough covers the env-set but
// non-executable case (Stat ok, mode&0111==0 → skip) followed by the walk.
func TestResolveEvolveBin_WalkFindsGoBin(t *testing.T) {
	// Serial (no t.Parallel): t.Setenv mutates process-global EVOLVE_GO_BIN/PATH.
	dir := t.TempDir()
	// Non-executable env target → env branch skipped.
	notExec := filepath.Join(dir, "not-exec")
	if err := os.WriteFile(notExec, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVOLVE_GO_BIN", notExec)
	gobin := filepath.Join(dir, "go", "bin", "evolve")
	writeExec(t, gobin, "#!/bin/sh\nexit 0\n")
	if got := resolveEvolveBin(dir); got != gobin {
		t.Errorf("walk: got %q, want %q", got, gobin)
	}
}

// TestResolveEvolveBin_NotFound covers the exhausted-walk → "" path. PATH is
// cleared so LookPath cannot find a real `evolve`.
func TestResolveEvolveBin_NotFound(t *testing.T) {
	// Serial (no t.Parallel): t.Setenv mutates process-global EVOLVE_GO_BIN/PATH.
	t.Setenv("EVOLVE_GO_BIN", "")
	t.Setenv("PATH", "")
	if got := resolveEvolveBin(t.TempDir()); got != "" {
		t.Errorf("not found: got %q, want empty", got)
	}
}

// TestResolveBashOrNative_Native covers the native-binary branch: a resolvable
// EVOLVE_GO_BIN yields `<bin> <subcmd> <args...>`.
func TestResolveBashOrNative_Native(t *testing.T) {
	// Serial (no t.Parallel): t.Setenv mutates process-global EVOLVE_GO_BIN/PATH.
	dir := t.TempDir()
	bin := filepath.Join(dir, "evolve")
	writeExec(t, bin, "#!/bin/sh\nexit 0\n")
	t.Setenv("EVOLVE_GO_BIN", bin)
	cmd := resolveBashOrNative(dir, "fanout-dispatch", []string{"a.tsv", "b.tsv"})
	want := []string{bin, "fanout-dispatch", "a.tsv", "b.tsv"}
	if len(cmd.Args) != len(want) {
		t.Fatalf("args = %v, want %v", cmd.Args, want)
	}
	for i := range want {
		if cmd.Args[i] != want[i] {
			t.Errorf("arg %d = %q, want %q", i, cmd.Args[i], want[i])
		}
	}
}

// TestResolveBashOrNative_BashFallback covers the legacy bash-script branch
// when no native binary resolves.
func TestResolveBashOrNative_BashFallback(t *testing.T) {
	// Serial (no t.Parallel): t.Setenv mutates process-global EVOLVE_GO_BIN/PATH.
	t.Setenv("EVOLVE_GO_BIN", "")
	t.Setenv("PATH", "")
	dir := t.TempDir()
	cmd := resolveBashOrNative(dir, "aggregator", []string{"out.md"})
	if cmd.Args[0] != "bash" {
		t.Errorf("expected bash fallback, got %v", cmd.Args)
	}
	if cmd.Args[1] != filepath.Join(dir, "aggregator.sh") {
		t.Errorf("script path = %q", cmd.Args[1])
	}
}

// TestExitCodeFromErr_NilState covers the nil-ProcessState branch.
func TestExitCodeFromErr_NilState(t *testing.T) {
	t.Parallel()
	if got := exitCodeFromErr(nil); got != 1 {
		t.Errorf("nil state: got %d, want 1", got)
	}
}
