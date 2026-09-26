package main

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// writeCycleStateN honors its cycle argument, unlike writeCycleState, so a test
// can prove the resolver reads the requested cycle's file.
func writeCycleStateN(t *testing.T, evolveDir string, cycle int, body string) {
	t.Helper()
	dir := filepath.Join(evolveDir, "runs", "cycle-"+strconv.Itoa(cycle))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cycle-state.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write cycle-state.json: %v", err)
	}
}

func TestACSSuiteRootAutosolve_NullAndTypeMismatch_Amp(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"null value", `{"active_worktree": null, "workspace_path": "/tmp/ws"}`},
		{"number value", `{"active_worktree": 42}`},
		{"array value", `{"active_worktree": []}`},
		{"object value", `{"active_worktree": {"nested": "value"}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			evolveDir := t.TempDir()
			writeCycleStateN(t, evolveDir, 230, tc.body)
			if got := resolveACSSuiteRoot(evolveDir, 230); got != "" {
				t.Errorf("resolveACSSuiteRoot with %s = %q, want \"\" (graceful fallback)", tc.name, got)
			}
		})
	}
}

func TestACSSuiteRootAutosolve_PathPreservation_Amp(t *testing.T) {
	cases := []struct {
		name string
		path string
	}{
		{"path with spaces", "/tmp/evolve worktrees/cycle-230"},
		{"absolute deep path", "/Users/user/.evolve/worktrees/cycle-230"},
		{"many segments", "/a/b/c/d/e/f/g/cycle-230"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			evolveDir := t.TempDir()
			writeCycleStateN(t, evolveDir, 230, `{"active_worktree":"`+tc.path+`"}`)
			got := resolveACSSuiteRoot(evolveDir, 230)
			if got != tc.path {
				t.Errorf("resolveACSSuiteRoot = %q, want %q (path must be returned verbatim)", got, tc.path)
			}
		})
	}
}

func TestACSSuiteRootAutosolve_WrongCycle_Amp(t *testing.T) {
	evolveDir := t.TempDir()
	writeCycleStateN(t, evolveDir, 229, `{"active_worktree":"/tmp/worktrees/cycle-229"}`)

	got := resolveACSSuiteRoot(evolveDir, 230)
	if got != "" {
		t.Errorf("resolveACSSuiteRoot(cycle=230) = %q, want \"\" (cycle-230 state file absent; must not return cycle-229 result)", got)
	}
}
