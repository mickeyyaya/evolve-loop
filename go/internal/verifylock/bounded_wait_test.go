package verifylock

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/addedtests"
)

func TestAcquireWithin_AWaiterBehindAHeldLockGivesUpAtTheBoundAndSaysSo(t *testing.T) {
	root := t.TempDir()
	hold, err := Acquire(context.Background(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer hold()

	start := time.Now()
	release, err := AcquireWithin(context.Background(), root, 100*time.Millisecond, nil)
	waited := time.Since(start)

	if err == nil {
		release()
		t.Fatal("a waiter behind a held lock must give up at its bound, not take the lock")
	}
	if waited > 5*time.Second {
		t.Fatalf("waited %s for a 100ms bound; the bound must end the wait", waited)
	}
	if !strings.Contains(err.Error(), "100ms") || !strings.Contains(err.Error(), "unserialized") {
		t.Errorf("error %q must name the bound and say the run goes unserialized, so the caller's WARN is self-explaining", err)
	}
}

func TestAcquireWithin_AFreeLockIsTakenAndReleased(t *testing.T) {
	root := t.TempDir()
	release, err := AcquireWithin(context.Background(), root, MaxWait, nil)
	if err != nil {
		t.Fatalf("a free lock must be taken: %v", err)
	}
	release()
	again, err := AcquireWithin(context.Background(), root, 100*time.Millisecond, nil)
	if err != nil {
		t.Fatalf("the lock must be free again after release: %v", err)
	}
	again()
}

func TestAcquireWithin_TheCallersOwnDeadlineIsNotReportedAsTheBound(t *testing.T) {
	root := t.TempDir()
	hold, err := Acquire(context.Background(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer hold()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err = AcquireWithin(ctx, root, MaxWait, nil)

	if err == nil || strings.Contains(err.Error(), "wait bound") {
		t.Errorf("a caller whose own deadline passed must get its own deadline error, not a claim that the %s wait bound passed; got %v", MaxWait, err)
	}
}

func TestMaxWait_EndsBeforeAHungHolderUsesItsWholeGoTestBudget(t *testing.T) {
	hold, err := time.ParseDuration(addedtests.PackageTimeout)
	if err != nil {
		t.Fatal(err)
	}
	if MaxWait <= 0 || MaxWait >= hold {
		t.Errorf("MaxWait = %s; it must be positive and shorter than the %s a holder may run under, so a waiter behind a hung holder degrades to unserialized instead of waiting out the hang", MaxWait, hold)
	}
}

func TestAcquireWithin_ResolvesTheLockAsAcquireDoes(t *testing.T) {
	root := t.TempDir()
	hold, err := Acquire(context.Background(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer hold()
	var warn bytes.Buffer

	_, _ = AcquireWithin(context.Background(), root, 100*time.Millisecond, &warn)

	if !strings.Contains(warn.String(), "PER-WORKTREE") {
		t.Errorf("AcquireWithin must resolve the lock exactly as Acquire does (a temp dir has no hub, so Acquire warns PER-WORKTREE); warn = %q", warn.String())
	}
}
