//go:build integration

package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/derived"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/researchindex"
)

const prRefreshMessage = "chore: regenerate the derived files"

func prVerb(args []string, stdout, stderr *bytes.Buffer) int {
	return runPR(args, nil, stdout, stderr)
}

func prRefreshFixture(t *testing.T, root string) *gittest.Repo {
	t.Helper()
	return gittest.FromTemplate(t, "cmd-evolve-pr-refresh-real", func(r *gittest.Repo) error {
		copyFile := func(rel string) error {
			data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
			if err != nil {
				return err
			}
			r.Write(rel, string(data))
			return nil
		}
		for _, rel := range []string{"docs/architecture/signal-codes.md", researchindex.IndexPath, "go/.darwin-set"} {
			if err := copyFile(rel); err != nil {
				return err
			}
		}
		entries, err := filepath.Glob(filepath.Join(root, researchindex.EntriesDir, "*.md"))
		if err != nil {
			return err
		}
		for _, path := range entries {
			if err := copyFile(researchindex.EntriesDir + "/" + filepath.Base(path)); err != nil {
				return err
			}
		}
		r.Git("add", "-A")
		r.Git("commit", "-q", "-m", "base")
		return nil
	})
}

func prRefreshTestWorktree(worktree string) derived.Worktree {
	return derived.Worktree{
		Run: func(ctx context.Context, args ...string) error {
			var stdout, stderr bytes.Buffer
			switch {
			case len(args) >= 2 && args[0] == "signals" && args[1] == "codes":
				if code := runSignals(args[1:], nil, &stdout, &stderr); code != 0 {
					return fmt.Errorf("evolve %s (exit %d): %s", strings.Join(args, " "), code, strings.TrimSpace(stderr.String()))
				}
				return nil
			case len(args) >= 2 && args[0] == "ci" && args[1] == "darwin-set":
				return nil
			case len(args) >= 3 && args[0] == "docs" && args[1] == "index":
				if code := docsIndexVerb.run([]string{args[2], "--project-root", worktree}, &stdout, &stderr); code != 0 {
					return fmt.Errorf("evolve %s (exit %d): %s", strings.Join(args, " "), code, strings.TrimSpace(stderr.String()))
				}
				return nil
			default:
				return core.DerivedWorktree(worktree).Run(ctx, args...)
			}
		},
		Git:  core.DerivedWorktree(worktree).Git,
		Base: "HEAD",
	}
}

func TestPRRefresh_TheRealGeneratorsOfACloneCommitOnlyTheStaleIndex(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	clone := prRefreshFixture(t, root)
	t.Setenv(ipcenv.WorktreeRootKey, clone.Dir)
	prev := prRefreshWorktree
	prRefreshWorktree = prRefreshTestWorktree
	t.Cleanup(func() { prRefreshWorktree = prev })
	if code, stdout, stderr := runVerb(prVerb, "refresh", "--project-root", clone.Dir); code != 0 || !strings.Contains(stdout, "up to date") {
		t.Fatalf("precondition: the derived files of HEAD must be in sync before the test adds an entry: %d %q %q", code, stdout, stderr)
	}
	clone.Write(researchindex.EntriesDir+"/2099-01-01-01-e2e.md", "## 2099-01-01 — the end-to-end entry\n")
	clone.Git("add", "-A")
	clone.Git("commit", "-q", "-m", "docs: an entry with no index line")

	code, stdout, stderr := runVerb(prVerb, "refresh", "--project-root", clone.Dir)

	if code != 0 || !strings.Contains(stdout, "committed") {
		t.Fatalf("refresh = %d %q %q", code, stdout, stderr)
	}
	if subject := clone.Git("log", "-1", "--format=%s"); subject != prRefreshMessage {
		t.Errorf("the refresh commit is %q, want %q", subject, prRefreshMessage)
	}
	if files := clone.Git("show", "--name-only", "--format=", "HEAD"); files != researchindex.IndexPath {
		t.Errorf("the refresh commit holds %q, want only %s: the signal codes of a clean tree are in sync", files, researchindex.IndexPath)
	}
	index, err := os.ReadFile(filepath.Join(clone.Dir, researchindex.IndexPath))
	if err != nil || !strings.Contains(string(index), "## 2099-01-01 — the end-to-end entry") {
		t.Errorf("the index lacks the new entry (err %v)", err)
	}
}
