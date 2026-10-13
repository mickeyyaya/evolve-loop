package treedelta

import (
	"context"
	"errors"
	"testing"
)

func TestDelta_AGitRunFailureIsReturnedWhateverTheExitCodeSays(t *testing.T) {
	boom := errors.New("git not runnable")
	git := func(context.Context, string, ...string) (string, int, error) { return "ignored", 0, boom }

	out, err := Delta(context.Background(), git, "/repo", "base", "tree")

	if !errors.Is(err, boom) || out != nil {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestIdentical_EitherDeltaFailingStopsTheComparison(t *testing.T) {
	boom := errors.New("second diff failed")
	calls := 0
	git := func(_ context.Context, _ string, args ...string) (string, int, error) {
		calls++
		if calls == 2 {
			return "", 0, boom
		}
		return "diff --git a/f b/f\n", 0, nil
	}

	audited, composed, ok, err := Identical(context.Background(), git, "/repo", "b0", "t0", "b1", "t1")

	if !errors.Is(err, boom) || ok || audited != nil || composed != nil {
		t.Fatalf("audited=%q composed=%q ok=%v err=%v: a failed second delta must not return the first", audited, composed, ok, err)
	}
	if calls != 2 {
		t.Errorf("both deltas are attempted in order, got %d call(s)", calls)
	}
}

func TestIdentical_EqualNonEmptyDeltasAreIdenticalAndEmptyOnesAreNot(t *testing.T) {
	with := func(out string) Git {
		return func(context.Context, string, ...string) (string, int, error) { return out, 0, nil }
	}
	_, _, ok, err := Identical(context.Background(), with("diff\n"), "/r", "a", "b", "c", "d")
	if err != nil || !ok {
		t.Errorf("equal non-empty deltas: ok=%v err=%v", ok, err)
	}
	_, _, ok, err = Identical(context.Background(), with(""), "/r", "a", "b", "c", "d")
	if err != nil || ok {
		t.Errorf("an empty delta proves nothing: ok=%v err=%v", ok, err)
	}
}
