package gitexec

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const refLockStderr = "error: cannot lock ref 'refs/remotes/origin/main': is at e1d9ae9e but expected 0bc8caed\n" +
	" ! 0bc8caed0..e1d9ae9e5  main -> origin/main  (unable to update local ref)\n"

type fetchCall struct{ args []string }

func scriptedFetchRunner(calls *[]fetchCall, script []struct {
	code   int
	stderr string
}) sysexec.RunFunc {
	return func(ctx context.Context, name, dir string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		*calls = append(*calls, fetchCall{args: append([]string(nil), args...)})
		step := script[min(len(*calls), len(script))-1]
		if stderr != nil {
			io.WriteString(stderr, step.stderr)
		}
		return step.code, nil
	}
}

func TestFetchOriginBranch_FetchesOnlyTheBranchRefspec(t *testing.T) {
	var calls []fetchCall
	g := Git{Dir: "/repo", Exec: scriptedFetchRunner(&calls, []struct {
		code   int
		stderr string
	}{{0, ""}})}

	_, _, code, err := g.FetchOriginBranch(context.Background(), FetchRetry{Sleep: func(time.Duration) {}}, "main")

	if err != nil || code != 0 {
		t.Fatalf("clean fetch: code=%d err=%v", code, err)
	}
	want := "fetch origin +refs/heads/main:refs/remotes/origin/main"
	if len(calls) != 1 || strings.Join(calls[0].args, " ") != want {
		t.Fatalf("argv = %v, want exactly one %q: a bare `git fetch origin` updates every remote-tracking ref, so contention on any unrelated ref fails the caller", calls, want)
	}
}

func TestFetchOriginBranch_RetriesRefLockContentionThenSucceeds(t *testing.T) {
	var calls []fetchCall
	var slept []time.Duration
	var announced []string
	g := Git{Dir: "/repo", Exec: scriptedFetchRunner(&calls, []struct {
		code   int
		stderr string
	}{{1, refLockStderr}, {0, ""}})}

	_, _, code, err := g.FetchOriginBranch(context.Background(), FetchRetry{
		Sleep: func(d time.Duration) { slept = append(slept, d) },
		OnRetry: func(attempt, attempts, code int, stderr string) {
			announced = append(announced, stderr)
		},
	}, "main")

	if err != nil || code != 0 {
		t.Fatalf("one ref-lock contention must be absorbed: code=%d err=%v", code, err)
	}
	if len(calls) != 2 || len(slept) != 1 || len(announced) != 1 {
		t.Fatalf("attempts=%d sleeps=%v announcements=%d, want 2 attempts, one backoff, one announcement", len(calls), slept, len(announced))
	}
	if !strings.Contains(announced[0], "cannot lock ref") {
		t.Errorf("the announcement carries the contended attempt's stderr, got %q", announced[0])
	}
}

func TestFetchOriginBranch_PersistentContentionFailsAfterTheBound(t *testing.T) {
	var calls []fetchCall
	var slept []time.Duration
	g := Git{Dir: "/repo", Exec: scriptedFetchRunner(&calls, []struct {
		code   int
		stderr string
	}{{1, refLockStderr}})}

	_, stderr, code, err := g.FetchOriginBranch(context.Background(), FetchRetry{
		Attempts: 4,
		Sleep:    func(d time.Duration) { slept = append(slept, d) },
	}, "main")

	if code != 1 || err != nil {
		t.Fatalf("persistent contention surfaces the final exit code through code, got code=%d err=%v", code, err)
	}
	if len(calls) != 4 || len(slept) != 3 {
		t.Fatalf("attempts=%d sleeps=%d, want the Attempts seam's 4 attempts and 3 backoffs", len(calls), len(slept))
	}
	if !strings.Contains(stderr, "initial fetch failure (rc=1)") || !strings.Contains(stderr, "final fetch failure:") {
		t.Errorf("the diagnostic keeps the first and the final failure, got %q", stderr)
	}
}

func TestFetchOriginBranch_ZeroAttemptsUsesDefaultFetchAttempts(t *testing.T) {
	var calls []fetchCall
	g := Git{Dir: "/repo", Exec: scriptedFetchRunner(&calls, []struct {
		code   int
		stderr string
	}{{1, refLockStderr}})}

	g.FetchOriginBranch(context.Background(), FetchRetry{Sleep: func(time.Duration) {}}, "main")

	if DefaultFetchAttempts < 2 || len(calls) != DefaultFetchAttempts {
		t.Fatalf("attempts=%d, want DefaultFetchAttempts=%d (>= 2: a bound of 1 is no retry)", len(calls), DefaultFetchAttempts)
	}
}

func TestFetchOriginBranch_NonContentionFailureIsNotRetried(t *testing.T) {
	var calls []fetchCall
	slept := 0
	g := Git{Dir: "/repo", Exec: scriptedFetchRunner(&calls, []struct {
		code   int
		stderr string
	}{{128, "fatal: couldn't find remote ref main\n"}})}

	_, stderr, code, _ := g.FetchOriginBranch(context.Background(), FetchRetry{Sleep: func(time.Duration) { slept++ }}, "main")

	if code != 128 || len(calls) != 1 || slept != 0 {
		t.Fatalf("code=%d attempts=%d sleeps=%d, want one attempt, no backoff: only ref-lock contention is transient", code, len(calls), slept)
	}
	if stderr != "fatal: couldn't find remote ref main\n" {
		t.Errorf("an unretried failure returns git's stderr untouched, got %q", stderr)
	}
}

func TestRetryableFetchFailure_ClassifiesRefLockContention(t *testing.T) {
	cases := []struct {
		name, stderr string
		want         bool
	}{
		{"moved ref", "error: cannot lock ref 'refs/remotes/origin/train/x': is at e1d9 but expected 0bc8", true},
		{"lock file held", "error: Cannot lock ref 'refs/remotes/origin/main': Unable to create '/s/refs/remotes/origin/main.lock': File exists.", true},
		{"update line alone", " ! 0bc8..e1d9  main -> origin/main  (unable to update local ref)", true},
		{"an unrelated 'but expected'", "error: inflate: data stream error (incorrect header check) but expected a pack", false},
		{"missing remote ref", "fatal: couldn't find remote ref nope", false},
		{"network", "fatal: unable to access 'https://github.com/x/y/': Could not resolve host: github.com", false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		if got := RetryableFetchFailure(1, tc.stderr); got != tc.want {
			t.Errorf("%s: RetryableFetchFailure(%q) = %v, want %v", tc.name, tc.stderr, got, tc.want)
		}
	}
}
