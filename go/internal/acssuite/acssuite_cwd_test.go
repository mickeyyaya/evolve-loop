package acssuite

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestRun_PredicateExecutesWithCwdAtRoot — cycle-190 regression. Predicates are
// DISCOVERED from opts.Root (the worktree), but were EXECUTED with the caller's
// cwd (the main repo) because runBash never set cmd.Dir. A predicate's `go test`
// therefore compiled MAIN's source, not the worktree's — so a cycle's new code
// (present only in the worktree) was invisible, the predicate went RED, EGPS
// blocked ship, and PASS-audited work was discarded.
//
// This pins the contract: a predicate must run with cwd == opts.Root, so a
// relative path check resolves against the tree being shipped. The predicate
// here passes IFF a sentinel that exists ONLY in the worktree root is visible
// from its cwd.
func TestRun_PredicateExecutesWithCwdAtRoot(t *testing.T) {
	worktree := t.TempDir()
	// Sentinel lives in the worktree root only — never in the test process cwd
	// (the package dir). A predicate run with cwd=worktree sees it; one run with
	// the inherited cwd does not.
	if err := os.WriteFile(filepath.Join(worktree, "worktree-sentinel"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	predDir := filepath.Join(worktree, "acs", "cycle-1")
	if err := os.MkdirAll(predDir, 0o755); err != nil {
		t.Fatal(err)
	}
	pred := "#!/usr/bin/env bash\n[ -f worktree-sentinel ] && exit 0 || exit 1\n"
	if err := os.WriteFile(filepath.Join(predDir, "001-cwd.sh"), []byte(pred), 0o755); err != nil {
		t.Fatal(err)
	}

	v, err := Run(Options{Root: worktree, Cycle: 1})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(v.Results) != 1 {
		t.Fatalf("want 1 predicate result, got %d", len(v.Results))
	}
	if v.Results[0].ResultStr != "green" {
		t.Errorf("predicate ran with cwd not at opts.Root: result=%q exit=%d excerpt=%q — cycle-190 tree-visibility bug",
			v.Results[0].ResultStr, v.Results[0].ExitCode, v.Results[0].EvidenceExcerpt)
	}
}

// TestResolveTimeout — cycle-200: a full-suite predicate can exceed the 60s
// default and flake to a false RED (exit 124). EVOLVE_ACS_PREDICATE_TIMEOUT_S
// must raise the per-predicate timeout; opts.Timeout still wins; bad/unset env
// falls back to the default.
func TestResolveTimeout(t *testing.T) {
	const def = DefaultTimeout
	cases := []struct {
		name string
		opts time.Duration
		env  string
		want time.Duration
	}{
		{"opts wins over env", 90 * time.Second, "300", 90 * time.Second},
		{"env override when opts unset", 0, "300", 300 * time.Second},
		{"unset env → default", 0, "", def},
		{"invalid env → default", 0, "abc", def},
		{"zero env → default", 0, "0", def},
		{"negative env → default", 0, "-5", def},
	}
	for _, c := range cases {
		got := resolveTimeout(c.opts, func(string) string { return c.env })
		if got != c.want {
			t.Errorf("%s: resolveTimeout(%v, env=%q)=%v, want %v", c.name, c.opts, c.env, got, c.want)
		}
	}
}
