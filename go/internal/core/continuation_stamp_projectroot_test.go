package core

// continuation_stamp_projectroot_test.go — a salvage snapshot (ADR-0076) is
// `git add -A` + commit of a cycle's ISOLATED worktree. When the active
// worktree is the project root itself — the --simulate root reads the root in
// place; a resume checkpoint can name it — that would commit the operator's
// own tree (the second mutation the 2026-09-14 simulate incident found after
// the dossier commit). The stamp refuses, out loud, and stamps no
// continuation. The refusal is the package's ONE "is this the live
// repository" predicate (sameDirectory), so a symlink alias or a relative
// spelling of the root is refused exactly like the absolute path.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
)

func TestStampContinuation_NeverSnapshotsTheProjectRoot(t *testing.T) {
	root := t.TempDir()
	initDossierRepo(t, root)
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(cwd, root)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, worktree, projectRoot string }{
		{"absolute", root, root},
		{"symlink alias of the root", alias, root},
		{"relative spelling of the root", root, relative},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// A dirty root: an untracked file a snapshot would sweep up with add -A.
			if err := os.WriteFile(filepath.Join(root, "untracked.txt"), []byte("operator's work\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			before := gitCommitCount(t, root)
			ws := filepath.Join(root, ".evolve", "runs", "cycle-9")
			if err := os.MkdirAll(ws, 0o755); err != nil {
				t.Fatal(err)
			}
			o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
			stderr := captureStderr(t, func() {
				o.stampContinuationManifest(context.Background(), CycleState{ActiveWorktree: tc.worktree, WorkspacePath: ws}, 9, tc.projectRoot)
			})
			if got := gitCommitCount(t, root); got != before {
				t.Errorf("the project root was snapshot-committed: %d → %d commits", before, got)
			}
			if _, ok, _ := continuation.ReadManifest(ws); ok {
				t.Error("no continuation is stamped for a root-resident worktree")
			}
			if _, err := os.Stat(filepath.Join(root, "untracked.txt")); err != nil {
				t.Error("the operator's untracked work is untouched")
			}
			if !strings.Contains(stderr, "the active worktree is the project root — no salvage snapshot") {
				t.Errorf("the refusal is said to the operator, got %q", stderr)
			}
		})
	}
}
