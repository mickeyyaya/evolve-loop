package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/shipwindow"
)

func TestShipWindowWiring_AuditAcquiresNonAuditReleases(t *testing.T) {
	root := t.TempDir()
	cr := &cycleRun{ctx: context.Background(), req: CycleRequest{ProjectRoot: root}}
	lockPath := shipwindow.PathIn(filepath.Join(root, ".evolve"))

	cr.acquireShipWindow(PhaseBuild)
	if cr.shipLease != nil {
		t.Fatalf("acquireShipWindow(PhaseBuild) took the lease; want audit-only")
	}

	cr.acquireShipWindow(PhaseAudit)
	if cr.shipLease == nil {
		t.Fatalf("acquireShipWindow(PhaseAudit) did not take the lease")
	}
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("lease file missing while held: %v", err)
	}

	cr.acquireShipWindow(PhaseAudit)
	if cr.shipLease == nil {
		t.Fatalf("re-audit dropped the lease; want re-acquired")
	}

	cr.releaseShipWindow()
	if cr.shipLease != nil {
		t.Fatalf("releaseShipWindow left shipLease non-nil")
	}
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("lease file still present after release (stat err=%v)", err)
	}
	cr.releaseShipWindow()
}

func TestShipWindowWiring_FailOpenWhenSiblingHolds(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	sib, err := shipwindow.Acquire(context.Background(), evolveDir, shipwindow.Options{})
	if err != nil {
		t.Fatalf("sibling Acquire: %v", err)
	}
	defer func() {
		if err := sib.Release(); err != nil {
			t.Errorf("sibling Release: %v", err)
		}
	}()

	old := shipWindowAcquireTimeout
	shipWindowAcquireTimeout = 100 * time.Millisecond
	defer func() { shipWindowAcquireTimeout = old }()

	cr := &cycleRun{ctx: context.Background(), req: CycleRequest{ProjectRoot: root}}
	cr.acquireShipWindow(PhaseAudit)
	if cr.shipLease != nil {
		t.Fatalf("lane acquired the lease while a live sibling held it; want fail-open unleased")
	}
}
