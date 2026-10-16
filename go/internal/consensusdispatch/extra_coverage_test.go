package consensusdispatch

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/capability"
)

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
		WorkspacePath: filepath.Join(blocker, "ws"),
		ProfilePath:   prof,
		PromptFile:    prompt,
	}, &stdout, &stderr)
	if rc != ExitRuntimeErr {
		t.Errorf("workspace mkdir failure should rc=%d, got %d (stderr=%s)", ExitRuntimeErr, rc, stderr.String())
	}
}

func TestRun_QuorumReducedThenNoDrivers(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	prof := filepath.Join(dir, "prof.json")
	prompt := filepath.Join(dir, "prompt.md")
	writeProfile(t, prof, map[string]any{
		"enabled":          true,
		"cli_voters":       []string{"no-driver-a", "no-driver-b"},
		"quorum":           5,
		"require_min_tier": "none",
	})
	writeFile(t, prompt, "audit")
	var stdout, stderr bytes.Buffer
	rc := Run(Inputs{
		Cycle: "1", WorkspacePath: dir, ProfilePath: prof, PromptFile: prompt,
		AdaptersDir: filepath.Join(dir, "empty-adapters"),
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

func TestFilterEligible_MissingManifestExcludedUnderHybrid(t *testing.T) {
	t.Parallel()
	emptyAdapters := t.TempDir()
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

func TestResolveEvolveBin_EnvOverride(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "evolve")
	writeExec(t, bin, "#!/bin/sh\nexit 0\n")
	t.Setenv("EVOLVE_GO_BIN", bin)
	if got := resolveEvolveBin(dir); got != bin {
		t.Errorf("env override: got %q, want %q", got, bin)
	}
}

func TestResolveEvolveBin_WalkFindsGoBin(t *testing.T) {
	dir := t.TempDir()
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

func TestResolveEvolveBin_NotFound(t *testing.T) {
	t.Setenv("EVOLVE_GO_BIN", "")
	t.Setenv("PATH", "")
	if got := resolveEvolveBin(t.TempDir()); got != "" {
		t.Errorf("not found: got %q, want empty", got)
	}
}

func TestResolveNativeDispatch_Native(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "evolve")
	writeExec(t, bin, "#!/bin/sh\nexit 0\n")
	t.Setenv("EVOLVE_GO_BIN", bin)
	cmd, err := resolveNativeDispatch(dir, "fanout-dispatch", []string{"a.tsv", "b.tsv"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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

func TestResolveNativeDispatch_RequiresNativeBin(t *testing.T) {
	t.Setenv("EVOLVE_GO_BIN", "")
	t.Setenv("PATH", "")
	if _, err := resolveNativeDispatch(t.TempDir(), "aggregator", []string{"out.md"}); err == nil {
		t.Error("expected error when no native evolve binary is available (bash fallback removed in v12)")
	}
}

func TestExitCodeFromErr_NilState(t *testing.T) {
	t.Parallel()
	if got := exitCodeFromErr(nil); got != 1 {
		t.Errorf("nil state: got %d, want 1", got)
	}
}
