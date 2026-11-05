package wave_test

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/landing"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/shipjournal"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

func emitResume(t *testing.T, root string, at time.Time, cycle int) {
	t.Helper()
	path := filepath.Join(root, ".evolve", signalcenter.StreamFileName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	c := signalcenter.New(signalcenter.WithClock(func() time.Time { return at }))
	c.Subscribe(c.NDJSONSink(func(int) string { return path }))
	core.EmitLaneResumed(c, &core.ResumePoint{CycleID: cycle, Phase: "build", AutoAttempts: 1, AutoMaxAttempts: 3}, []string{"todo"})
}

func journalEntry(cycle int, sha string, at time.Time) shipjournal.Entry {
	return shipjournal.Entry{SHA: sha, Class: shipjournal.ClassCycle, TS: stamp(at), Cycle: cycle}
}

func landCycles(t *testing.T, root string, at time.Time, cycles ...int) {
	t.Helper()
	var lines []string
	for _, id := range cycles {
		sha := "sha" + strings.Repeat("a", id%7+1)
		lines = append(lines, journalLine(t, journalEntry(id, sha, at)))
		writeLanding(t, root, landing.Intent{Cycle: id, CommitSHA: sha, Status: landing.IntentComplete})
	}
	writeJournalLines(t, root, lines...)
}

func cycleIDs(r wave.WaveRead) []int {
	ids := []int{}
	for _, c := range r.Cycles {
		ids = append(ids, c.ID)
	}
	return ids
}

func shadowWindow(floor int) wave.WaveWindow {
	return wave.WaveWindow{Floor: floor, From: shipEpoch, To: shipEpoch.Add(time.Hour), Stage: policy.FleetIntegrationShadow}
}

func TestReadWave_ARedProofAResumedCycleAtTheFloorShipsInTheWindow(t *testing.T) {
	t.Parallel()
	p := newPlane(t)
	p.write(".evolve/runs/cycle-5/run.json", `{"cycle_id":5,"phase":"ship"}`)
	emitResume(t, p.root, shipEpoch.Add(time.Minute), 5)
	landCycles(t, p.root, shipEpoch.Add(2*time.Minute), 5)

	old, _ := wave.ReadCycles(p.root, 10, 0)
	got := wave.ReadWave(p.root, shadowWindow(10))

	if len(old) != 0 {
		t.Fatalf("the floor reader found %v; the proof needs a cycle it cannot see", old)
	}
	if !reflect.DeepEqual(got.Ships, []int{5}) || !reflect.DeepEqual(cycleIDs(got), []int{5}) {
		t.Fatalf("ships %v, cycles %v; want [5] and [5]", got.Ships, cycleIDs(got))
	}
}

func TestReadWave_TheMembersAreTheUnionOfTheCyclesAboveTheFloorTheResumedAndTheShips(t *testing.T) {
	t.Parallel()
	p := newPlane(t)
	p.write(".evolve/runs/cycle-12/run.json", `{"cycle_id":12,"phase":"build"}`)
	p.write(".evolve/runs/cycle-3/run.json", `{"cycle_id":3,"phase":"scout"}`)
	p.write("knowledge-base/cycles/cycle-5.json", `{"cycle":5,"final_verdict":"PASS"}`)
	emitResume(t, p.root, shipEpoch.Add(time.Minute), 5)
	emitResume(t, p.root, shipEpoch.Add(time.Minute), 12)
	landCycles(t, p.root, shipEpoch.Add(2*time.Minute), 7)

	got := wave.ReadWave(p.root, shadowWindow(10))

	if want := []int{5, 7, 12}; !reflect.DeepEqual(cycleIDs(got), want) {
		t.Fatalf("members %v, want %v", cycleIDs(got), want)
	}
	if got.Cycles[0].Verdict != "PASS" {
		t.Errorf("the resumed cycle 5 reads its dossier: verdict %q, want PASS", got.Cycles[0].Verdict)
	}
	if !reflect.DeepEqual(got.Ships, []int{7}) {
		t.Errorf("ships %v, want [7]: a resume is not a ship", got.Ships)
	}
}

func TestReadWave_ACycleAboveTheFloorIsReadOnceEvenWhenItResumedAndShipped(t *testing.T) {
	t.Parallel()
	p := newPlane(t)
	p.write(".evolve/runs/cycle-12/run.json", `{"cycle_id":12,"phase":"build"}`)
	emitResume(t, p.root, shipEpoch.Add(time.Minute), 12)
	landCycles(t, p.root, shipEpoch.Add(2*time.Minute), 12)

	got := wave.ReadWave(p.root, shadowWindow(10))

	if !reflect.DeepEqual(cycleIDs(got), []int{12}) || !reflect.DeepEqual(got.Ships, []int{12}) {
		t.Fatalf("cycles %v, ships %v; want [12] and [12]", cycleIDs(got), got.Ships)
	}
}

func TestReadWave_APlantedShippedFlagCountsNothing(t *testing.T) {
	t.Parallel()
	p := newPlane(t)
	p.write(".evolve/runs/cycle-12/run.json", `{"cycle_id":12,"phase":"ship","shipped":true}`)
	p.write(".evolve/runs/cycle-12/phase-timing.json", `{"cycle_id":12,"shipped":true}`)

	got := wave.ReadWave(p.root, shadowWindow(10))

	if len(got.Ships) != 0 {
		t.Fatalf("ships %v; a cell-writable flag is not a ship", got.Ships)
	}
}

func TestReadWave_TheStageSelectsTheShipRule(t *testing.T) {
	t.Parallel()
	p := newPlane(t)
	writeDeliverable(t, p.root, "w1", deliverable(9, shipEpoch.Add(time.Minute)))
	enforce := shadowWindow(10)
	enforce.Stage = policy.FleetIntegrationEnforce

	atEnforce := wave.ReadWave(p.root, enforce)
	atShadow := wave.ReadWave(p.root, shadowWindow(10))

	if !reflect.DeepEqual(atEnforce.Ships, []int{9}) || len(atShadow.Ships) != 0 {
		t.Fatalf("enforce ships %v, shadow ships %v; want [9] and none", atEnforce.Ships, atShadow.Ships)
	}
}

func TestReadWave_TheFailuresOfTheShipSourceAreKeptApartFromTheRest(t *testing.T) {
	t.Parallel()
	p := newPlane(t)
	p.write(".evolve/ship-journal.jsonl/keep", "x")
	p.write(".evolve/runs/cycle-12/run.json", `{broken`)
	p.write(".evolve/signals.ndjson", "not json\n")

	got := wave.ReadWave(p.root, shadowWindow(10))

	if len(got.ShipFailures) != 1 || !strings.Contains(got.ShipFailures[0].Path, "ship-journal.jsonl") {
		t.Fatalf("ship failures %v, want the journal path alone", got.ShipFailures)
	}
	var paths []string
	for _, f := range got.Failures {
		paths = append(paths, f.Path)
	}
	joined := strings.Join(paths, "\n")
	if !strings.Contains(joined, "cycle-12") || !strings.Contains(joined, "signals.ndjson") {
		t.Errorf("failures %v, want the broken run.json and the resume scan", paths)
	}
	if strings.Contains(joined, "ship-journal.jsonl") {
		t.Errorf("failures %v hold the ship source; it belongs to ShipFailures only", paths)
	}
}

func TestReadWave_AnUnreadableDossierDirOfAResumedCycleIsAFailure(t *testing.T) {
	t.Parallel()
	p := newPlane(t)
	p.write("knowledge-base/cycles/cycle-5.json", `{broken`)
	emitResume(t, p.root, shipEpoch.Add(time.Minute), 5)

	got := wave.ReadWave(p.root, shadowWindow(10))

	if !reflect.DeepEqual(cycleIDs(got), []int{5}) || len(got.Failures) == 0 {
		t.Fatalf("cycles %v, failures %v; want cycle 5 read and its bad dossier reported", cycleIDs(got), got.Failures)
	}
}

func TestReadWave_ReplayOfWave155TheJournalCountsTwoWhereTheFloorReaderCountsNone(t *testing.T) {
	t.Parallel()
	p := newPlane(t)
	for id := 2160; id <= 2168; id++ {
		p.write(filepath.Join(".evolve/runs", "cycle-"+itoa(id), "run.json"), `{"cycle_id":`+itoa(id)+`,"phase":"audit"}`)
	}
	p.write(".evolve/runs/cycle-2158/run.json", `{"cycle_id":2158,"phase":"ship"}`)
	p.write(".evolve/runs/cycle-2159/run.json", `{"cycle_id":2159,"phase":"ship"}`)
	emitResume(t, p.root, shipEpoch.Add(time.Minute), 2158)
	emitResume(t, p.root, shipEpoch.Add(2*time.Minute), 2159)
	landCycles(t, p.root, shipEpoch.Add(3*time.Minute), 2158, 2159)

	old, _ := wave.ReadCycles(p.root, 2159, 0)
	got := wave.ReadWave(p.root, shadowWindow(2159))

	shippedOld := 0
	for _, c := range old {
		if c.Shipped {
			shippedOld++
		}
	}
	if shippedOld != 0 || len(old) != 9 {
		t.Fatalf("the floor reader: %d of %d shipped; the replay needs 0 of 9", shippedOld, len(old))
	}
	if !reflect.DeepEqual(got.Ships, []int{2158, 2159}) || len(got.Cycles) != 11 {
		t.Fatalf("ships %v of %d cycles; want [2158 2159] of 11", got.Ships, len(got.Cycles))
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

func TestReadWave_AMemberAtOrBelowTheFloorIsResumedAndOneAboveItIsNot(t *testing.T) {
	t.Parallel()
	p := newPlane(t)
	p.write(".evolve/runs/cycle-10/run.json", `{"cycle_id":10,"phase":"ship"}`)
	p.write(".evolve/runs/cycle-11/run.json", `{"cycle_id":11,"phase":"build"}`)
	emitResume(t, p.root, shipEpoch.Add(time.Minute), 10)

	got := wave.ReadWave(p.root, shadowWindow(10))

	marks := map[int]bool{}
	for _, c := range got.Cycles {
		marks[c.ID] = c.Resumed
	}
	if want := map[int]bool{10: true, 11: false}; !reflect.DeepEqual(marks, want) {
		t.Fatalf("resumed marks %v, want %v", marks, want)
	}
}

func TestReadWave_TheLoopEventsAreReadAndTheLaneDeadlinesApplyToTheCycles(t *testing.T) {
	t.Parallel()
	p := newPlane(t)
	p.write(".evolve/signals.ndjson", eventAt(30, "loop.wave", "LOOP_ORCH_LANE_DEADLINE", `"action":"stopped","todo":"slow"`))
	p.write(".evolve/runs/cycle-7/lane-scope.json", `{"todo_ids":["slow"]}`)
	p.write(".evolve/runs/cycle-7/signals.ndjson", eventAt(1, "ledger.appended", "", ""))

	got := wave.ReadWave(p.root, shadowWindow(6))

	if len(got.Loop) != 1 {
		t.Fatalf("loop events %d, want 1", len(got.Loop))
	}
	if len(got.Cycles) != 1 || got.Cycles[0].Termination != cyclestate.CycleTerminationLaneDeadline {
		t.Fatalf("cycles %+v, want cycle 7 stopped by its lane deadline", got.Cycles)
	}
}

func TestReadWave_AMalformedOrUnreadableLoopStreamIsReportedOnceAsAFailure(t *testing.T) {
	t.Parallel()
	p := newPlane(t)
	stream := filepath.Join(p.root, ".evolve", "signals.ndjson")
	count := func(r wave.WaveRead) int {
		n := 0
		for _, f := range r.Failures {
			if f.Path == stream {
				n++
			}
		}
		return n
	}
	p.write(".evolve/signals.ndjson", "not json\n")
	got := wave.ReadWave(p.root, shadowWindow(0))
	if count(got) != 1 || len(got.Warnings) != 0 {
		t.Fatalf("malformed: failures %v warnings %q, want the stream once and no warning", got.Failures, got.Warnings)
	}
	denyRead(t, stream)
	if got := wave.ReadWave(p.root, shadowWindow(0)); count(got) != 1 {
		t.Fatalf("unreadable: failures %v, want the stream once", got.Failures)
	}
}

func TestRunFrom_TheRecordStartOrTheEarlierLogRunID(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)
	if got := wave.RunFrom(wave.Record{StartedAt: start}); !got.Equal(start) {
		t.Errorf("RunFrom = %v, want the record start %v", got, start)
	}
	early := wave.Record{StartedAt: start, RunID: gcpolicy.LogRunID(start.Add(-time.Hour))}
	if got := wave.RunFrom(early); !got.Equal(start.Add(-time.Hour)) {
		t.Errorf("RunFrom = %v, want the earlier run id time", got)
	}
}
