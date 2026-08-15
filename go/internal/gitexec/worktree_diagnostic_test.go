package gitexec

// worktree_diagnostic_test.go — cycle-1474 RED contract for
// `worktree-retry-diagnostic-integrity`.
//
// AddWorktreeWithRetry absorbs the live collision shape (rc=255, nothing on
// stderr but "Preparing worktree"), but its terminal diagnostic keeps ONLY the
// last attempt: worktree.go:101-126 overwrites (stdout, stderr, exitCode, err)
// on every pass of the loop. When a transient rc=255 is followed by a DIFFERENT
// terminal failure — the recorded SIGKILL/partial-directory shape, where the
// first attempt leaves a half-built directory and the second dies rc=128
// "already exists" — the initiating failure is erased and the operator reads a
// path-collision that never happened.
//
// The second defect is ordering: OnRetry is documented as "called before each
// RE-attempt" so a caller can ANNOUNCE contention, yet the loop sleeps the
// backoff ladder first (worktree.go:121-124) and announces afterwards. A lane
// that stalls inside the 2s/4s window therefore emits no contention line at
// all — the announcement arrives only if the sleep completes.
//
// Nothing here changes the retry BOUND, the permanent-failure classifier, or
// any caller signature: a persistent failure must still surface the FINAL exit
// code and git's own stderr (refuted PR #400 is the record of what silencing
// that costs).

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

// mixedFailRunner reproduces the two-failure sequence from the inbox record:
// attempt 1 is the transient collision (rc=255, EMPTY stderr — the shape that
// carries no diagnosis of its own), attempt 2 is a permanent rc=128 the first
// failure caused. Every attempt is counted.
func mixedFailRunner(attempts *int) sysexec.RunFunc {
	return func(ctx context.Context, name, dir string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if len(args) >= 2 && args[0] == "worktree" && args[1] == "add" {
			*attempts++
			if *attempts == 1 {
				return 255, nil // transient collision: rc only, no stderr
			}
			if stderr != nil {
				io.WriteString(stderr, "fatal: '/base/cycle-9' already exists\n")
			}
			return 128, nil
		}
		return 0, nil
	}
}

// TestAddWorktreeWithRetry_PreservesFirstFailure is the crux. Both the
// INITIATING and the TERMINAL failure must remain distinguishable in what the
// helper returns; today only the terminal one survives, so the rc=255 that
// actually started the incident is unrecoverable from the diagnostic.
func TestAddWorktreeWithRetry_PreservesFirstFailure(t *testing.T) {
	attempts := 0
	g := Git{Dir: "/repo", Exec: mixedFailRunner(&attempts)}

	_, stderr, code, _ := g.AddWorktreeWithRetry(context.Background(),
		WorktreeAddRetry{
			Sleep:     func(time.Duration) {},
			Retryable: RetryableWorktreeAddFailure,
		},
		"-B", "cycle-9", "/base/cycle-9", "HEAD")

	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2 (transient rc=255, then the permanent rc=128 that ends the loop)", attempts)
	}
	// The terminal failure must stay intact — the fail-fast alarm downstream
	// reads exactly this.
	if code != 128 {
		t.Errorf("exit code = %d, want the FINAL attempt's 128 (the terminal failure is what the caller fails on)", code)
	}
	if !strings.Contains(stderr, "already exists") {
		t.Errorf("terminal diagnostic lost git's own final stderr, got %q", stderr)
	}
	// ...and the initiating failure must still be recoverable from it. The
	// first attempt had NO stderr, so its exit code is the only evidence there
	// is: if 255 is absent, the masking defect is live.
	if !strings.Contains(stderr, "255") {
		t.Errorf("terminal diagnostic does not preserve the INITIATING rc=255 failure — a transient collision is being reported as a plain path collision; got %q", stderr)
	}
}

// TestAddWorktreeWithRetry_AnnouncesBeforeBackoff pins the documented order.
// OnRetry exists so a caller can announce contention while it is happening;
// announcing after the backoff makes the line useless for a lane that is stuck
// inside the ladder.
func TestAddWorktreeWithRetry_AnnouncesBeforeBackoff(t *testing.T) {
	failures, attempts := 1, 0
	var events []string
	g := Git{Dir: "/repo", Exec: addFailRunner(&failures, &attempts)}

	_, _, code, err := g.AddWorktreeWithRetry(context.Background(),
		WorktreeAddRetry{
			Sleep:   func(time.Duration) { events = append(events, "sleep") },
			OnRetry: func(_, _, _ int, _ string) { events = append(events, "retry") },
		},
		"-B", "cycle-9", "/base/cycle-9", "HEAD")

	if err != nil || code != 0 {
		t.Fatalf("one transient failure must still be absorbed, got code=%d err=%v", code, err)
	}
	want := []string{"retry", "sleep"}
	if len(events) != len(want) || events[0] != want[0] || events[1] != want[1] {
		t.Errorf("callback order = %v, want %v (announce the contention, THEN pay the backoff)", events, want)
	}
}

// The third acceptance criterion — "a permanent failure still performs one
// attempt and no backoff" — is already pinned by
// TestAddWorktreeWithRetry_PermanentFailureSkipsBackoff in
// worktree_retryable_test.go (pre-existing GREEN, asserts attempts==1,
// zero sleeps, zero announcements, rc + stderr intact). It is a regression
// guard for this task, not new coverage, so it is not restated here.

// TestAddWorktreeWithRetry_SuccessCarriesNoRetryHistory is the anti-gaming
// axis for PreservesFirstFailure: preserving evidence must be conditional on
// FAILING. A helper that unconditionally decorates its output would pollute
// every successful provision's stderr — and would pass PreservesFirstFailure
// while doing so.
func TestAddWorktreeWithRetry_SuccessCarriesNoRetryHistory(t *testing.T) {
	failures, attempts := 1, 0
	g := Git{Dir: "/repo", Exec: addFailRunner(&failures, &attempts)}

	_, stderr, code, err := g.AddWorktreeWithRetry(context.Background(),
		WorktreeAddRetry{Sleep: func(time.Duration) {}},
		"-B", "cycle-9", "/base/cycle-9", "HEAD")

	if err != nil || code != 0 {
		t.Fatalf("transient failure then success must return success, got code=%d err=%v", code, err)
	}
	if strings.Contains(stderr, "255") || strings.Contains(stderr, "Preparing worktree") {
		t.Errorf("a SUCCEEDING add must not carry the absorbed attempt's failure noise, got %q", stderr)
	}
}
