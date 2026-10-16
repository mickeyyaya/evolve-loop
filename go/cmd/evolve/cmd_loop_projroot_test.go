package main

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestParseLoopArgs_ProjectRootResolvedAbsolute(t *testing.T) {
	t.Parallel()
	abs := func(p string) string { a, _ := filepath.Abs(p); return a }
	cases := []struct {
		name     string
		args     []string
		wantRoot string
	}{
		{"default_dot_is_absolutized", []string{"--goal-text", "x"}, abs(".")},
		{"explicit_relative_is_absolutized", []string{"--project-root", "sub/dir", "--goal-text", "x"}, abs("sub/dir")},
		{"already_absolute_is_idempotent", []string{"--project-root", "/tmp/evolve-x", "--goal-text", "x"}, "/tmp/evolve-x"},
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
			if !filepath.IsAbs(cfg.EvolveDir) {
				t.Errorf("EvolveDir=%q is not absolute (cycle-119 ExitArtifactTimeout class)", cfg.EvolveDir)
			}
		})
	}
}
