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
	"strings"
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

// TestStageExplicitPaths_AlreadyStagedRename is the sibling of the deletion
// case above, and it is a DIFFERENT porcelain shape: git reports a staged
// rename as `R  old -> new`, never as `D  old`, so the "D " filter does not
// see the source path — yet the source is gone from the worktree AND from the
// index under its old name, which is exactly the rc=128 condition.
//
// Real trigger (2026-07-30 console): reconciling the inbox moves each consumed
// item from .evolve/inbox/<ts>-<id>.json to .evolve/inbox/consumed/<date>-<id>.json
// with one field appended, so git's similarity detection reports renames rather
// than delete+add, and every boundary ship carrying a consumed item died on
// `fatal: pathspec '.evolve/inbox/…-spine-failopen-telemetry.json' did not
// match any files`. This has been carried as a known operator gotcha
// ("ship-staging RENAME rc=128") instead of being fixed.
func TestStageExplicitPaths_AlreadyStagedRename(t *testing.T) {
	root := t.TempDir()
	gitIn(t, root, "init", "-q")
	gitIn(t, root, "config", "user.email", "t@t")
	gitIn(t, root, "config", "user.name", "t")
	// Content long enough that git scores old→new as a rename, not add+delete.
	body := []byte("{\n  \"id\": \"some-item\",\n  \"weight\": 0.85,\n  \"title\": \"a queued item with enough body to score as a rename\"\n}\n")
	if err := os.MkdirAll(filepath.Join(root, "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "inbox", "item.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "-q", "-m", "seed")

	// The boundary shape: move the file and PRE-STAGE the move.
	if err := os.MkdirAll(filepath.Join(root, "inbox", "consumed"), 0o755); err != nil {
		t.Fatal(err)
	}
	moved := append(append([]byte(nil), body[:len(body)-2]...), []byte(",\n  \"consumed_by\": \"landed\"\n}\n")...)
	if err := os.WriteFile(filepath.Join(root, "inbox", "consumed", "item.json"), moved, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "inbox", "item.json")); err != nil {
		t.Fatal(err)
	}
	gitIn(t, root, "add", "-A")

	// Guard the premise: if git stops calling this a rename the test is no
	// longer exercising the shape it claims to, and must fail loudly rather
	// than pass for the wrong reason.
	out, err := exec.Command("git", "-C", root, "status", "--porcelain").CombinedOutput()
	if err != nil {
		t.Fatalf("git status: %v", err)
	}
	if !strings.HasPrefix(string(out), "R ") {
		t.Fatalf("premise broken: expected a staged RENAME (`R  old -> new`), got:\n%s", out)
	}

	opts := &Options{ProjectRoot: root, Stderr: os.Stderr}
	res := &RunResult{}
	if err := stageExplicitPaths(context.Background(), opts, res, ""); err != nil {
		t.Fatalf("already-staged rename must stage cleanly, got: %v", err)
	}
}
