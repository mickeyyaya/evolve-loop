package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

func TestRunnerMissingProfile_NoProfilesDir(t *testing.T) {
	t.Parallel()
	root := t.TempDir() // no .evolve/profiles inside
	hooks := &fakeHooks{phase: "build", agent: "evolve-builder", model: "auto", verdict: core.VerdictPASS}
	fb := &fakeBridge{writeArtifact: "# build artifact\n## Files Modified\n- a.go\n"}
	r := New(Options{
		Hooks:   hooks,
		Bridge:  fb,
		Prompts: fakePromptsFS("evolve-builder", "x"),
		NowFn:   fixtures.FixedClock(time.Unix(1_700_000_000, 0), 0),
	})

	_, err := r.Run(context.Background(), core.PhaseRequest{
		ProjectRoot: root,
		Workspace:   t.TempDir(),
	})
	if err != nil {
		t.Fatalf("runner must not fast-fail when profiles dir is absent; got %v", err)
	}
}

func TestRunnerMissingProfile_ProfilePresent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	profileDir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		t.Fatal(err)
	}
	profileContent := `{"name":"builder","role":"builder","cli":"claude-tmux"}`
	if err := os.WriteFile(filepath.Join(profileDir, "builder.json"), []byte(profileContent), 0o644); err != nil {
		t.Fatal(err)
	}

	hooks := &fakeHooks{phase: "build", agent: "evolve-builder", model: "auto", verdict: core.VerdictPASS}
	fb := &fakeBridge{writeArtifact: "# build artifact\n## Files Modified\n- a.go\n"}
	r := New(Options{
		Hooks:   hooks,
		Bridge:  fb,
		Prompts: fakePromptsFS("evolve-builder", "x"),
		NowFn:   fixtures.FixedClock(time.Unix(1_700_000_000, 0), 0),
	})

	_, err := r.Run(context.Background(), core.PhaseRequest{
		ProjectRoot: root,
		Workspace:   t.TempDir(),
	})
	if err != nil {
		t.Fatalf("runner must not fast-fail when profile is present; got %v", err)
	}
}

func TestRunnerMissingProfile_AgentNameStripping(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	profileDir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Write "scout.json" (not "evolve-scout.json") — proves the prefix strip.
	profileContent := `{"name":"scout","role":"scout","cli":"claude-tmux"}`
	if err := os.WriteFile(filepath.Join(profileDir, "scout.json"), []byte(profileContent), 0o644); err != nil {
		t.Fatal(err)
	}

	hooks := &fakeHooks{phase: "scout", agent: "evolve-scout", model: "sonnet", verdict: core.VerdictPASS}
	fb := &fakeBridge{writeArtifact: "## Proposed Tasks\n1. x\n"}
	r := New(Options{
		Hooks:   hooks,
		Bridge:  fb,
		Prompts: fakePromptsFS("evolve-scout", "x"),
		NowFn:   fixtures.FixedClock(time.Unix(1_700_000_000, 0), 0),
	})

	_, err := r.Run(context.Background(), core.PhaseRequest{
		Cycle:       1,
		ProjectRoot: root,
		Workspace:   t.TempDir(),
	})
	if err != nil {
		t.Fatalf("runner must find scout.json (stripped from evolve-scout); got %v", err)
	}
}

func TestRunnerMissingProfile_ProfilesDirIsFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "profiles"), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}

	hooks := &fakeHooks{phase: "build", agent: "evolve-builder", model: "auto", verdict: core.VerdictPASS}
	fb := &fakeBridge{writeArtifact: "# build artifact\n## Files Modified\n- a.go\n"}
	r := New(Options{
		Hooks:   hooks,
		Bridge:  fb,
		Prompts: fakePromptsFS("evolve-builder", "x"),
		NowFn:   fixtures.FixedClock(time.Unix(1_700_000_000, 0), 0),
	})

	_, err := r.Run(context.Background(), core.PhaseRequest{
		ProjectRoot: root,
		Workspace:   t.TempDir(),
	})
	if err != nil {
		t.Fatalf("profiles path is a FILE not a dir; runner must not fast-fail; got %v", err)
	}
}
