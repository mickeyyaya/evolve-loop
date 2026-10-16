package pruneephemeral

import (
	"testing"
	"time"
)

func TestRun_ResultContract(t *testing.T) {
	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	staleEph := now.Add(-10 * 24 * time.Hour)
	staleLog := now.Add(-45 * 24 * time.Hour)
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
	if len(got.EphemeralPaths) != got.EphemeralPruned {
		t.Errorf("len(EphemeralPaths)=%d != EphemeralPruned=%d", len(got.EphemeralPaths), got.EphemeralPruned)
	}
	if len(got.LogPaths) != got.LogFilesPruned {
		t.Errorf("len(LogPaths)=%d != LogFilesPruned=%d", len(got.LogPaths), got.LogFilesPruned)
	}
}
