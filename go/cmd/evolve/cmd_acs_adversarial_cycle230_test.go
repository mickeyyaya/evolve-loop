package main

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// Cycle-230 test-amplification adversarial tests for task acs-suite-root-autosolve.
// Written from spec only (no implementation read) — anti-bias isolation.
//
// Coverage gaps addressed:
//   - JSON null and type-mismatch for active_worktree: graceful fallback to ""
//   - Path values with spaces/special chars: resolver must return verbatim
//   - Wrong cycle number passed: resolver constructs path from cycle param; if
//     the state file for the requested cycle doesn't exist, it must return ""

// writeCycleStateN writes a cycle-state.json for an arbitrary cycle number.
// Unlike writeCycleState (TDD helper hardcoded to cycle-230), this respects the
// cycle parameter — needed to test that the resolver uses the cycle param correctly.
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

// TestACSSuiteRootAutosolve_NullAndTypeMismatch_Amp: JSON null and type-mismatch
// values for active_worktree must all produce "" (graceful fallback), not panic
// or return a non-empty string.
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

// TestACSSuiteRootAutosolve_PathPreservation_Amp: when active_worktree contains
// a valid path string (including spaces or multiple path segments), the resolver
// must return it verbatim without normalization or truncation.
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

// TestACSSuiteRootAutosolve_WrongCycle_Amp: the resolver constructs its file path
// from the cycle parameter. When cycle=230 is requested but only cycle=229's
// state file exists, the resolver must return "" (file-not-found fallback), NOT
// bleed the cycle-229 active_worktree value.
func TestACSSuiteRootAutosolve_WrongCycle_Amp(t *testing.T) {
	evolveDir := t.TempDir()
	// Write state only for cycle 229 — cycle 230 has no file
	writeCycleStateN(t, evolveDir, 229, `{"active_worktree":"/tmp/worktrees/cycle-229"}`)

	got := resolveACSSuiteRoot(evolveDir, 230)
	if got != "" {
		t.Errorf("resolveACSSuiteRoot(cycle=230) = %q, want \"\" (cycle-230 state file absent; must not return cycle-229 result)", got)
	}
}
