package core

// dossier_producer_simulate_test.go — the closeout dossier's git commit is a
// decision the ROOT makes, not the producer: a --simulate walk (no-LLM
// plumbing check) writes its dossier files but never commits into the
// operator's repo. Before this, `evolve campaign run --simulate` from a
// checkout left `dossier: cycle-N closeout` commits on the operator's branch
// (acs/cycle8 on every whole-module floor — the 2026-09-14 verification wave;
// docs/incidents/2026-09-14-simulate-runs-against-the-checkout.md).

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// FilesOnly writes the two dossier files and touches git not at all — so a
// project root that is NOT a git working tree is fine; the committing default
// on the same root fails, because there is nothing to commit into.
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

// WithDossierCommit is the root's knob: default true (every production cycle
// leaves a committed record — one more commit on the repo), false for the
// simulate root (the record is written, the repo's history is untouched).
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
