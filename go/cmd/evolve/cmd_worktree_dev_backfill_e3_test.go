package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func e3DevCreate(t *testing.T, runtime, task, branch string) (string, int) {
	t.Helper()
	return runDev(t, "create", "--dev", task, "--branch", branch, "--project-root", runtime)
}

func TestRunWorktreeCreateDev_RefusalsAndGitFailures(t *testing.T) {
	cases := []struct {
		name     string
		branch   string
		setup    func(t *testing.T, hub, store string)
		wantCode int
		want     string
		branchOK bool
	}{
		{"invalid branch name", "bad..name", func(*testing.T, string, string) {}, 1, `refused: "bad..name" is not a valid branch name`, false},
		{"branch already exists", "taken", func(t *testing.T, _, store string) {
			brGit(t, store, "branch", "taken", "main")
		}, 1, "refused: branch taken already exists", true},
		{"dev path is not a directory", "b1", func(t *testing.T, hub, _ string) {
			if err := os.WriteFile(filepath.Join(hub, "dev"), []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
		}, 2, "evolve worktree create --dev: stat ", false},
		{"origin cannot be fetched", "b1", func(t *testing.T, _, store string) {
			brGit(t, store, "remote", "remove", "origin")
		}, 2, "evolve worktree create --dev: git fetch origin main: rc=", false},
		{"the dev dir cannot be created", "b1", func(t *testing.T, hub, _ string) {
			dev := filepath.Join(hub, "dev")
			if err := os.Mkdir(dev, 0o555); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(dev, 0o755) })
		}, 2, "evolve worktree create --dev: git worktree add rc=", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runtime, _ := devHubFixture(t)
			hub := filepath.Dir(runtime)
			store := filepath.Join(hub, ".repo.git")
			tc.setup(t, hub, store)

			out, code := e3DevCreate(t, runtime, "t1", tc.branch)

			if code != tc.wantCode || !strings.Contains(out, tc.want) {
				t.Fatalf("code %d out %q, want %d with %q", code, out, tc.wantCode, tc.want)
			}
			if !tc.branchOK && brBranchExists(t, store, tc.branch) {
				t.Errorf("a failed create left branch %s behind", tc.branch)
			}
		})
	}
}
