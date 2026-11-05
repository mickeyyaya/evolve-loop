package wave_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/landing"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/shipjournal"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

var shipEpoch = time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)

func stamp(at time.Time) string { return at.UTC().Format(time.RFC3339) }

func journalLine(t *testing.T, e shipjournal.Entry) string {
	t.Helper()
	raw, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw) + "\n"
}

func writeJournalLines(t *testing.T, root string, lines ...string) {
	t.Helper()
	path := shipjournal.Path(filepath.Join(root, ".evolve"))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "")), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeLanding(t *testing.T, root string, in landing.Intent) {
	t.Helper()
	if err := landing.WriteIntent(landing.IntentPath(filepath.Join(root, ".evolve"), in.Cycle), in); err != nil {
		t.Fatal(err)
	}
}

func landedCycle(t *testing.T, root string, cycle int, sha string, journalAt time.Time, completedAt string) {
	t.Helper()
	writeJournalLines(t, root, journalLine(t, shipjournal.Entry{SHA: sha, Class: shipjournal.ClassCycle, TS: stamp(journalAt), Cycle: cycle}))
	writeLanding(t, root, landing.Intent{Cycle: cycle, CommitSHA: sha, Status: landing.IntentComplete, CompletedAt: completedAt})
}

func shadowShips(root string, from, to time.Time) ([]int, []wave.ReadFailure) {
	return wave.ShipsIn(root, policy.FleetIntegrationShadow, from, to)
}

func TestShipsIn_ACycleEntryJoinedToACompleteLandingOfTheSameSHACounts(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	landedCycle(t, root, 7, "aaa", shipEpoch.Add(time.Minute), "")

	ids, fails := shadowShips(root, shipEpoch, shipEpoch.Add(time.Hour))

	if !reflect.DeepEqual(ids, []int{7}) || len(fails) != 0 {
		t.Fatalf("ids %v, failures %v; want [7] and none", ids, fails)
	}
}

func TestShipsIn_AJournalEntryWithoutACompleteMatchingLandingCountsNothing(t *testing.T) {
	t.Parallel()
	cases := map[string]landing.Intent{
		"prepared only":   {Cycle: 7, CommitSHA: "aaa", Status: landing.IntentPrepared},
		"another SHA":     {Cycle: 7, CommitSHA: "bbb", Status: landing.IntentComplete},
		"no landing file": {},
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writeJournalLines(t, root, journalLine(t, shipjournal.Entry{SHA: "aaa", Class: shipjournal.ClassCycle, TS: stamp(shipEpoch.Add(time.Minute)), Cycle: 7}))
			if in.Cycle != 0 {
				writeLanding(t, root, in)
			}
			ids, fails := shadowShips(root, shipEpoch, shipEpoch.Add(time.Hour))
			if len(ids) != 0 || len(fails) != 0 {
				t.Fatalf("ids %v, failures %v; want none", ids, fails)
			}
		})
	}
}

func TestShipsIn_OnlyAnEntryOfTheCycleClassWithACycleCounts(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeJournalLines(t, root,
		journalLine(t, shipjournal.Entry{SHA: "aaa", Class: "manual", TS: stamp(shipEpoch.Add(time.Minute)), Cycle: 7}),
		journalLine(t, shipjournal.Entry{SHA: "bbb", Class: shipjournal.ClassCycle, TS: stamp(shipEpoch.Add(time.Minute))}),
	)
	writeLanding(t, root, landing.Intent{Cycle: 7, CommitSHA: "aaa", Status: landing.IntentComplete})
	writeLanding(t, root, landing.Intent{Cycle: 0, CommitSHA: "bbb", Status: landing.IntentComplete})

	ids, _ := shadowShips(root, shipEpoch, shipEpoch.Add(time.Hour))

	if len(ids) != 0 {
		t.Fatalf("ids %v, want none", ids)
	}
}

