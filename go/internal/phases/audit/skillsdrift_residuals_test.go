package audit

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"
)

func shrinkWorktreeSkillsCheckLimit(t *testing.T, limit time.Duration) {
	t.Helper()
	orig := worktreeSkillsCheckLimit
	worktreeSkillsCheckLimit = limit
	t.Cleanup(func() { worktreeSkillsCheckLimit = orig })
}

func TestWorktreeSkillsDrift_Exit2AtDeadlineWithDriftIsOffender(t *testing.T) {
	cases := []struct {
		name      string
		runnerErr func(ctx context.Context) error
	}{
		{"the run returns no error", func(context.Context) error { return nil }},
		{"the run returns the deadline error", func(ctx context.Context) error { return ctx.Err() }},
	}
	wantOffenders := []string{"commands/scout.md"}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			shrinkWorktreeSkillsCheckLimit(t, 20*time.Millisecond)
			withFakeRunner(t, func(ctx context.Context, _, _ string, _, _ []string, _ io.Reader, _, stderr io.Writer) (int, error) {
				<-ctx.Done()
				fmt.Fprint(stderr, "DRIFT: commands/scout.md is stale or missing (run `evolve skills generate`)\nexit status 2\n")
				return 2, tc.runnerErr(ctx)
			})

			got, err := worktreeSkillsDrift(context.Background(), t.TempDir())

			if err != nil || !reflect.DeepEqual(got, wantOffenders) {
				t.Fatalf("exit 2 with a drift report at the deadline = (%q, %v), want the offenders %q and no error", got, err, wantOffenders)
			}
		})
	}
}

func TestWorktreeSkillsDrift_DeadlineWithoutADriftReportStaysUngraded(t *testing.T) {
	shrinkWorktreeSkillsCheckLimit(t, 20*time.Millisecond)
	withFakeRunner(t, func(ctx context.Context, _, _ string, _, _ []string, _ io.Reader, _, stderr io.Writer) (int, error) {
		<-ctx.Done()
		fmt.Fprint(stderr, "go: downloading example.com/m v1.0.0\n")
		return -1, ctx.Err()
	})

	got, err := worktreeSkillsDrift(context.Background(), t.TempDir())

	if got != nil || err == nil || !strings.Contains(err.Error(), "is NOT graded") || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("a killed run with no drift report = (%q, %v), want a NOT-graded warning that wraps the deadline", got, err)
	}
}

func TestWorktreeSkillsDrift_ContextCancellationStopsRun(t *testing.T) {
	shrinkWorktreeSkillsCheckLimit(t, time.Hour)
	phaseCtx, cancelPhase := context.WithCancel(context.Background())
	defer cancelPhase()
	var runCtxErr error
	withFakeRunner(t, func(ctx context.Context, _, _ string, _, _ []string, _ io.Reader, _, _ io.Writer) (int, error) {
		cancelPhase()
		select {
		case <-ctx.Done():
			runCtxErr = ctx.Err()
			return -1, ctx.Err()
		default:
			return -1, errors.New("the worktree run did not observe the cancelled phase context")
		}
	})

	got, err := worktreeSkillsDrift(phaseCtx, t.TempDir())

	if !errors.Is(runCtxErr, context.Canceled) {
		t.Fatalf("the worktree run context error = %v, want context.Canceled from the phase context", runCtxErr)
	}
	if got != nil || err == nil || !strings.Contains(err.Error(), "is NOT graded") || !errors.Is(err, context.Canceled) {
		t.Fatalf("a cancelled phase = (%q, %v), want a NOT-graded warning that wraps context.Canceled", got, err)
	}
}

func TestWorktreeSkillsDrift_CancelledPhaseIsNotADeadline(t *testing.T) {
	shrinkWorktreeSkillsCheckLimit(t, time.Hour)
	phaseCtx, cancelPhase := context.WithCancel(context.Background())
	cancelPhase()
	withFakeRunner(t, func(ctx context.Context, _, _ string, _, _ []string, _ io.Reader, _, _ io.Writer) (int, error) {
		if ctx.Err() == nil {
			return -1, errors.New("the worktree run started under a live context although the phase was already cancelled")
		}
		return -1, ctx.Err()
	})

	_, err := worktreeSkillsDrift(phaseCtx, t.TempDir())

	if errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, context.Canceled) {
		t.Fatalf("a phase cancelled before the run = %v, want context.Canceled and not a deadline", err)
	}
}

func TestWorktreeSkillsDrift_MissingGoBinaryWarns(t *testing.T) {
	missingGo := &exec.Error{Name: "go", Err: exec.ErrNotFound}
	withFakeRunner(t, func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
		return -1, missingGo
	})

	got, err := worktreeSkillsDrift(context.Background(), t.TempDir())

	var notice gateWarning
	if errors.As(err, &notice) {
		t.Fatalf("a missing go binary = %v, want a could-not-run error, not a generator-disagreement notice", err)
	}
	if got != nil || err == nil || !strings.Contains(err.Error(), "did not run") || !strings.Contains(err.Error(), "is NOT graded") || !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("a missing go binary = (%q, %v), want a NOT-graded warning that names the missing executable", got, err)
	}
}
