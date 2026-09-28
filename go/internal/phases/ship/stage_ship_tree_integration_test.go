//go:build integration

package ship

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/shipmanifest"
)

func TestShipFromWorktree_CommitsTheOneShipTree(t *testing.T) {
	repo, wt := makeWorktreeScenario(t)
	mustWrite(t, filepath.Join(wt, "fixture.txt"), "fixture line 1\nan undeclared tracked edit\n")
	mustWrite(t, filepath.Join(wt, "docs", "explain", "cycle-1.md"), "# Why\n")
	mustWrite(t, filepath.Join(wt, "residue.txt"), "another lane's scratch\n")
	ws := writeWorkspaceReports(t, "wt-change.txt", "docs/explain/cycle-1.md")
	ctx := context.Background()
	want, err := shipmanifest.TakeShipTree(ctx, shipmanifest.GitIn(ctx, wt), wt, ws)
	if err != nil {
		t.Fatal(err)
	}

	opts := &Options{Class: ClassCycle, CommitMessage: "feat: the one ship tree", ProjectRoot: repo, PluginRoot: repo,
		WorkspacePath: ws, Stdout: io.Discard, Stderr: io.Discard}
	if err := shipFromWorktree(ctx, opts, &RunResult{}, "main", wt); err != nil {
		t.Fatalf("shipFromWorktree: %v", err)
	}
	out, err := exec.Command("git", "-C", wt, "rev-parse", "cycle-1^{tree}").Output()
	if err != nil {
		t.Fatal(err)
	}
	if committed := strings.TrimSpace(string(out)); committed != want.Tree {
		names, _ := exec.Command("git", "-C", wt, "ls-tree", "-r", "--name-only", committed).Output()
		t.Fatalf("Ship committed %s, the one ship tree is %s (committed: %q): the audit, the binding and Ship must agree", committed, want.Tree, names)
	}
}

func TestShipFromWorktree_ADeclaredFileRemovedWithPlainRmShipsItsDeletion(t *testing.T) {
	repo, wt := makeWorktreeScenario(t)
	if err := os.Remove(filepath.Join(wt, "fixture.txt")); err != nil {
		t.Fatal(err)
	}
	ws := writeWorkspaceReports(t, "wt-change.txt", "fixture.txt")
	ctx := context.Background()
	want, err := shipmanifest.TakeShipTree(ctx, shipmanifest.GitIn(ctx, wt), wt, ws)
	if err != nil {
		t.Fatal(err)
	}
	opts := &Options{Class: ClassCycle, CommitMessage: "refactor: delete the fixture", ProjectRoot: repo, PluginRoot: repo,
		WorkspacePath: ws, Stdout: io.Discard, Stderr: io.Discard}
	if err := shipFromWorktree(ctx, opts, &RunResult{}, "main", wt); err != nil {
		t.Fatalf("a declared file removed with plain rm ships its deletion: %v", err)
	}
	out, err := exec.Command("git", "-C", wt, "rev-parse", "cycle-1^{tree}").Output()
	if err != nil {
		t.Fatal(err)
	}
	if committed := strings.TrimSpace(string(out)); committed != want.Tree {
		t.Fatalf("Ship committed %s, the one ship tree is %s", committed, want.Tree)
	}
	if names, _ := exec.Command("git", "-C", wt, "ls-tree", "-r", "--name-only", "cycle-1").Output(); strings.Contains(string(names), "fixture.txt") {
		t.Fatalf("the declared deletion is committed: %q", names)
	}
}

func TestShipFromWorktree_AFailedTrackedEditAddFailsTheShipBeforeAnyCommit(t *testing.T) {
	repo, wt := makeWorktreeScenario(t)
	mustWrite(t, filepath.Join(wt, "fixture.txt"), "fixture line 1\nan undeclared tracked edit\n")
	var commits int
	runner := func(ctx context.Context, name, cwd string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if name == "git" && slices.Contains(args, "commit") {
			commits++
		}
		if name == "git" && slices.Contains(args, "add") && slices.Contains(args, "-u") {
			_, _ = io.WriteString(stderr, "fatal: Unable to create '.git/index.lock': File exists.\n")
			return 128, nil
		}
		return execRunner(ctx, name, cwd, args, env, stdin, stdout, stderr)
	}
	opts := &Options{Class: ClassCycle, CommitMessage: "feat: add -u fails", ProjectRoot: repo, PluginRoot: repo,
		WorkspacePath: writeWorkspaceReports(t, "wt-change.txt"), Runner: runner, Stdout: io.Discard, Stderr: io.Discard}
	err := shipFromWorktree(context.Background(), opts, &RunResult{}, "main", wt)
	se, ok := core.AsShipError(err)
	if !ok || se.Code != core.CodeGitStageFailed || se.Class != core.ShipClassTransient {
		t.Fatalf("a failed add -u is a transient stage failure, never swallowed: %v", err)
	}
	if !strings.Contains(err.Error(), "index.lock") || commits != 0 {
		t.Fatalf("the failure carries git's reason and no commit is attempted (commits=%d): %v", commits, err)
	}
}