func TestShipsIn_AnUnwoundAttemptAndALandedAttemptOfOneCycleCountOnceInTheWindowOfTheLanded(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeJournalLines(t, root,
		journalLine(t, shipjournal.Entry{SHA: "unwound", Class: shipjournal.ClassCycle, TS: stamp(shipEpoch.Add(time.Minute)), Cycle: 7}),
		journalLine(t, shipjournal.Entry{SHA: "landed", Class: shipjournal.ClassCycle, TS: stamp(shipEpoch.Add(2 * time.Hour)), Cycle: 7}),
	)
	writeLanding(t, root, landing.Intent{Cycle: 7, CommitSHA: "landed", Status: landing.IntentComplete, CompletedAt: stamp(shipEpoch.Add(2*time.Hour + time.Minute))})

	first, _ := shadowShips(root, shipEpoch, shipEpoch.Add(time.Hour))
	second, _ := shadowShips(root, shipEpoch.Add(time.Hour), shipEpoch.Add(3*time.Hour))
	all, _ := shadowShips(root, shipEpoch, time.Time{})

	if len(first) != 0 || !reflect.DeepEqual(second, []int{7}) || !reflect.DeepEqual(all, []int{7}) {
		t.Fatalf("first %v, second %v, all %v; want none, [7], [7]", first, second, all)
	}
}

func TestShipsIn_ALandingPreparedInOneWaveAndCompletedInTheNextCountsInTheNext(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	landedCycle(t, root, 7, "aaa", shipEpoch.Add(time.Minute), stamp(shipEpoch.Add(time.Hour+time.Minute)))

	early, _ := shadowShips(root, shipEpoch, shipEpoch.Add(time.Hour))
	late, _ := shadowShips(root, shipEpoch.Add(time.Hour), shipEpoch.Add(2*time.Hour))

	if len(early) != 0 || !reflect.DeepEqual(late, []int{7}) {
		t.Fatalf("early %v, late %v; want none then [7]", early, late)
	}
}

func TestShipsIn_APastRecordWithNoCompletionTimeUsesTheJournalTime(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	landedCycle(t, root, 7, "aaa", shipEpoch.Add(time.Minute), "")

	in, _ := shadowShips(root, shipEpoch, shipEpoch.Add(time.Hour))
	out, _ := shadowShips(root, shipEpoch.Add(time.Hour), shipEpoch.Add(2*time.Hour))

	if !reflect.DeepEqual(in, []int{7}) || len(out) != 0 {
		t.Fatalf("in %v, out %v; want [7] then none", in, out)
	}
}

func TestShipsIn_TheWindowHoldsFromAndNotToAtTheSecond(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	landedCycle(t, root, 7, "aaa", shipEpoch, "")
	from, to := shipEpoch, shipEpoch.Add(10*time.Second)

	atFrom, _ := shadowShips(root, from, to)
	subSecondFrom, _ := shadowShips(root, from.Add(900*time.Millisecond), to)
	justBefore, _ := shadowShips(root, from.Add(time.Second), to)
	atTo, _ := shadowShips(root, from.Add(-10*time.Second), from)
	subSecondTo, _ := shadowShips(root, from.Add(-10*time.Second), from.Add(900*time.Millisecond))
	open, _ := shadowShips(root, from, time.Time{})

	if !reflect.DeepEqual(atFrom, []int{7}) || !reflect.DeepEqual(subSecondFrom, []int{7}) || len(justBefore) != 0 {
		t.Errorf("From: at %v, sub-second %v, after %v; want [7], [7], none", atFrom, subSecondFrom, justBefore)
	}
	if len(atTo) != 0 || len(subSecondTo) != 0 || !reflect.DeepEqual(open, []int{7}) {
		t.Errorf("To: at %v, sub-second %v, open %v; want none, none, [7]", atTo, subSecondTo, open)
	}
}

