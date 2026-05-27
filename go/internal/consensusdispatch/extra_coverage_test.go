package consensusdispatch

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

// TestRun_QuorumReducedThenNoAdapters covers the quorum-reduction WARN block
// (eligible < declared quorum) followed by the post-build "workers ready < 2"
// branch: with no capability-check all voters are eligible, quorum=5 forces the
// reduction, then the empty adapters dir yields zero ready workers.
func TestRun_QuorumReducedThenNoAdapters(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	prof := filepath.Join(dir, "prof.json")
	prompt := filepath.Join(dir, "prompt.md")
	writeProfile(t, prof, map[string]any{
		"enabled":          true,
		"cli_voters":       []string{"claude", "gemini"},
		"quorum":           5, // > eligible(2) → reduction WARN
		"require_min_tier": "none",
	})
	writeFile(t, prompt, "audit")
	var stdout, stderr bytes.Buffer
	rc := Run(Inputs{
		Cycle: "1", WorkspacePath: dir, ProfilePath: prof, PromptFile: prompt,
		AdaptersDir: filepath.Join(dir, "empty-adapters"), // no capability-check, no adapters
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

// TestProbeQualityTier covers all four return paths via fake capability-check
// shims: success, non-zero exit, bad JSON, and empty tier field.
func TestProbeQualityTier(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cases := []struct {
		name string
		body string
		want string
	}{
		{"success", "#!/bin/sh\necho '{\"quality_tier\":\"full\"}'\n", "full"},
		{"exit-error", "#!/bin/sh\nexit 3\n", "unknown"},
		{"bad-json", "#!/bin/sh\necho 'not json'\n", "unknown"},
		{"empty-tier", "#!/bin/sh\necho '{\"quality_tier\":\"\"}'\n", "unknown"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cap := filepath.Join(dir, tc.name+".sh")
			writeExec(t, cap, tc.body)
			if got := probeQualityTier(cap, "claude"); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestExitCodeFromErr_NilState covers the nil-ProcessState branch.
func TestExitCodeFromErr_NilState(t *testing.T) {
	t.Parallel()
	if got := exitCodeFromErr(nil); got != 1 {
		t.Errorf("nil state: got %d, want 1", got)
	}
}
