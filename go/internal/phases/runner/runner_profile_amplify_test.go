package runner

// runner_profile_amplify_test.go — adversarial profile-loading boundary tests
// for cycle-276 T3 (bridge-profile-contract-symmetry). The builder's
// runner_test.go proves that profiles-dir-present + profile-absent → fast-fail
// with the path in the error message. These tests probe the complementary
// boundaries: absent profiles dir → existing behavior preserved (no fast-fail),
// and profile-present → no fast-fail.

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
	"github.com/mickeyyaya/evolveloop/go/test/fixtures"
)

// TestRunnerMissingProfile_NoProfilesDir verifies the critical backward-compat
// boundary: when the profiles directory does NOT exist, the runner must proceed
// normally (the existing behavior that all pre-276 tests rely on).
// The fast-fail is gated on `os.Stat(profileDir).IsDir()` — absent dir must
// not trigger the guard.
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

// TestRunnerMissingProfile_ProfilePresent verifies that when the profiles dir
// exists AND the expected profile file is present, the runner proceeds without
// error (fast-fail must not fire on a healthy configuration).
func TestRunnerMissingProfile_ProfilePresent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	profileDir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// agent "evolve-builder" → profile name "builder" → "builder.json"
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

// TestRunnerMissingProfile_AgentNameStripping verifies that the profile
// filename uses the agent name with the "evolve-" prefix stripped:
//   - agent "evolve-scout"  → looks for "scout.json"
//   - agent "evolve-tdd-engineer" → looks for "tdd-engineer.json"
//
// If the runner accidentally looked for "evolve-scout.json" it would find
// nothing (a silent naming mismatch that the builder's tests don't probe).
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

// TestRunnerMissingProfile_ProfilesDirIsFile verifies that a path collision
// where .evolve/profiles is a FILE (not a directory) does not trigger the
// fast-fail. os.Stat(profileDir).IsDir() must return false for a regular file,
// so the runner proceeds as if the directory is absent.
func TestRunnerMissingProfile_ProfilesDirIsFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Write a FILE at the profiles path (not a directory).
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
