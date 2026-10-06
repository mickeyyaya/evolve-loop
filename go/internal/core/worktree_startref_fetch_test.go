package core

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type fetchRecorder struct {
	argv      [][]string
	contended int
}

func (f *fetchRecorder) runner() sysexec.RunFunc {
	return func(ctx context.Context, name, dir string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if len(args) > 0 && args[0] == "fetch" {
			f.argv = append(f.argv, append([]string(nil), args...))
			if f.contended > 0 {
				f.contended--
				io.WriteString(stderr, "error: cannot lock ref 'refs/remotes/origin/train/wave64-boundary': is at e1d9ae9e but expected 0bc8caed\n")
				return 1, nil
			}
		}
		return sysexec.DefaultRunner(ctx, name, dir, args, env, stdin, stdout, stderr)
	}
}

func withFetchSeams(t *testing.T, rec *fetchRecorder) *[]time.Duration {
	t.Helper()
	prevRunner, prevSleep := gitRunner, worktreeAddRetrySleep
	t.Cleanup(func() { gitRunner, worktreeAddRetrySleep = prevRunner, prevSleep })
	slept := &[]time.Duration{}
	gitRunner = rec.runner()
	worktreeAddRetrySleep = func(d time.Duration) { *slept = append(*slept, d) }
	return slept
}

func TestLaneStartRef_FetchesOnlyTheDefaultBranch(t *testing.T) {
	seed, runtime := startRefFixture(t)
	seed.Git("push", "-q", "origin", "main:train/wave64-boundary")
	rec := &fetchRecorder{}
	withFetchSeams(t, rec)

	ref, err := laneStartRef(context.Background(), runtime.Dir)

	if err != nil || ref != "origin/main" {
		t.Fatalf("ref=%q err=%v, want origin/main", ref, err)
	}
	want := "fetch origin +refs/heads/main:refs/remotes/origin/main"
	if len(rec.argv) != 1 || strings.Join(rec.argv[0], " ") != want {
		t.Fatalf("fetches = %v, want exactly [%s]", rec.argv, want)
	}
	if out := runtime.Git("for-each-ref", "refs/remotes/origin/train"); out != "" {
		t.Errorf("the lane base fetch touched an unrelated remote-tracking ref: %q", out)
	}
}

func TestLaneStartRef_RetriesRefLockContention(t *testing.T) {
	_, runtime := startRefFixture(t)
	rec := &fetchRecorder{contended: 1}
	slept := withFetchSeams(t, rec)

	var ref string
	var err error
	out := captureStderr(t, func() { ref, err = laneStartRef(context.Background(), runtime.Dir) })

	if err != nil || ref != "origin/main" {
		t.Fatalf("one ref-lock contention must not fail the lane base: ref=%q err=%v", ref, err)
	}
	if len(rec.argv) != 2 || len(*slept) != 1 {
		t.Fatalf("fetches=%d sleeps=%v, want 2 fetches and one backoff", len(rec.argv), *slept)
	}
	if !strings.Contains(out, "[worktree] retry 1/2: git fetch origin main after ref-lock contention rc=1") {
		t.Errorf("the retry is announced on stderr, got %q", out)
	}
}

func TestLaneStartRef_PersistentContentionFailsAfterTheBound(t *testing.T) {
	_, runtime := startRefFixture(t)
	rec := &fetchRecorder{contended: 99}
	withFetchSeams(t, rec)

	ref, err := laneStartRef(context.Background(), runtime.Dir)

	if err == nil || !strings.Contains(err.Error(), "cannot lock ref") {
		t.Fatalf("ref=%q err=%v, want the contention surfaced after the bound", ref, err)
	}
	if len(rec.argv) != gitexec.DefaultFetchAttempts {
		t.Errorf("fetches=%d, want gitexec.DefaultFetchAttempts=%d", len(rec.argv), gitexec.DefaultFetchAttempts)
	}
}

func TestLaneStartRef_RealLockOnAnUnrelatedRefDoesNotFailTheBase(t *testing.T) {
	seed, runtime := startRefFixture(t)
	seed.Git("push", "-q", "origin", "main:train/wave64-boundary")
	runtime.Git("fetch", "-q", "origin")
	writeCommit(t, seed, "moved.txt", "moved")
	seed.Git("push", "-q", "origin", "main", "main:train/wave64-boundary")
	lock := filepath.Join(runtime.Dir, ".git", "refs", "remotes", "origin", "train", "wave64-boundary.lock")
	if err := os.MkdirAll(filepath.Dir(lock), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lock, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(lock) })

	ref, err := laneStartRef(context.Background(), runtime.Dir)

	if err != nil || ref != "origin/main" {
		t.Fatalf("a held lock on an unrelated remote-tracking ref failed the lane base: ref=%q err=%v", ref, err)
	}
	if got, want := runtime.Git("rev-parse", "origin/main"), seed.Git("rev-parse", "HEAD"); got != want {
		t.Errorf("origin/main = %s, want the pushed tip %s", got, want)
	}
}

func TestLaneStartRef_RealLockOnTheBaseRefIsRetried(t *testing.T) {
	seed, runtime := startRefFixture(t)
	writeCommit(t, seed, "moved.txt", "moved")
	seed.Git("push", "-q", "origin", "main")
	lock := filepath.Join(runtime.Dir, ".git", "refs", "remotes", "origin", "main.lock")
	if err := os.MkdirAll(filepath.Dir(lock), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lock, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	prevSleep := worktreeAddRetrySleep
	t.Cleanup(func() { worktreeAddRetrySleep = prevSleep })
	released := 0
	worktreeAddRetrySleep = func(time.Duration) {
		released++
		os.Remove(lock)
	}

	ref, err := laneStartRef(context.Background(), runtime.Dir)

	if err != nil || ref != "origin/main" || released != 1 {
		t.Fatalf("ref=%q err=%v backoffs=%d, want origin/main after one backoff: git's own lock-contention stderr must classify as retryable", ref, err, released)
	}
}

func TestLaneStartRef_FollowsTheRemotesDefaultBranch(t *testing.T) {
	var fetches [][]string
	prevRunner := gitRunner
	t.Cleanup(func() { gitRunner = prevRunner })
	gitRunner = func(_ context.Context, _, _ string, args, _ []string, _ io.Reader, stdout, _ io.Writer) (int, error) {
		switch strings.Join(args, " ") {
		case "config --get remote.origin.url":
			io.WriteString(stdout, "https://example.invalid/repo.git\n")
		case "symbolic-ref --quiet --short refs/remotes/origin/HEAD":
			io.WriteString(stdout, "origin/trunk\n")
		case "symbolic-ref --quiet --short HEAD":
			return 1, nil
		default:
			if args[0] != "fetch" {
				t.Fatalf("unexpected git %v", args)
			}
			fetches = append(fetches, args)
		}
		return 0, nil
	}

	ref, err := laneStartRef(context.Background(), t.TempDir())

	if err != nil || ref != "origin/trunk" {
		t.Fatalf("ref=%q err=%v, want origin/trunk: the base is the remote's default branch, not a hard-coded main", ref, err)
	}
	want := "fetch origin +refs/heads/trunk:refs/remotes/origin/trunk"
	if len(fetches) != 1 || strings.Join(fetches[0], " ") != want {
		t.Errorf("fetches = %v, want exactly [%s]", fetches, want)
	}
}
