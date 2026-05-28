package main

import (
	"bytes"
	"path/filepath"
	"testing"
)

// TestParseLoopArgs_ProjectRootResolvedAbsolute encodes the cycle-119
// ExitArtifactTimeout (exit 81) root cause.
//
// --project-root defaults to "." and was never absolutized, so WorkspacePath
// (= <root>/.evolve/runs/cycle-N) and the artifact path derived from it were
// RELATIVE. Worktree phases (tdd/build) run the agent with cwd=worktree, so the
// agent resolved a relative artifact path INTO the worktree subtree while the
// in-process bridge polled the same relative path against the main-repo cwd.
// The two cwds diverged, the artifact "never appeared" where the bridge looked,
// and the driver returned ExitArtifactTimeout — aborting the cycle at tdd.
//
// The flag's own help text promises "absolute path to project root"; this test
// asserts that documented invariant is actually enforced for every input shape,
// so the workspace/artifact path is cwd-independent for worktree-phase agents.
func TestParseLoopArgs_ProjectRootResolvedAbsolute(t *testing.T) {
	abs := func(p string) string { a, _ := filepath.Abs(p); return a }
	cases := []struct {
		name     string
		args     []string
		wantRoot string
	}{
		{"default_dot_is_absolutized", []string{"--goal-text", "x"}, abs(".")},
		{"explicit_relative_is_absolutized", []string{"--project-root", "sub/dir", "--goal-text", "x"}, abs("sub/dir")},
		{"already_absolute_is_idempotent", []string{"--project-root", "/tmp/evolve-x", "--goal-text", "x"}, "/tmp/evolve-x"},
		// --evolve-dir is a separately-passed path with the same cwd-independence
		// requirement (many consumers join cfg.EvolveDir). A relative value must
		// also be absolutized, independent of the resolved project root.
		{"explicit_relative_evolve_dir_is_absolutized", []string{"--project-root", "/tmp/evolve-x", "--evolve-dir", "rel/.evolve", "--goal-text", "x"}, "/tmp/evolve-x"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stderr bytes.Buffer
			cfg, rc := parseLoopArgs(tc.args, &stderr)
			if rc != 0 {
				t.Fatalf("rc=%d, want 0 (stderr=%q)", rc, stderr.String())
			}
			if !filepath.IsAbs(cfg.ProjectRoot) {
				t.Errorf("ProjectRoot=%q is not absolute", cfg.ProjectRoot)
			}
			if cfg.ProjectRoot != tc.wantRoot {
				t.Errorf("ProjectRoot=%q, want %q", cfg.ProjectRoot, tc.wantRoot)
			}
			// The bug was specifically that paths derived from the root stayed
			// relative; EvolveDir must be absolute too so worktree-phase artifact
			// paths resolve identically across the agent/bridge cwd boundary.
			if !filepath.IsAbs(cfg.EvolveDir) {
				t.Errorf("EvolveDir=%q is not absolute (cycle-119 ExitArtifactTimeout class)", cfg.EvolveDir)
			}
		})
	}
}
