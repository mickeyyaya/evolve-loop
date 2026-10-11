//go:build integration

package explanationdocs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func swapSeam[T any](t *testing.T, seam *T, fake T) {
	t.Helper()
	saved := *seam
	*seam = fake
	t.Cleanup(func() { *seam = saved })
}

func nestedRepo(t *testing.T, f fixture, rel string, commit bool) string {
	t.Helper()
	repo := gittest.Fixture(t)
	head := ""
	if commit {
		repo.Git("commit", "-q", "--allow-empty", "-m", "nested")
		head = repo.Git("rev-parse", "HEAD")
	}
	dir := filepath.Join(f.worktree, rel)
	if err := os.Rename(repo.Dir, dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Rename(dir, repo.Dir) })
	return head
}

func TestCurrentDiffState_DigestsEachFileTypeByContent(t *testing.T) {
	f := newFixture(t)
	f.write(t, "plain.txt", "plain body\n")
	f.write(t, "tool.sh", "#!/bin/sh\n")
	if err := os.Chmod(filepath.Join(f.worktree, "tool.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("plain.txt", filepath.Join(f.worktree, "link")); err != nil {
		t.Fatal(err)
	}
	commit := nestedRepo(t, f, "sub", true)
	for _, tc := range []struct {
		rel, mode, sha string
		size           int64
	}{
		{"plain.txt", "100644", sha256Hex("plain body\n"), 11},
		{"tool.sh", "100755", sha256Hex("#!/bin/sh\n"), 10},
		{"link", "120000", sha256Hex("plain.txt"), 9},
		{"sub", "160000", sha256Hex(commit), 40},
		{"gone.txt", "absent", "-", 0},
	} {
		t.Run(tc.rel, func(t *testing.T) {
			mode, sha, size, err := currentDiffState(context.Background(), f.worktree, tc.rel, 1<<20)

			if err != nil || mode != tc.mode || sha != tc.sha || size != tc.size {
				t.Fatalf("currentDiffState(%q) = %q, %q, %d, %v; want %q, %q, %d", tc.rel, mode, sha, size, err, tc.mode, tc.sha, tc.size)
			}
		})
	}
}

func TestCurrentDiffState_RefusesWhatItCannotDigestSafely(t *testing.T) {
	f := newFixture(t)
	f.write(t, "plain.txt", "plain body\n")
	nestedRepo(t, f, "empty-sub", false)
	nestedRepo(t, f, "full-sub", true)
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(f.worktree, "escape")); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(f.worktree, "pipe"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, worktree, rel string
		remaining           int64
		want                string
	}{
		{"negative budget", f.worktree, "plain.txt", -1, errDiffDigestLimit.Error()},
		{"missing worktree", filepath.Join(f.worktree, "nope"), "plain.txt", 1 << 20, "resolve worktree"},
		{"file used as a directory", f.worktree, "plain.txt/child", 1 << 20, "not a directory"},
		{"parent escapes through a symlink", f.worktree, "escape/secret.txt", 1 << 20, "changed path escapes the worktree"},
		{"gitlink without a commit", f.worktree, "empty-sub", 1 << 20, "changed directory is not a readable gitlink"},
		{"gitlink beyond the budget", f.worktree, "full-sub", 39, "gitlink commit is invalid or exceeds the digest limit"},
		{"fifo", f.worktree, "pipe", 1 << 20, "unsupported file type"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mode, sha, size, err := currentDiffState(context.Background(), tc.worktree, tc.rel, tc.remaining)

			if err == nil || !strings.Contains(err.Error(), tc.want) || mode != "" || sha != "" || size != 0 {
				t.Fatalf("currentDiffState = %q, %q, %d, %v; want only an error naming %q", mode, sha, size, err, tc.want)
			}
		})
	}
}

func TestCurrentDiffState_AFileOneByteOverTheBudgetIsTheDigestLimit(t *testing.T) {
	f := newFixture(t)
	f.write(t, "plain.txt", "plain body\n")

	_, over, _, overErr := currentDiffState(context.Background(), f.worktree, "plain.txt", 10)
	_, exact, size, exactErr := currentDiffState(context.Background(), f.worktree, "plain.txt", 11)

	if !errors.Is(overErr, errDiffDigestLimit) || over != "" {
		t.Fatalf("over budget = %q, %v; want errDiffDigestLimit and no digest", over, overErr)
	}
	if exactErr != nil || exact != sha256Hex("plain body\n") || size != 11 {
		t.Fatalf("exact budget = %q, %d, %v; want the digest", exact, size, exactErr)
	}
}

func TestCurrentDiffState_GitlinkFitsAnExactBudget(t *testing.T) {
	f := newFixture(t)
	commit := nestedRepo(t, f, "sub", true)

	mode, sha, size, err := currentDiffState(context.Background(), f.worktree, "sub", int64(len(commit)))

	if err != nil || mode != "160000" || sha != sha256Hex(commit) || size != int64(len(commit)) {
		t.Fatalf("currentDiffState = %q, %q, %d, %v; want the gitlink digest at an exact budget", mode, sha, size, err)
	}
}

func TestCurrentDiffState_RacesAndResolutionFailuresAreErrors(t *testing.T) {
	f := newFixture(t)
	f.write(t, "plain.txt", "plain body\n")
	t.Run("unresolvable worktree", func(t *testing.T) {
		swapSeam(t, &absPath, func(string) (string, error) { return "", errors.New("getwd failed") })
		_, _, _, err := currentDiffState(context.Background(), "relative", "plain.txt", 1<<20)
		if err == nil || err.Error() != "getwd failed" {
			t.Fatalf("err = %v, want the Abs error", err)
		}
	})
	t.Run("deleted path reappears", func(t *testing.T) {
		calls := 0
		swapSeam(t, &lstat, func(p string) (os.FileInfo, error) {
			calls++
			if calls == 1 {
				return nil, &fs.PathError{Op: "lstat", Path: p, Err: fs.ErrNotExist}
			}
			return os.Lstat(p)
		})
		_, _, _, err := currentDiffState(context.Background(), f.worktree, "plain.txt", 1<<20)
		if err == nil || err.Error() != "deleted path changed while hashing" || calls != 2 {
			t.Fatalf("err = %v after %d lstats, want the changed-while-hashing error", err, calls)
		}
	})
	t.Run("parent unresolvable", func(t *testing.T) {
		calls := 0
		swapSeam(t, &evalSymlinks, func(p string) (string, error) {
			calls++
			if calls == 2 {
				return "", errors.New("parent vanished")
			}
			return filepath.EvalSymlinks(p)
		})
		_, _, _, err := currentDiffState(context.Background(), f.worktree, "plain.txt", 1<<20)
		if err == nil || err.Error() != "resolve changed-path parent: parent vanished" {
			t.Fatalf("err = %v, want the parent resolution error", err)
		}
	})
}
