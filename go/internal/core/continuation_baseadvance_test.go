package core

// continuation_baseadvance_test.go — pins for the worktree-base heal at
// continuation adoption; incident narrative and degrade contract: see
// continuation_baseadvance.go. The heal fixture reproduces the live
// cycle-1365 shape (stale base predating a landed .gitignore carve-out).

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestAdvanceContinuationBase_HealsStaleBase(t *testing.T) {
	root, wt := initContinuationRepo(t, 81)
	// Lane work on the preserved branch (non-conflicting with main's fix).
	if err := os.WriteFile(filepath.Join(wt, "lane.go"), []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := snapshotPreservedWorktree(context.Background(), wt); err != nil {
		t.Fatal(err)
	}
	// Main advances with the cycle-1365-shape fix: a .gitignore carve-out.
	// The real #418 ladder shape: parent excluded by glob (not by directory,
	// which would make re-inclusion impossible), evals carved back in.
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".evolve/*\n!.evolve/evals/\ngo/bin/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitOut(t, root, "add", ".gitignore")
	gitOut(t, root, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-m", "carve out .evolve/evals (the #418 shape)")
	mainTip := gitOut(t, root, "rev-parse", "main")

	healed := advanceContinuationBase(context.Background(), wt, 81)
	if healed != mainTip {
		t.Fatalf("advanceContinuationBase = %q, want main tip %s — the stale base was not healed", healed, mainTip)
	}
	// The worktree now carries the landed fix: .evolve/evals is stageable.
	evalsDir := filepath.Join(wt, ".evolve", "evals")
	if err := os.MkdirAll(evalsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evalsDir, "x.md"), []byte("# eval\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out := gitOut(t, wt, "add", ".evolve/evals/x.md"); out != "" {
		t.Fatalf("git add of the carved-out path failed post-heal: %s", out)
	}
	// Lane work survived the merge.
	if _, err := os.Stat(filepath.Join(wt, "lane.go")); err != nil {
		t.Error("lane work lost by the base advance")
	}
}

func TestAdvanceContinuationBase_NoOpWhenCurrent(t *testing.T) {
	_, wt := initContinuationRepo(t, 82)
	if healed := advanceContinuationBase(context.Background(), wt, 82); healed != "" {
		t.Fatalf("advance on an already-current base returned %q, want \"\" (no-op)", healed)
	}
}

func TestAdvanceContinuationBase_ConflictDegradesLoudlyToStaleBase(t *testing.T) {
	root, wt := initContinuationRepo(t, 83)
	// Lane and main edit the SAME file divergently (the raced-conflict shape
	// the adopt-time Clean screen normally excludes).
	if err := os.WriteFile(filepath.Join(wt, "a.txt"), []byte("lane\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := snapshotPreservedWorktree(context.Background(), wt); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitOut(t, root, "add", "a.txt")
	gitOut(t, root, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-m", "diverge")
	head := gitOut(t, wt, "rev-parse", "HEAD")

	if healed := advanceContinuationBase(context.Background(), wt, 83); healed != "" {
		t.Fatalf("conflicting advance returned %q, want \"\" (degrade to stale base)", healed)
	}
	if got := gitOut(t, wt, "rev-parse", "HEAD"); got != head {
		t.Errorf("worktree HEAD moved across an aborted merge: %s -> %s", head, got)
	}
	if status := gitOut(t, wt, "status", "--porcelain"); status != "" {
		t.Errorf("aborted merge left residue:\n%s", status)
	}
}
