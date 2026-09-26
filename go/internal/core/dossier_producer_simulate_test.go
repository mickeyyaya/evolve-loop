package core

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestWriteCycleDossier_CommitFalse_WritesFilesOnly(t *testing.T) {
	root := t.TempDir() // deliberately not a git repository
	ws := t.TempDir()
	params := cycleDossierParams{ProjectRoot: root, WorkspacePath: ws, Cycle: 3, Goal: "simulate", RunID: "run-sim", Outcome: CycleOutcomeShippedViaBuild}
	params.FilesOnly = true
	if err := writeCycleDossier(nil, params); err != nil {
		t.Fatalf("files-only dossier on a non-git root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "knowledge-base", "cycles", "cycle-3.json")); err != nil {
		t.Errorf("the dossier file is still written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
		t.Error("a files-only dossier must not create a repository")
	}
	params.Cycle, params.FilesOnly = 4, false
	if err := writeCycleDossier(nil, params); err == nil {
		t.Error("the committing default on a non-git root cannot succeed — the two modes are distinguishable")
	}
}

func gitCommitCount(t *testing.T, root string) int {
	t.Helper()
	out, err := exec.Command("git", "-C", root, "rev-list", "--all", "--count").Output()
	if err != nil {
		t.Fatalf("rev-list: %v", err)
	}
	n, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	return n
}

func TestWithDossierCommit_IsTheRootsKnob(t *testing.T) {
	for _, tc := range []struct {
		name    string
		opts    []Option
		commits int
	}{
		{"default commits", nil, 1},
		{"simulate root writes only", []Option{WithDossierCommit(false)}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			initDossierRepo(t, root)
			before := gitCommitCount(t, root)
			opts := append([]Option{WithWorktreeProvisioner(&fakeWorktree{path: t.TempDir()})}, tc.opts...)
			o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), opts...)
			res, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root, GoalHash: "dossier-knob"})
			if err != nil {
				t.Fatalf("RunCycle: %v", err)
			}
			if _, serr := os.Stat(filepath.Join(root, "knowledge-base", "cycles", "cycle-"+strconv.Itoa(res.Cycle)+".json")); serr != nil {
				t.Errorf("the dossier file is written either way: %v", serr)
			}
			if got := gitCommitCount(t, root) - before; got != tc.commits {
				t.Errorf("commits added = %d, want %d", got, tc.commits)
			}
		})
	}
}
