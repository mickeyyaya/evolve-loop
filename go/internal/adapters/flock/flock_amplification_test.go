package flock_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
)

func TestLockCreatesMissingParentsAndReleaseAllowsRelock(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "missing", "parents", "run.lock")

	release, err := flock.Lock(lockPath)
	if err != nil {
		t.Fatalf("Lock(%q) returned error: %v", lockPath, err)
	}

	if _, err := os.Stat(lockPath); err != nil {
		release()
		t.Fatalf("Lock(%q) did not create the lock file: %v", lockPath, err)
	}

	release()

	releaseAgain, err := flock.Lock(lockPath)
	if err != nil {
		t.Fatalf("Lock(%q) after release returned error: %v", lockPath, err)
	}
	releaseAgain()
}
