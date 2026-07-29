package looppreflight

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

// stubProbe swaps the git probe for the duration of a test.
func stubProbe(t *testing.T, fn func(context.Context, string) (baseState, error)) {
	t.Helper()
	orig := baseDivergenceProbe
	baseDivergenceProbe = fn
	t.Cleanup(func() { baseDivergenceProbe = orig })
}

// TestCheckBaseDivergence_BehindHalts — the cycle-969 case: a base behind
// origin must HALT and the halt must name `evolve sync-main`, so the operator
// gets a stop WITH a next step.
func TestCheckBaseDivergence_BehindHalts(t *testing.T) {
	stubProbe(t, func(context.Context, string) (baseState, error) {
		return baseState{Branch: "main", Ahead: 0, Behind: 3}, nil
	})

	c := checkBaseDivergence(resolved{projectRoot: "/repo"})
	if c.Level != LevelHalt {
		t.Fatalf("behind-origin base: want LevelHalt, got %s (%s)", c.Level, c.Detail)
	}
	if !strings.Contains(c.Message+c.Detail, reconcileCommand) {
		t.Errorf("halt does not name %q:\n%s\n%s", reconcileCommand, c.Message, c.Detail)
	}
}

// TestCheckBaseDivergence_InSyncPasses — the anti-blanket-halt negative. An
// in-sync base must pass or every healthy boot is benched.
func TestCheckBaseDivergence_InSyncPasses(t *testing.T) {
	stubProbe(t, func(context.Context, string) (baseState, error) {
		return baseState{Branch: "main"}, nil
	})

	if c := checkBaseDivergence(resolved{projectRoot: "/repo"}); c.Level != LevelPass {
		t.Fatalf("in-sync base: want LevelPass, got %s (%s)", c.Level, c.Detail)
	}
}

// TestCheckBaseDivergence_AheadOnlyPasses — unpushed local work is normal and
// must not halt; only BEHIND (stale base) does.
func TestCheckBaseDivergence_AheadOnlyPasses(t *testing.T) {
	stubProbe(t, func(context.Context, string) (baseState, error) {
		return baseState{Branch: "main", Ahead: 4}, nil
	})

	if c := checkBaseDivergence(resolved{projectRoot: "/repo"}); c.Level != LevelPass {
		t.Fatalf("ahead-only base: want LevelPass, got %s (%s)", c.Level, c.Detail)
	}
}

// TestCheckBaseDivergence_ProbeErrorWarns — an unverifiable comparison is
// surfaced as Warn, never a silent PASS (a fetch failure must not read as
// "base is fine") and never a Halt (a transient network fault must not bench a
// ready boot).
func TestCheckBaseDivergence_ProbeErrorWarns(t *testing.T) {
	stubProbe(t, func(context.Context, string) (baseState, error) {
		return baseState{}, errors.New("fetch origin main: connection refused")
	})

	c := checkBaseDivergence(resolved{projectRoot: "/repo"})
	if c.Level != LevelWarn {
		t.Fatalf("probe error: want LevelWarn, got %s (%s)", c.Level, c.Detail)
	}
	if !strings.Contains(c.Detail, "connection refused") {
		t.Errorf("warn detail drops the underlying cause: %s", c.Detail)
	}
}

// TestCheckBaseDivergence_SkippedPasses — a non-repo / no-origin project root
// has nothing to compare and must pass with the reason visible.
func TestCheckBaseDivergence_SkippedPasses(t *testing.T) {
	stubProbe(t, func(context.Context, string) (baseState, error) {
		return baseState{Skipped: true, Reason: "no `origin` remote"}, nil
	})

	c := checkBaseDivergence(resolved{projectRoot: "/repo"})
	if c.Level != LevelPass {
		t.Fatalf("skipped probe: want LevelPass, got %s", c.Level)
	}
	if !strings.Contains(c.Detail, "origin") {
		t.Errorf("skip reason not surfaced: %s", c.Detail)
	}
}

