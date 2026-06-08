package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Cycle-230 task acs-suite-root-autosolve (mode 5 of
// user-phase-persona-resolution): `evolve acs suite` must resolve its suite
// root from the kernel-owned cycle state instead of trusting the caller's cwd,
// eliminating the LLM-topology nondeterminism that false-FAILed cycles 226-227.
//
// Contract under test: resolveACSSuiteRoot(evolveDir string, cycle int) string
//   - reads <evolveDir>/runs/cycle-<N>/cycle-state.json
//   - returns its non-empty "active_worktree" value
//   - returns "" when the file is absent, malformed, or the field is empty
//     (caller then falls back to the --root flag default)
//
// NOTE (TDD): the cycle param is int (not string as sketched in
// scout-report.md Build Plan §3) — it must match the existing --cycle
// flag.IntVar in runACSSuite; a string param would force lossy round-trips.
//
// DO NOT MODIFY (builder contract): implement resolveACSSuiteRoot in
// cmd_acs.go and wire it into runACSSuite when --root is not explicitly set.

// writeCycleState writes a cycle-state.json fixture under a temp evolve dir.
func writeCycleState(t *testing.T, evolveDir string, cycle int, body string) {
	t.Helper()
	dir := filepath.Join(evolveDir, "runs", "cycle-230")
	_ = cycle // path is fixed to cycle-230 fixtures in these tests
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cycle-state.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write cycle-state.json: %v", err)
	}
}

// TestACSSuiteRootAutosolve: happy path — cycle-state.json carries a non-empty
// active_worktree; the resolver must return it verbatim.
func TestACSSuiteRootAutosolve(t *testing.T) {
	evolveDir := t.TempDir()
	want := "/tmp/evolve-worktrees/cycle-230"
	writeCycleState(t, evolveDir, 230, `{"active_worktree":"`+want+`","workspace_path":"/tmp/ws","intent_required":false}`)

	got := resolveACSSuiteRoot(evolveDir, 230)
	if got != want {
		t.Errorf("resolveACSSuiteRoot = %q, want %q (active_worktree from cycle-state.json)", got, want)
	}
}

// TestACSSuiteRootFallback: every degraded input must yield "" so runACSSuite
// falls back to the --root flag default instead of guessing.
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
