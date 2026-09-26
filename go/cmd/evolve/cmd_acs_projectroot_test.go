package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

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
	// The plane is itself a linked worktree of the console, as in the live hub.
	plane := filepath.Join(base, "plane")
	addWorktree(t, console, plane)
	planeEvolve := filepath.Join(plane, ".evolve")
	runDir := filepath.Join(planeEvolve, "runs", "cycle-7")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// An empty state file: presence alone must prove the plane.
	if err := os.WriteFile(filepath.Join(runDir, "cycle-state.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
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