// TestParseLeftRightCount covers the rev-list parse, including the malformed
// output that must become an error rather than a silent 0/0 ("up to date").
func TestParseLeftRightCount(t *testing.T) {
	ahead, behind, err := parseLeftRightCount("2\t5\n")
	if err != nil || ahead != 2 || behind != 5 {
		t.Fatalf("got ahead=%d behind=%d err=%v; want 2/5/nil", ahead, behind, err)
	}
	if _, _, err := parseLeftRightCount("garbage"); err == nil {
		t.Errorf("malformed rev-list output must error, not report 0 behind")
	}
}

// TestHasRemoteOrigin guards the remote-name match against substring hits like
// "upstream-origin".
func TestHasRemoteOrigin(t *testing.T) {
	if !hasRemoteOrigin("upstream\norigin\n") {
		t.Errorf("origin present but not detected")
	}
	if hasRemoteOrigin("upstream-origin\n") {
		t.Errorf("substring match wrongly detected origin")
	}
}

// --- defaultBaseDivergenceProbe: scripted-git branch coverage ---

// scriptGit installs a fake git whose reply is chosen by the first argument
// (the git subcommand). A "" reply with a non-nil error slot makes that
// subcommand exit non-zero, which gitexec folds into an error.
func scriptGit(t *testing.T, replies map[string]string, failing map[string]bool) {
	t.Helper()
	orig := newGit
	newGit = func(dir string) gitexec.Git {
		return gitexec.Git{Dir: dir, Exec: func(_ context.Context, _, _ string, args, _ []string,
			_ io.Reader, stdout, stderr io.Writer) (int, error) {
			sub := ""
			if len(args) > 0 {
				sub = args[0]
			}
			if failing[sub] {
				fmt.Fprintf(stderr, "fatal: %s failed", sub)
				return 1, nil
			}
			fmt.Fprint(stdout, replies[sub])
			return 0, nil
		}}
	}
	t.Cleanup(func() { newGit = orig })
}

// healthyReplies is a scripted repo on main, with origin, in sync.
func healthyReplies() map[string]string {
	return map[string]string{
		"rev-parse": "main",
		"remote":    "origin\n",
		"fetch":     "",
		"rev-list":  "0\t0\n",
	}
}

// TestDefaultProbe_BehindIsReported — the probe must surface the behind count
// the halt is built from.
func TestDefaultProbe_BehindIsReported(t *testing.T) {
	r := healthyReplies()
	r["rev-list"] = "1\t7\n"
	scriptGit(t, r, nil)

	st, err := defaultBaseDivergenceProbe(context.Background(), "/repo")
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if st.Skipped || st.Behind != 7 || st.Ahead != 1 || st.Branch != "main" {
		t.Fatalf("got %+v; want main 1 ahead / 7 behind, not skipped", st)
	}
}

// TestDefaultProbe_InSync — 0/0 is a clean, non-skipped verdict.
func TestDefaultProbe_InSync(t *testing.T) {
	scriptGit(t, healthyReplies(), nil)

	st, err := defaultBaseDivergenceProbe(context.Background(), "/repo")
	if err != nil || st.Skipped || st.Behind != 0 {
		t.Fatalf("got %+v err=%v; want in-sync non-skipped", st, err)
	}
}

// TestDefaultProbe_NotAWorkTree — a non-repo project root is skipped, not an
// error: the loop legitimately runs outside a checkout in some fixtures.
func TestDefaultProbe_NotAWorkTree(t *testing.T) {
	scriptGit(t, healthyReplies(), map[string]bool{"rev-parse": true})

	st, err := defaultBaseDivergenceProbe(context.Background(), "/repo")
	if err != nil || !st.Skipped {
		t.Fatalf("got %+v err=%v; want skipped", st, err)
	}
}

