package ship

// stage_deleted_test.go — boundary-flow pin: stageExplicitPaths must stage
// cleanly when a changed path is a DELETION that is already staged (the
// operator flow stages explicit paths → commit-gate → ship re-stages; plain
// `git add -- <deleted+staged>` fatals rc=128 "pathspec did not match any
// files", which broke every boundary ship after the explicit-paths rework).

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestStageExplicitPaths_AlreadyStagedDeletion(t *testing.T) {
	root := t.TempDir()
	gitIn(t, root, "init", "-q")
	gitIn(t, root, "config", "user.email", "t@t")
	gitIn(t, root, "config", "user.name", "t")
	if err := os.WriteFile(filepath.Join(root, "doomed.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "kept.txt"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "-q", "-m", "seed")
	// The boundary shape: delete + PRE-STAGE the deletion, plus a normal edit.
	if err := os.Remove(filepath.Join(root, "doomed.txt")); err != nil {
		t.Fatal(err)
	}
	gitIn(t, root, "add", "-A", "--", "doomed.txt")
	if err := os.WriteFile(filepath.Join(root, "kept.txt"), []byte("y2"), 0o644); err != nil {
		t.Fatal(err)
	}

	opts := &Options{ProjectRoot: root, Stderr: os.Stderr}
	res := &RunResult{}
	if err := stageExplicitPaths(context.Background(), opts, res, ""); err != nil {
		t.Fatalf("already-staged deletion must stage cleanly, got: %v", err)
	}
}