func TestShipsIn_AForgedShippedFlagOfACellFileCountsNothing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, rel := range []string{".evolve/runs/cycle-7/run.json", ".evolve/runs/cycle-7/phase-timing.json"} {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(`{"cycle_id":7,"shipped":true}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	ids, fails := shadowShips(root, shipEpoch, time.Time{})

	if len(ids) != 0 || len(fails) != 0 {
		t.Fatalf("ids %v, failures %v; want none", ids, fails)
	}
}

func TestShipsIn_ABrokenShipSourceIsAFailureAndNotAMissingShip(t *testing.T) {
	t.Parallel()
	good := journalLine(t, shipjournal.Entry{SHA: "aaa", Class: shipjournal.ClassCycle, TS: stamp(shipEpoch.Add(time.Minute)), Cycle: 7})

	t.Run("a bad stamp", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeJournalLines(t, root, journalLine(t, shipjournal.Entry{SHA: "bbb", Class: shipjournal.ClassCycle, TS: "bad", Cycle: 8}))
		ids, fails := shadowShips(root, shipEpoch, time.Time{})
		if len(ids) != 0 || len(fails) != 1 || !strings.Contains(fails[0].Error(), "bbb") || !strings.Contains(fails[0].Error(), "bad") {
			t.Fatalf("ids %v, failures %v", ids, fails)
		}
	})
	t.Run("a torn line keeps the good lines", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeJournalLines(t, root, good, "{torn\n")
		writeLanding(t, root, landing.Intent{Cycle: 7, CommitSHA: "aaa", Status: landing.IntentComplete})
		ids, fails := shadowShips(root, shipEpoch, time.Time{})
		if !reflect.DeepEqual(ids, []int{7}) || len(fails) != 1 || !strings.Contains(fails[0].Error(), "line 2") {
			t.Fatalf("ids %v, failures %v", ids, fails)
		}
	})
	t.Run("an unreadable journal", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		if err := os.MkdirAll(shipjournal.Path(filepath.Join(root, ".evolve")), 0o755); err != nil {
			t.Fatal(err)
		}
		ids, fails := shadowShips(root, shipEpoch, time.Time{})
		if len(ids) != 0 || len(fails) != 1 || !strings.Contains(fails[0].Error(), shipjournal.FileName) {
			t.Fatalf("ids %v, failures %v", ids, fails)
		}
	})
	t.Run("an unreadable landing record", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeJournalLines(t, root, good)
		path := landing.IntentPath(filepath.Join(root, ".evolve"), 7)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("{torn"), 0o644); err != nil {
			t.Fatal(err)
		}
		ids, fails := shadowShips(root, shipEpoch, time.Time{})
		if len(ids) != 0 || len(fails) != 1 || fails[0].Path != path {
			t.Fatalf("ids %v, failures %v", ids, fails)
		}
	})
	t.Run("a completion stamp that does not parse", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		landedCycle(t, root, 7, "aaa", shipEpoch.Add(time.Minute), "soon")
		ids, fails := shadowShips(root, shipEpoch, time.Time{})
		if len(ids) != 0 || len(fails) != 1 || !strings.Contains(fails[0].Error(), "soon") {
			t.Fatalf("ids %v, failures %v", ids, fails)
		}
	})
}

func deliverable(cycle int, at time.Time) integration.DeliverableRecord {
	return integration.DeliverableRecord{
		Wave: "w1", Cycle: cycle, Trailer: integration.CycleTrailer(cycle),
		Commits:     []integration.CommitIdentity{{SHA: "s", BaseSHA: "b", PatchID: "p"}},
		DeliveredAt: at,
	}
}

func writeDeliverable(t *testing.T, root, waveName string, r integration.DeliverableRecord) string {
	t.Helper()
	r.Wave = waveName
	path, err := integration.WriteRecord(integration.WaveDir(root, waveName), r)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func TestShipsIn_AtEnforceADeliverableRecordInTheWindowCountsWithNoLandingRecord(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeDeliverable(t, root, "w1", deliverable(7, shipEpoch.Add(time.Minute)))
	writeDeliverable(t, root, "w2", deliverable(8, shipEpoch.Add(3*time.Hour)))
	writeDeliverable(t, root, "w2", deliverable(7, shipEpoch.Add(time.Minute)))

	ids, fails := wave.ShipsIn(root, policy.FleetIntegrationEnforce, shipEpoch, shipEpoch.Add(time.Hour))

	if !reflect.DeepEqual(ids, []int{7}) || len(fails) != 0 {
		t.Fatalf("ids %v, failures %v; want [7] once and none", ids, fails)
	}
}

func TestShipsIn_AtEnforceTheWindowHoldsFromAndNotTo(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeDeliverable(t, root, "w1", deliverable(7, shipEpoch))
	writeDeliverable(t, root, "w1", deliverable(8, shipEpoch.Add(time.Hour)))

	ids, _ := wave.ShipsIn(root, policy.FleetIntegrationEnforce, shipEpoch, shipEpoch.Add(time.Hour))
	open, _ := wave.ShipsIn(root, policy.FleetIntegrationEnforce, shipEpoch, time.Time{})

	if !reflect.DeepEqual(ids, []int{7}) || !reflect.DeepEqual(open, []int{7, 8}) {
		t.Fatalf("ids %v, open %v; want [7] and [7 8]", ids, open)
	}
}

func TestShipsIn_AtEnforceAConsolePRRecordAndTheJournalCountNothing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	landedCycle(t, root, 9, "aaa", shipEpoch.Add(time.Minute), "")
	console := integration.DeliverableRecord{
		Source: integration.SourceConsolePR, PR: 12, Wave: integration.ConsoleWave,
		Commits:     []integration.CommitIdentity{{SHA: "s", BaseSHA: "b", PatchID: "p"}},
		Audit:       integration.AuditBinding{ArtifactSHA: strings.Repeat("a", 64), Class: "review"},
		DeliveredAt: shipEpoch.Add(time.Minute),
	}
	if _, err := integration.WriteRecord(integration.WaveDir(root, integration.ConsoleWave), console); err != nil {
		t.Fatal(err)
	}

	ids, fails := wave.ShipsIn(root, policy.FleetIntegrationEnforce, shipEpoch, time.Time{})

	if len(ids) != 0 || len(fails) != 0 {
		t.Fatalf("ids %v, failures %v; want none", ids, fails)
	}
}

func TestShipsIn_AtShadowADeliverableRecordWithoutALandingCountsNothing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeDeliverable(t, root, "w1", deliverable(7, shipEpoch.Add(time.Minute)))

	ids, _ := shadowShips(root, shipEpoch, time.Time{})

	if len(ids) != 0 {
		t.Fatalf("ids %v, want none", ids)
	}
}

func TestShipsIn_AtEnforceABrokenRecordIsAFailureAndAMissingWavesDirIsNot(t *testing.T) {
	t.Parallel()
	empty, fails := wave.ShipsIn(t.TempDir(), policy.FleetIntegrationEnforce, shipEpoch, time.Time{})
	if len(empty) != 0 || len(fails) != 0 {
		t.Fatalf("no waves dir: ids %v, failures %v", empty, fails)
	}

	root := t.TempDir()
	good := writeDeliverable(t, root, "w1", deliverable(7, shipEpoch.Add(time.Minute)))
	bad := filepath.Join(filepath.Dir(good), "deliverable-8.json")
	if err := os.WriteFile(bad, []byte("{torn"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(good), "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "waves", "other"), 0o755); err != nil {
		t.Fatal(err)
	}
	ids, fails := wave.ShipsIn(root, policy.FleetIntegrationEnforce, shipEpoch, time.Time{})
	if !reflect.DeepEqual(ids, []int{7}) || len(fails) != 1 || fails[0].Path != bad {
		t.Fatalf("ids %v, failures %v; want [7] and one failure at %s", ids, fails, bad)
	}

	wavesFile := t.TempDir()
	if err := os.MkdirAll(filepath.Join(wavesFile, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wavesFile, ".evolve", "waves"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, fails = wave.ShipsIn(wavesFile, policy.FleetIntegrationEnforce, shipEpoch, time.Time{})
	if len(fails) != 1 {
		t.Fatalf("an unreadable waves dir: failures %v, want one", fails)
	}
}

func TestShipsIn_AtEnforceAnUnreadableWaveDirIsAFailure(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	dir := integration.WaveDir(root, "w1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	_, fails := wave.ShipsIn(root, policy.FleetIntegrationEnforce, shipEpoch, time.Time{})

	if len(fails) != 1 || fails[0].Path != dir {
		t.Fatalf("failures %v, want one at %s", fails, dir)
	}
}
