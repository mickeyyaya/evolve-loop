package dashboard

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

// seedProject builds a small but complete project root: three closed cycles
// (PASS, FAIL, PASS — the last with a workspace), one in-flight cycle with a
// fresh lease, a quarantined duplicate workspace and an archive dir to ignore,
// and two inbox items.
func seedProject(t *testing.T, now time.Time) string {
	t.Helper()
	root := t.TempDir()
	writeDossier(t, root, passDossier(1))
	writeDossier(t, root, failDossier(2, "audit|gate-block|aaaa"))
	writeDossier(t, root, passDossier(3))
	writeWorkspace(t, root, 3, []phasetiming.Entry{
		entry("build", "PASS", "2026-09-01T21:40:00Z", "2026-09-01T21:45:00Z", 5),
		entry("audit", "FAIL", "2026-09-01T22:00:00Z", "2026-09-01T22:30:00Z", 5),
		entry("audit", "PASS", "2026-09-01T23:00:00Z", "2026-09-01T23:30:00Z", 5),
	}, 2)
	ws4 := writeCycleState(t, root, cyclestate.CycleState{CycleID: 4, Phase: "build",
		PhaseStartedAt: now.Add(-2 * time.Minute).Format(time.RFC3339)})
	if err := runlease.Write(ws4, runlease.Lease{RunID: "r4"}, now); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"cycle-3.polluted-20260901T211450", "archive"} {
		if err := os.MkdirAll(filepath.Join(root, ".evolve", "runs", d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	inbox := filepath.Join(root, ".evolve", "inbox")
	writeInboxItem(t, inbox, "a.json", `{"id":"a","title":"A","weight":0.9}`)
	writeInboxItem(t, inbox, "b.json", `{"id":"b","title":"B","weight":0.8}`)
	return root
}

func TestCollect_WholePicture(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	root := seedProject(t, now)

	snap := Collect(root, now)
	if len(snap.Warnings) != 0 {
		t.Fatalf("warnings: %v", snap.Warnings)
	}
	if !snap.Loop.Running || snap.Loop.CycleID != 4 || snap.Loop.Phase != "build" {
		t.Fatalf("loop = %+v", snap.Loop)
	}
	ids := []int{}
	states := []string{}
	for _, c := range snap.Cycles {
		ids = append(ids, c.ID)
		states = append(states, c.State)
	}
	if len(ids) != 4 || ids[0] != 4 || ids[1] != 3 || ids[2] != 2 || ids[3] != 1 {
		t.Fatalf("cycle ids = %v, want newest first 4,3,2,1 (no polluted/archive)", ids)
	}
	want := []string{StateRunning, StatePass, StateFail, StatePass}
	for i := range want {
		if states[i] != want[i] {
			t.Fatalf("states = %v, want %v", states, want)
		}
	}
	if snap.Cycles[0].CurrentPhase != "build" || !snap.Cycles[1].HasWorkspace || snap.Cycles[1].AuditRounds != 2 {
		t.Fatalf("cycle detail = %+v / %+v", snap.Cycles[0], snap.Cycles[1])
	}
	if snap.Cycles[2].Failure == nil || snap.Cycles[2].Failure.Fingerprint != "audit|gate-block|aaaa" {
		t.Fatalf("FAIL cycle carries its dossier failure: %+v", snap.Cycles[2].Failure)
	}
	if snap.Trend.Closed != 3 || snap.Trend.Shipped != 2 || len(snap.Fingerprints) != 1 {
		t.Fatalf("trend = %+v fps = %+v", snap.Trend, snap.Fingerprints)
	}
	if len(snap.Trend.RoundHistogram) != 1 || snap.Trend.RoundHistogram[0] != (RoundBucket{Rounds: 2, Cycles: 1, Shipped: 1}) {
		t.Fatalf("round histogram = %+v (only cycle 3 has both workspace and dossier)", snap.Trend.RoundHistogram)
	}
	if len(snap.Queue.Pending) != 2 || snap.Queue.Pending[0].ID != "a" {
		t.Fatalf("queue = %+v", snap.Queue)
	}
	if snap.Root != root || !snap.GeneratedAt.Equal(now) {
		t.Fatalf("envelope = %s %v", snap.Root, snap.GeneratedAt)
	}
}

func TestCollect_CapKeepsNewestDossiersAndEveryWorkspace(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for i := 1; i <= 60; i++ {
		writeDossier(t, root, passDossier(i))
	}
	// An OLD cycle with a workspace must survive the cap.
	if err := os.MkdirAll(core.RunWorkspacePath(root, 2), 0o755); err != nil {
		t.Fatal(err)
	}
	c := newCollector(root)
	c.maxCycles = 10
	snap, _ := c.collect(time.Now())
	if len(snap.Cycles) != 10 || snap.Cycles[0].ID != 60 {
		t.Fatalf("cap: %d cycles, first %d", len(snap.Cycles), snap.Cycles[0].ID)
	}
	found := false
	for _, cs := range snap.Cycles {
		if cs.ID == 2 && cs.HasWorkspace {
			found = true
		}
	}
	if !found {
		t.Fatalf("workspace cycle 2 dropped by the cap")
	}
	if snap.Trend.Closed != 60 {
		t.Fatalf("trend must cover ALL dossiers, not the capped list: %d", snap.Trend.Closed)
	}
}

func TestCollect_EmptyRootIsQuiet(t *testing.T) {
	t.Parallel()
	snap := Collect(t.TempDir(), time.Now())
	if len(snap.Cycles) != 0 || len(snap.Warnings) != 0 || snap.Loop.Running {
		t.Fatalf("empty root: %+v", snap)
	}
}
