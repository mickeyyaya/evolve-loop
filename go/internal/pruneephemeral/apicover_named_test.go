package pruneephemeral

import (
	"testing"
	"time"
)

// TestRun_ResultContract names the pruneephemeral.Result type (returned by Run
// but never named in a test) and pins the dry-run auditing contract: the prune
// counters and their matching audit-path slices stay in lockstep.
func TestRun_ResultContract(t *testing.T) {
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	staleEph := now.Add(-10 * 24 * time.Hour) // older than the 7d tracker TTL
	staleLog := now.Add(-45 * 24 * time.Hour) // older than the 30d dispatch-log TTL
	repo := makeRepo(t,
		[]cycleSpec{{"cycle-1", staleEph}},
		[]logSpec{{"batch-old.log", staleLog}},
	)

	got, err := Run(Options{
		ProjectRoot: repo,
		DryRun:      true,
		Now:         func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("Run err = %v", err)
	}

	want := Result{EphemeralPruned: 1, LogFilesPruned: 1}
	if got.EphemeralPruned != want.EphemeralPruned || got.LogFilesPruned != want.LogFilesPruned {
		t.Errorf("Run() counts = (eph=%d log=%d), want (1,1)", got.EphemeralPruned, got.LogFilesPruned)
	}
	// Contract: in dry-run each counted entry is also recorded in the audit slice.
	if len(got.EphemeralPaths) != got.EphemeralPruned {
		t.Errorf("len(EphemeralPaths)=%d != EphemeralPruned=%d", len(got.EphemeralPaths), got.EphemeralPruned)
	}
	if len(got.LogPaths) != got.LogFilesPruned {
		t.Errorf("len(LogPaths)=%d != LogFilesPruned=%d", len(got.LogPaths), got.LogFilesPruned)
	}
}
