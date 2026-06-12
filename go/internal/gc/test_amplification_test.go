package gc

import (
	"path/filepath"
	"testing"
)

func TestAmplifyDiscover_DeduplicatesMarkerAndLedgerEvidence(t *testing.T) {
	dir := t.TempDir()
	run := filepath.Join(dir, "runs", "manual-release")
	writeFile(t, filepath.Join(run, "run.json"), `{"cycle_id":300}`)

	got, err := Discover(dir, DiscoverOptions{
		Now:        nowT0,
		LedgerRefs: []string{run, run},
	})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("duplicate marker/ledger evidence must yield one run, got %d: %+v", len(got), got)
	}
	if got[0].Path != run {
		t.Fatalf("discovered path = %q, want %q", got[0].Path, run)
	}
}

func TestAmplifyPlan_QuarantineSubtreeNeverTargeted(t *testing.T) {
	dir := t.TempDir()
	quarantinedRun := filepath.Join(dir, "quarantine", "nested", "cycle-77")
	keptRun := mkRun(t, dir, "cycle-300", daysAgo(1))

	m, err := Plan(Options{
		EvolveDir: dir,
		Policy:    Policy{Runs: RunsPolicy{KeepFull: 1, DeleteAfterDays: 1}},
		Runs: []RunDir{
			{Path: quarantinedRun, ModTime: daysAgo(1000)},
			keptRun,
		},
		Now: nowT0,
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	for _, it := range m.Items {
		if it.Path == quarantinedRun {
			t.Fatalf("quarantine subtree path must never be planned for mutation: %+v", it)
		}
	}
}
