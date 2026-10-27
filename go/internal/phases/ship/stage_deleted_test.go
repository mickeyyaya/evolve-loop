package ship

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
	t.Parallel()
	root := emptyRepo(t).Dir
	if err := os.WriteFile(filepath.Join(root, "doomed.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "kept.txt"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "-q", "-m", "seed")
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

func TestStageExplicitPaths_AlreadyStagedRename(t *testing.T) {
	t.Parallel()
	root := emptyRepo(t).Dir
	body := []byte("{\n  \"id\": \"some-item\",\n  \"weight\": 0.85,\n  \"title\": \"a queued item with enough body to score as a rename\"\n}\n")
	if err := os.MkdirAll(filepath.Join(root, "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "inbox", "item.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "-q", "-m", "seed")

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
