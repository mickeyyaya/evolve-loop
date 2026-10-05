package acssuite

import (
	"context"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/verifylock"
)

func TestAcquireSuiteLock_BehindAHeldLockWaitsOnlyItsBoundThenRunsUnserialized(t *testing.T) {
	root := t.TempDir()
	hold, err := verifylock.Acquire(context.Background(), root, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer hold()
	saved := suiteLockWait
	suiteLockWait = 200 * time.Millisecond
	defer func() { suiteLockWait = saved }()

	done := make(chan func(), 1)
	go func() { done <- acquireSuiteLock(root) }()

	select {
	case release := <-done:
		release()
	case <-time.After(30 * time.Second):
		t.Fatal("the suite waited past its 200ms bound behind a held lock; a waiter must degrade to unserialized at the bound, never wait out the holder")
	}
}

func TestSuiteLockWait_IsTheSharedVerificationWaitBound(t *testing.T) {
	if suiteLockWait != verifylock.MaxWait {
		t.Errorf("suiteLockWait = %s, want verifylock.MaxWait (%s): the ACS suite and the build floor's coverage pass wait for the host lock under one bound", suiteLockWait, verifylock.MaxWait)
	}
}
