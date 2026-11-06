package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship/landing"
	"github.com/mickeyyaya/evolve-loop/go/internal/shipjournal"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

const rwFloor = 10

func rwEmitResume(t *testing.T, root string, at time.Time, cycle int) {
	t.Helper()
	c := newRootSignalCenter(root, filepath.Join(root, ".evolve"), io.Discard, signalcenter.WithClock(func() time.Time { return at }))
	core.EmitLaneResumed(c, &core.ResumePoint{CycleID: cycle, Phase: "build", AutoAttempts: 1, AutoMaxAttempts: 3}, []string{"todo"})
}

func rwLand(t *testing.T, p wavePlane, at time.Time, cycle int, sha string) {
	t.Helper()
	raw, err := json.Marshal(shipjournal.Entry{SHA: sha, Class: shipjournal.ClassCycle, TS: at.UTC().Format(time.RFC3339), Cycle: cycle})
	if err != nil {
		t.Fatal(err)
	}
	p.write(".evolve/ship-journal.jsonl", string(raw)+"\n")
	in := landing.Intent{Cycle: cycle, CommitSHA: sha, Status: landing.IntentComplete}
	if err := landing.WriteIntent(landing.IntentPath(p.evolveDir(), cycle), in); err != nil {
		t.Fatal(err)
	}
}

func rwResumedShipPlane(t *testing.T) (wavePlane, wave.Record) {
	t.Helper()
	p := newWavePlane(t)
	rec := wave.Record{Number: 130, RunID: "r130", PID: 9, CycleFloor: rwFloor, StartedAt: waveTestClock}
	if err := p.store().Save(rec); err != nil {
		t.Fatal(err)
	}
	p.write(".evolve/runs/cycle-5/run.json", `{"cycle_id":5,"phase":"ship"}`)
	p.write("knowledge-base/cycles/cycle-5.json", `{"cycle":5,"final_verdict":"PASS"}`)
	p.write(".evolve/runs/cycle-12/run.json", `{"cycle_id":12,"phase":"build"}`)
	rwEmitResume(t, p.root, waveTestClock.Add(time.Minute), 5)
	rwLand(t, p, waveTestClock.Add(2*time.Minute), 5, "sha5")
	return p, rec
}

func rwPlaneDirs(p wavePlane) wavePlaneDirs {
	return wavePlaneDirs{root: p.root, evolveDir: p.evolveDir(), store: p.store()}
}

func TestBuildWaveStatus_AResumedCycleAtTheFloorThatLandedIsOneShip(t *testing.T) {
	t.Parallel()
	p, rec := rwResumedShipPlane(t)

	report := buildWaveStatus(newWaveHarness().env(p.root), rwPlaneDirs(p), []wave.Record{rec})

	if report.Outcome != "1 of 2 cycles shipped" {
		t.Errorf("outcome %q, want 1 of 2 cycles shipped", report.Outcome)
	}
	var text []string
	for _, c := range report.Cycles {
		text = append(text, waveCycleText(c))
	}
	if want := "cycle 5: verdict PASS, shipped (resumed)"; len(text) != 2 || text[0] != want {
		t.Errorf("rows %q, want the first to be %q", text, want)
	}
}

func TestBuildWaveStatus_TheMeasureOfAWaveCountsOnlyTheCyclesAboveTheFloor(t *testing.T) {
	t.Parallel()
	p, rec := rwResumedShipPlane(t)

	report := buildWaveStatus(newWaveHarness().env(p.root), rwPlaneDirs(p), []wave.Record{rec})

	if got := report.Measures.Cost.Started; got != 1 {
		t.Errorf("cost started %d, want 1: the resumed cycle 5 is no cycle of this wave's own", got)
	}
}

