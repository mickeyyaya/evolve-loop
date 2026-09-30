package gitexec

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func mixedFailRunner(attempts *int) sysexec.RunFunc {
	return func(ctx context.Context, name, dir string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if len(args) >= 2 && args[0] == "worktree" && args[1] == "add" {
			*attempts++
			if *attempts == 1 {
				return 255, nil
			}
			if stderr != nil {
				io.WriteString(stderr, "fatal: '/base/cycle-9' already exists\n")
			}
			return 128, nil
		}
		return 0, nil
	}
}

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
	if code != 128 {
		t.Errorf("exit code = %d, want the FINAL attempt's 128 (the terminal failure is what the caller fails on)", code)
	}
	if !strings.Contains(stderr, "already exists") {
		t.Errorf("terminal diagnostic lost git's own final stderr, got %q", stderr)
	}
	if !strings.Contains(stderr, "255") {
		t.Errorf("terminal diagnostic does not preserve the INITIATING rc=255 failure — a transient collision is being reported as a plain path collision; got %q", stderr)
	}
}

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
