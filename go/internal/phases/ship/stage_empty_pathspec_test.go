package ship

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestStageExplicitPaths_NothingSelectedStagesNothing(t *testing.T) {
	for _, tc := range []struct {
		name     string
		declared string
		change   func(t *testing.T, root string)
	}{
		{"the only declared change is an already-staged deletion", "doomed.txt", func(t *testing.T, root string) {
			if err := os.Remove(filepath.Join(root, "doomed.txt")); err != nil {
				t.Fatal(err)
			}
			gitIn(t, root, "add", "-A", "--", "doomed.txt")
		}},
		{"the only declared change is gitignored", "gen/out.txt", func(t *testing.T, root string) {
			mustWrite(t, filepath.Join(root, "gen", "out.txt"), "generated")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			gitIn(t, root, "init", "-q")
			gitIn(t, root, "config", "user.email", "t@t")
			gitIn(t, root, "config", "user.name", "t")
			mustWrite(t, filepath.Join(root, "doomed.txt"), "x")
			mustWrite(t, filepath.Join(root, ".gitignore"), "gen/\n")
			gitIn(t, root, "add", "-A")
			gitIn(t, root, "commit", "-q", "-m", "seed")
			tc.change(t, root)
			mustWrite(t, filepath.Join(root, "sibling-lane-leak.txt"), "undeclared")

			opts := &Options{ProjectRoot: root, WorkspacePath: writeWorkspaceReports(t, tc.declared), Stderr: io.Discard}
			res := &RunResult{}
			if err := stageExplicitPaths(context.Background(), opts, res, ""); err != nil {
				t.Fatalf("an empty selection is not a staging failure: %v", err)
			}
			out, err := exec.Command("git", "-C", root, "diff", "--cached", "--name-only").Output()
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(out), "sibling-lane-leak.txt") {
				t.Fatalf("an empty pathspec staged the whole tree, undeclared leak included:\n%s\nlogs=%v", out, res.Logs)
			}
		})
	}
}