func TestWaveWatcherPoll_AResumedCycleThatLandedReachesTheEvents(t *testing.T) {
	t.Parallel()
	p, rec := rwResumedShipPlane(t)
	var stdout, stderr bytes.Buffer
	w := &waveWatcher{env: newWaveHarness().env(p.root), plane: rwPlaneDirs(p), cur: rec, prev: wave.Snapshot{LoopLive: true}, seen: map[string]bool{}, stdout: &stdout, stderr: &stderr}

	if _, err := w.poll(); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(stdout.String(), "cycle 5") {
		t.Errorf("events:\n%s\nwant cycle 5, a resumed cycle at the floor", stdout.String())
	}
}

func TestLastWaveFacts_AResumedCycleThatLandedIsOneShip(t *testing.T) {
	t.Parallel()
	p, rec := rwResumedShipPlane(t)
	plan := &wavePlan{plane: rwPlaneDirs(p)}

	facts := lastWaveFacts(newWaveHarness().env(p.root), plan, rec)

	if got := facts.Outcome(); got != "1 of 2 cycles shipped" {
		t.Errorf("outcome %q, want 1 of 2 cycles shipped", got)
	}
}

func TestWaveFacts_APlantedShippedFlagWithNoJournalEntryCountsNothingAtTheStatusAndComposeEntries(t *testing.T) {
	t.Parallel()
	p, rec := rwResumedShipPlane(t)
	p.write(".evolve/runs/cycle-12/run.json", `{"cycle_id":12,"phase":"build","shipped":true}`)

	report := buildWaveStatus(newWaveHarness().env(p.root), rwPlaneDirs(p), []wave.Record{rec})
	facts := lastWaveFacts(newWaveHarness().env(p.root), &wavePlan{plane: rwPlaneDirs(p)}, rec)

	if report.Outcome != "1 of 2 cycles shipped" {
		t.Errorf("status outcome %q, want 1 of 2 cycles shipped: the flag of cycle 12 is forgeable", report.Outcome)
	}
	if got := facts.Outcome(); got != "1 of 2 cycles shipped" {
		t.Errorf("compose outcome %q, want 1 of 2 cycles shipped: the flag of cycle 12 is forgeable", got)
	}
}

func TestWaveShipStageOf_TheStageComesFromThePolicyAndABadPolicyFallsBackToShadowWithAWarning(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, policy, stage string
		warn                bool
	}{
		{"no policy file", "", "shadow", false},
		{"enforce", `{"fleet":{"integration":{"stage":"enforce"}}}`, "enforce", false},
		{"a stage that is neither", `{"fleet":{"integration":{"stage":"bogus"}}}`, "shadow", true},
		{"a policy that is not json", `{`, "shadow", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := newWavePlane(t)
			if c.policy != "" {
				p.write(".evolve/policy.json", c.policy)
			}
			stage, warning := shipStageOf(p.evolveDir())
			if stage != c.stage || (warning != "") != c.warn {
				t.Errorf("stage %q warning %q, want %q and warning=%v", stage, warning, c.stage, c.warn)
			}
		})
	}
}

func TestReadWaveOf_ABadStageIsAWarningOfTheRead(t *testing.T) {
	t.Parallel()
	p, rec := rwResumedShipPlane(t)
	p.write(".evolve/policy.json", `{"fleet":{"integration":{"stage":"bogus"}}}`)

	_, warnings := readWaveOf(rwPlaneDirs(p), rec)

	if len(warnings) != 1 || !strings.Contains(warnings[0], "shadow stage") {
		t.Errorf("warnings %q, want the stage fallback named", warnings)
	}
}

func TestFoldWaveRead_TheShipFailuresAndTheOtherFailuresAreBothWarnings(t *testing.T) {
	t.Parallel()
	read := wave.WaveRead{
		Warnings:     []string{"w"},
		Failures:     []wave.ReadFailure{{Path: "/a", Err: errors.New("one")}},
		ShipFailures: []wave.ReadFailure{{Path: "/b", Err: errors.New("two")}},
	}

	got := foldWaveRead(read)

	if len(got) != 3 || !strings.Contains(got[1], "/a") || !strings.Contains(got[2], "/b") {
		t.Errorf("warnings %q, want the warning then both failures", got)
	}
}
