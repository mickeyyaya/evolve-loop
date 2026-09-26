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

func stubProbe(t *testing.T, fn func(context.Context, string) (baseState, error)) {
	t.Helper()
	orig := baseDivergenceProbe
	baseDivergenceProbe = fn
	t.Cleanup(func() { baseDivergenceProbe = orig })
}

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

func TestCheckBaseDivergence_InSyncPasses(t *testing.T) {
	stubProbe(t, func(context.Context, string) (baseState, error) {
		return baseState{Branch: "main"}, nil
	})

	if c := checkBaseDivergence(resolved{projectRoot: "/repo"}); c.Level != LevelPass {
		t.Fatalf("in-sync base: want LevelPass, got %s (%s)", c.Level, c.Detail)
	}
}

func TestCheckBaseDivergence_AheadOnlyPasses(t *testing.T) {
	stubProbe(t, func(context.Context, string) (baseState, error) {
		return baseState{Branch: "main", Ahead: 4}, nil
	})

	if c := checkBaseDivergence(resolved{projectRoot: "/repo"}); c.Level != LevelPass {
		t.Fatalf("ahead-only base: want LevelPass, got %s (%s)", c.Level, c.Detail)
	}
}

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

func TestParseLeftRightCount(t *testing.T) {
	ahead, behind, err := parseLeftRightCount("2\t5\n")
	if err != nil || ahead != 2 || behind != 5 {
		t.Fatalf("got ahead=%d behind=%d err=%v; want 2/5/nil", ahead, behind, err)
	}
	if _, _, err := parseLeftRightCount("garbage"); err == nil {
		t.Errorf("malformed rev-list output must error, not report 0 behind")
	}
}

func TestHasRemoteOrigin(t *testing.T) {
	if !hasRemoteOrigin("upstream\norigin\n") {
		t.Errorf("origin present but not detected")
	}
	if hasRemoteOrigin("upstream-origin\n") {
		t.Errorf("substring match wrongly detected origin")
	}
}

// scriptGit fakes git: replies are keyed by subcommand, and a subcommand in failing exits 1.
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

func TestDefaultProbe_InSync(t *testing.T) {
	scriptGit(t, healthyReplies(), nil)

	st, err := defaultBaseDivergenceProbe(context.Background(), "/repo")
	if err != nil || st.Skipped || st.Behind != 0 {
		t.Fatalf("got %+v err=%v; want in-sync non-skipped", st, err)
	}
}

func TestDefaultProbe_NotAWorkTree(t *testing.T) {
	scriptGit(t, healthyReplies(), map[string]bool{"rev-parse": true})

	st, err := defaultBaseDivergenceProbe(context.Background(), "/repo")
	if err != nil || !st.Skipped {
		t.Fatalf("got %+v err=%v; want skipped", st, err)
	}
}

func TestDefaultProbe_DetachedHead(t *testing.T) {
	r := healthyReplies()
	r["rev-parse"] = "HEAD"
	scriptGit(t, r, nil)

	st, err := defaultBaseDivergenceProbe(context.Background(), "/repo")
	if err != nil || !st.Skipped || !strings.Contains(st.Reason, "detached") {
		t.Fatalf("got %+v err=%v; want detached-HEAD skip", st, err)
	}
}

func TestDefaultProbe_NoOriginRemote(t *testing.T) {
	r := healthyReplies()
	r["remote"] = "upstream\n"
	scriptGit(t, r, nil)

	st, err := defaultBaseDivergenceProbe(context.Background(), "/repo")
	if err != nil || !st.Skipped || !strings.Contains(st.Reason, "origin") {
		t.Fatalf("got %+v err=%v; want no-origin skip", st, err)
	}
}

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

func TestDefaultProbe_RemoteListFailureErrors(t *testing.T) {
	scriptGit(t, healthyReplies(), map[string]bool{"remote": true})

	if _, err := defaultBaseDivergenceProbe(context.Background(), "/repo"); err == nil {
		t.Fatalf("remote-list failure must error")
	}
}

func TestDefaultProbe_RevListFailureErrors(t *testing.T) {
	scriptGit(t, healthyReplies(), map[string]bool{"rev-list": true})

	if _, err := defaultBaseDivergenceProbe(context.Background(), "/repo"); err == nil {
		t.Fatalf("rev-list failure must error")
	}
}

func TestDefaultProbe_MalformedRevListErrors(t *testing.T) {
	r := healthyReplies()
	r["rev-list"] = "not-a-count\n"
	scriptGit(t, r, nil)

	if _, err := defaultBaseDivergenceProbe(context.Background(), "/repo"); err == nil {
		t.Fatalf("malformed rev-list output must error")
	}
}

func TestDefaultProbe_BranchResolveFailureErrors(t *testing.T) {
	calls := 0
	orig := newGit
	newGit = func(dir string) gitexec.Git {
		return gitexec.Git{Dir: dir, Exec: func(_ context.Context, _, _ string, args, _ []string,
			_ io.Reader, stdout, stderr io.Writer) (int, error) {
			// The work-tree rev-parse succeeds; the --abbrev-ref HEAD one fails.
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
