package main

import (
	"os"
	"path/filepath"
	"testing"
)

// writeCycleState ignores its cycle argument; writeCycleStateN honors it.
func writeCycleState(t *testing.T, evolveDir string, cycle int, body string) {
	t.Helper()
	dir := filepath.Join(evolveDir, "runs", "cycle-230")
	_ = cycle
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cycle-state.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write cycle-state.json: %v", err)
	}
}

func TestACSSuiteRootAutosolve(t *testing.T) {
	evolveDir := t.TempDir()
	want := "/tmp/evolve-worktrees/cycle-230"
	writeCycleState(t, evolveDir, 230, `{"active_worktree":"`+want+`","workspace_path":"/tmp/ws","intent_required":false}`)

	got := resolveACSSuiteRoot(evolveDir, 230)
	if got != want {
		t.Errorf("resolveACSSuiteRoot = %q, want %q (active_worktree from cycle-state.json)", got, want)
	}
}

func TestACSSuiteRootFallback(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, evolveDir string)
	}{
		{"cycle-state.json absent", func(t *testing.T, evolveDir string) {}},
		{"active_worktree empty", func(t *testing.T, evolveDir string) {
			writeCycleState(t, evolveDir, 230, `{"active_worktree":"","workspace_path":"/tmp/ws","intent_required":false}`)
		}},
		{"active_worktree key missing", func(t *testing.T, evolveDir string) {
			writeCycleState(t, evolveDir, 230, `{"workspace_path":"/tmp/ws","intent_required":false}`)
		}},
		{"malformed JSON", func(t *testing.T, evolveDir string) {
			writeCycleState(t, evolveDir, 230, `{"active_worktree": not-json`)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			evolveDir := t.TempDir()
			tc.setup(t, evolveDir)
			if got := resolveACSSuiteRoot(evolveDir, 230); got != "" {
				t.Errorf("resolveACSSuiteRoot = %q, want \"\" (graceful fallback)", got)
			}
		})
	}
}
