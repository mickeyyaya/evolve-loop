package runlease

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLiveRuns_OnlyFreshLeasesOfDirectoriesAreReportedSortedByDirectory(t *testing.T) {
	runs := t.TempDir()
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	mk := func(name string, heartbeat time.Time) string {
		dir := filepath.Join(runs, name)
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := Write(dir, Lease{RunID: "run-" + name, OwnerPID: os.Getpid()}, heartbeat); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	b := mk("b-live", now.Add(-time.Minute))
	a := mk("a-live", now.Add(-2*time.Minute))
	mk("c-stale", now.Add(-2*DefaultTTL))
	if err := os.Mkdir(filepath.Join(runs, "d-no-lease"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runs, "e-plain-file"), []byte("not a run dir"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := LiveRuns(runs, now)

	if len(got) != 2 || got[0].Dir != a || got[1].Dir != b {
		t.Fatalf("LiveRuns = %+v, want [%s %s] in that order", got, a, b)
	}
	if got[0].Lease.RunID != "run-a-live" || got[1].Lease.RunID != "run-b-live" {
		t.Errorf("leases = %+v", got)
	}
	if none := LiveRuns(filepath.Join(runs, "absent"), now); none != nil {
		t.Errorf("a missing runs dir has no live runs: %+v", none)
	}
}
