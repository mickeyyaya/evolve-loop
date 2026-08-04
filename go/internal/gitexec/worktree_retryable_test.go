package gitexec

// worktree_retryable_test.go — cycle-1270 blocker (B-1/B-2/B-3).
//
// The retry loop slept the full 2s+4s ladder on ANY non-zero exit, including
// conditions no amount of waiting can change. Measured cost: 33 go/cmd/evolve
// tests reach this loop transitively over a t.TempDir() that is not a git
// repository — a permanent rc=128 — so the package paid 33 × 6s = 198s of pure
// backoff under a build floor that runs it with `-timeout 120s`. Deterministic,
// not a flake.
//
// Three axes, each load-bearing on its own:
//
//	Permanent → zero backoff        the fix
//	Transient → still rides the bound   the NEGATIVE guard: a "fix" that simply
//	                                    stopped retrying would pass the first
//	                                    test and re-break PR #401's absorber
//	Nil Retryable → unchanged       the zero value stays usable, so no existing
//	                                caller silently changes behaviour

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

// permanentAddRunner fails every `worktree add` with the live shape of the
// condition that cost cmd/evolve 198s: rc=128 and git's own fatal message.
func permanentAddRunner(attempts *int) sysexec.RunFunc {
	return func(ctx context.Context, name, dir string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if len(args) >= 2 && args[0] == "worktree" && args[1] == "add" {
			*attempts++
			if stderr != nil {
				io.WriteString(stderr, "fatal: not a git repository (or any of the parent directories): .git\n")
			}
			return 128, nil
		}
		return 0, nil
	}
}

func TestAddWorktreeWithRetry_PermanentFailureSkipsBackoff(t *testing.T) {
	attempts := 0
	var slept []time.Duration
	announced := 0
	g := Git{Dir: t.TempDir(), Exec: permanentAddRunner(&attempts)}

	_, stderr, code, err := g.AddWorktreeWithRetry(context.Background(), WorktreeAddRetry{
		Sleep:     func(d time.Duration) { slept = append(slept, d) },
		Retryable: RetryableWorktreeAddFailure,
		OnRetry:   func(attempt, attempts, code int, stderr string) { announced++ },
	}, "-B", "lane", t.TempDir(), "HEAD")

	if len(slept) != 0 {
		t.Errorf("slept %v on a PERMANENT failure — backoff bought on `not a git repository` is pure latency (33 cmd/evolve tests × 6s = 198s under a 120s floor timeout)", slept)
	}
	if attempts != 1 {
		t.Errorf("attempts=%d, want 1 — a permanent condition must return after the first invocation", attempts)
	}
	if announced != 0 {
		t.Errorf("announced %d retry line(s) for a failure that was never retried", announced)
	}
	// The fail-fast alarm chain stays armed: refuted PR #400 is the record of
	// what silencing it costs. Speed must come from not WAITING, never from not
	// reporting.
	if code != 128 || err != nil {
		t.Errorf("(code,err)=(%d,%v), want (128,nil) — the final exit code must survive intact", code, err)
	}
	if !strings.Contains(stderr, "not a git repository") {
		t.Errorf("stderr=%q — git's own message must be returned verbatim, not swallowed by the speed-up", stderr)
	}
}

func TestAddWorktreeWithRetry_TransientStillRetriesToBound(t *testing.T) {
	failures, attempts := 2, 0
	var slept []time.Duration
	g := Git{Dir: t.TempDir(), Exec: addFailRunner(&failures, &attempts)}

	_, _, code, err := g.AddWorktreeWithRetry(context.Background(), WorktreeAddRetry{
		Sleep:     func(d time.Duration) { slept = append(slept, d) },
		Retryable: RetryableWorktreeAddFailure,
	}, "-B", "lane", t.TempDir(), "HEAD")

	if code != 0 || err != nil {
		t.Fatalf("(code,err)=(%d,%v), want (0,nil) — an rc=255 lock collision must still be absorbed (PR #401)", code, err)
	}
	if attempts != 3 {
		t.Errorf("attempts=%d, want 3 — classifying must not shorten the bound for transients", attempts)
	}
	if len(slept) != 2 {
		t.Errorf("slept %v, want two backoffs — the collision absorber pays the ladder on purpose", slept)
	}
}

func TestAddWorktreeWithRetry_NilRetryablePreservesRetryEverything(t *testing.T) {
	attempts := 0
	var slept []time.Duration
	g := Git{Dir: t.TempDir(), Exec: permanentAddRunner(&attempts)}

	// Nil Retryable — the zero value every pre-existing caller had.
	_, _, code, _ := g.AddWorktreeWithRetry(context.Background(), WorktreeAddRetry{
		Sleep: func(d time.Duration) { slept = append(slept, d) },
	}, "-B", "lane", t.TempDir(), "HEAD")

	if attempts != DefaultWorktreeAddAttempts || len(slept) != DefaultWorktreeAddAttempts-1 {
		t.Errorf("attempts=%d slept=%v, want %d attempts and %d backoffs — a nil classifier must retry everything exactly as before, so adding the field changes no existing caller",
			attempts, slept, DefaultWorktreeAddAttempts, DefaultWorktreeAddAttempts-1)
	}
	if code != 128 {
		t.Errorf("code=%d, want 128", code)
	}
}

func TestRetryableWorktreeAddFailure_ClassifiesPermanentAndUnknown(t *testing.T) {
	cases := []struct {
		name   string
		code   int
		stderr string
		want   bool
	}{
		{"not a git repository", 128, "fatal: not a git repository (or any of the parent directories): .git", false},
		{"branch already checked out", 128, "fatal: 'lane' is already checked out at '/x'", false},
		{"destination exists", 128, "fatal: '/x' already exists", false},
		// The live incident shape: rc=255 with nothing but "Preparing worktree".
		{"lock collision", 255, "Preparing worktree (new branch 'lane')\n", true},
		// Unrecognised stays RETRYABLE by design. A misclassified transient
		// costs a lane its cycle; a misclassified permanent costs 6s.
		{"unknown failure", 1, "some future git message", true},
		{"empty stderr", 255, "", true},
	}
	for _, tc := range cases {
		if got := RetryableWorktreeAddFailure(tc.code, tc.stderr); got != tc.want {
			t.Errorf("%s: RetryableWorktreeAddFailure(%d, %q) = %v, want %v", tc.name, tc.code, tc.stderr, got, tc.want)
		}
	}
}
