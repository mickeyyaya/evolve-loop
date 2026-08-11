package main

// cmd_acs_projectroot_test.go — RED contract for the cycle-1434 ADR-0072 halt
// (auto-filed P0): `evolve acs suite` derived EVOLVE_PROJECT_ROOT via
// mainProjectRoot (git --git-common-dir parent), which resolves to the OWNING
// repo. Correct when a cycle worktree's owner IS the plane; wrong the moment
// the plane is itself a linked worktree — all linked worktrees share one
// common dir, so the derivation skipped the plane and landed on the console
// checkout, whose .evolve has none of this cycle's run state. Three predicates
// red'd against the wrong state root while the audit phase's correct-root run
// was 8/8 green, and the CLI-written artifact won.
//
// The invocation already names the correct plane: --evolve-dir (default
// ".evolve" under the caller's cwd, which the audit persona pins to the plane
// root). suiteProjectRoot anchors on it whenever it actually holds this
// cycle's run — kernel-owned proof it is the plane — and falls back to the
// git derivation for invocations with no plane evolveDir (issue #12 shape).

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// addWorktree links a new worktree of repo at abs path dst, detached on HEAD.
func addWorktree(t *testing.T, repo, dst string) {
	t.Helper()
	cmd := exec.Command("git", "worktree", "add", "--detach", dst)
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git worktree add %s: %v\n%s", dst, err, out)
	}
}

func TestSuiteProjectRoot_NestedPlaneWorktree(t *testing.T) {
	requireGit(t)
	base := t.TempDir()
	console := filepath.Join(base, "console")
	if err := os.MkdirAll(console, 0o755); err != nil {
		t.Fatal(err)
	}
	initRepo(t, console)
	// The plane is a LINKED worktree of the console repo (the live topology:
	// evolve-loop-runtime is a worktree of evolve-loop) and holds the runtime
	// state for cycle 7.
	plane := filepath.Join(base, "plane")
	addWorktree(t, console, plane)
	planeEvolve := filepath.Join(plane, ".evolve")
	runDir := filepath.Join(planeEvolve, "runs", "cycle-7")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// The anchor is the kernel-owned cycle-state.json — presence alone, no
	// field values (an empty active_worktree must not demote the plane).
	if err := os.WriteFile(filepath.Join(runDir, "cycle-state.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The cycle worktree is a sibling linked worktree — its git common dir is
	// the CONSOLE's, not the plane's.
	cycleWT := filepath.Join(base, "cycle-wt")
	addWorktree(t, console, cycleWT)

	got := suiteProjectRoot(planeEvolve, 7, cycleWT)
	if got != plane {
		t.Errorf("suiteProjectRoot = %q, want the plane %q (the git derivation lands on the console %q — the cycle-1434 wrong-root class)",
			got, plane, console)
	}
}

func TestSuiteProjectRoot_FallbackToGitDerivation(t *testing.T) {
	requireGit(t)
	base := t.TempDir()
	console := filepath.Join(base, "console")
	if err := os.MkdirAll(console, 0o755); err != nil {
		t.Fatal(err)
	}
	initRepo(t, console)
	cycleWT := filepath.Join(base, "cycle-wt")
	addWorktree(t, console, cycleWT)

	legacy := mainProjectRoot(cycleWT)
	cases := []struct {
		name      string
		evolveDir string
	}{
		{"evolveDir absent entirely", filepath.Join(base, "no-such-dir", ".evolve")},
		{"evolveDir without this cycle's run", func() string {
			d := filepath.Join(cycleWT, ".evolve")
			if err := os.MkdirAll(d, 0o755); err != nil {
				t.Fatal(err)
			}
			return d
		}()},
		{"bare runs/cycle-N minted by `acs run` — no kernel cycle-state.json (review HIGH: an agent invoking acs run from inside the worktree must not consecrate the worktree as the plane)", func() string {
			d := filepath.Join(cycleWT, ".evolve-minted")
			if err := os.MkdirAll(filepath.Join(d, "runs", "cycle-7"), 0o755); err != nil {
				t.Fatal(err)
			}
			return d
		}()},
	}
	for _, tc := range cases {
		if got := suiteProjectRoot(tc.evolveDir, 7, cycleWT); got != legacy {
			t.Errorf("%s: suiteProjectRoot = %q, want the issue-#12 git derivation %q", tc.name, got, legacy)
		}
	}
}