// TestDefaultProbe_DetachedHead — no base branch to compare against.
func TestDefaultProbe_DetachedHead(t *testing.T) {
	r := healthyReplies()
	r["rev-parse"] = "HEAD"
	scriptGit(t, r, nil)

	st, err := defaultBaseDivergenceProbe(context.Background(), "/repo")
	if err != nil || !st.Skipped || !strings.Contains(st.Reason, "detached") {
		t.Fatalf("got %+v err=%v; want detached-HEAD skip", st, err)
	}
}

// TestDefaultProbe_NoOriginRemote — nothing to diverge from.
func TestDefaultProbe_NoOriginRemote(t *testing.T) {
	r := healthyReplies()
	r["remote"] = "upstream\n"
	scriptGit(t, r, nil)

	st, err := defaultBaseDivergenceProbe(context.Background(), "/repo")
	if err != nil || !st.Skipped || !strings.Contains(st.Reason, "origin") {
		t.Fatalf("got %+v err=%v; want no-origin skip", st, err)
	}
}

// TestDefaultProbe_FetchFailureErrors — the fetch is the whole point of the
// check, so a failed fetch must become an error (→ Warn), NEVER a clean 0/0
// that reads as "base is up to date".
func TestDefaultProbe_FetchFailureErrors(t *testing.T) {
	scriptGit(t, healthyReplies(), map[string]bool{"fetch": true})

	st, err := defaultBaseDivergenceProbe(context.Background(), "/repo")
	if err == nil {
		t.Fatalf("fetch failure returned %+v with nil error — a stale base would pass silently", st)
	}
	if st.Skipped {
		t.Errorf("fetch failure must not be reported as a skip")
	}
}

// TestDefaultProbe_RemoteListFailureErrors — an unusable git also degrades to
// UNVERIFIED rather than a skip.
func TestDefaultProbe_RemoteListFailureErrors(t *testing.T) {
	scriptGit(t, healthyReplies(), map[string]bool{"remote": true})

	if _, err := defaultBaseDivergenceProbe(context.Background(), "/repo"); err == nil {
		t.Fatalf("remote-list failure must error")
	}
}

// TestDefaultProbe_RevListFailureErrors — likewise for the comparison itself.
func TestDefaultProbe_RevListFailureErrors(t *testing.T) {
	scriptGit(t, healthyReplies(), map[string]bool{"rev-list": true})

	if _, err := defaultBaseDivergenceProbe(context.Background(), "/repo"); err == nil {
		t.Fatalf("rev-list failure must error")
	}
}

// TestDefaultProbe_MalformedRevListErrors — garbage counts must not silently
// become "0 behind".
func TestDefaultProbe_MalformedRevListErrors(t *testing.T) {
	r := healthyReplies()
	r["rev-list"] = "not-a-count\n"
	scriptGit(t, r, nil)

	if _, err := defaultBaseDivergenceProbe(context.Background(), "/repo"); err == nil {
		t.Fatalf("malformed rev-list output must error")
	}
}

// TestDefaultProbe_BranchResolveFailureErrors — an empty branch name from a
// failing rev-parse --abbrev-ref must not be treated as a work tree check.
func TestDefaultProbe_BranchResolveFailureErrors(t *testing.T) {
	calls := 0
	orig := newGit
	newGit = func(dir string) gitexec.Git {
		return gitexec.Git{Dir: dir, Exec: func(_ context.Context, _, _ string, args, _ []string,
			_ io.Reader, stdout, stderr io.Writer) (int, error) {
			// First rev-parse (--is-inside-work-tree) succeeds; the second
			// (--abbrev-ref HEAD) fails.
			if len(args) > 0 && args[0] == "rev-parse" {
				calls++
				if calls == 1 {
					fmt.Fprint(stdout, "true\n")
					return 0, nil
				}
				fmt.Fprint(stderr, "fatal: ambiguous HEAD")
				return 128, nil
			}
			return 0, nil
		}}
	}
	t.Cleanup(func() { newGit = orig })

	if _, err := defaultBaseDivergenceProbe(context.Background(), "/repo"); err == nil {
		t.Fatalf("branch-resolve failure must error, not silently skip")
	}
}
