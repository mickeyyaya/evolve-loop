package wavehealth

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/dispatchevents"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

var (
	waveStart = time.Date(2026, 11, 21, 8, 17, 37, 0, time.UTC)
	checkTime = waveStart.Add(3 * time.Hour)
)

func settings() policy.WaveCheckinConfig { return policy.Policy{}.WaveCheckinSettings() }

func healthy() Snapshot {
	return Snapshot{
		Now:      checkTime,
		Settings: settings(),
		Loop:     Have(LoopState{Running: true}),
		Wave:     Have(WaveFacts{Start: waveStart, Started: 8, Shipped: 4, PoolLive: Have(5)}),
		Failures: Have(Failures{Breaker: core.BlockerBreakerConfig{IdenticalFingerprintCeiling: 3, ConsecutiveFailuresCeiling: 3}}),
		Abnormal: Have([]CycleEvent{}),
		Stalls:   Have([]StallLine{}),
		CI:       Have(CIState{Available: true, Status: "completed", Conclusion: "success", RunID: "9001"}),
		DiskFree: Have(uint64(100_000_000_000)),
		LoadSamples: Have([]LoadSample{
			{At: checkTime.Add(-10 * time.Minute), PerCore: 0.4},
			{At: checkTime.Add(-time.Minute), PerCore: 0.5},
		}),
	}
}

func step(t *testing.T, r Report, id string) Step {
	t.Helper()
	for _, s := range r.Steps {
		if s.ID == id {
			return s
		}
	}
	t.Fatalf("report has no step %s: %+v", id, r.Steps)
	return Step{}
}

func wantStep(t *testing.T, r Report, id string, result Result, reading string) {
	t.Helper()
	s := step(t, r, id)
	if s.Result != result || !strings.Contains(s.Reading, reading) {
		t.Errorf("step %s = {%s %q}; want result %s with reading containing %q", id, s.Result, s.Reading, result, reading)
	}
}

func TestEvaluate_AHealthySnapshotContinuesAndKeepsTheStepOrder(t *testing.T) {
	t.Parallel()
	r := Evaluate(healthy())
	var ids []string
	for _, s := range r.Steps {
		ids = append(ids, s.ID)
	}
	want := "A1 A2 A3 A4 A5 A6 A7 A8-disk A8-load A8-seats A9 A10"
	if got := strings.Join(ids, " "); got != want {
		t.Fatalf("step ids = %q, want %q", got, want)
	}
	if r.Outcome != OutcomeContinue {
		t.Fatalf("outcome = %s, want CONTINUE; steps %+v", r.Outcome, r.Steps)
	}
	for _, s := range r.Steps {
		if s.Threshold == "" || s.Reading == "" {
			t.Errorf("step %s has an empty reading or threshold: %+v", s.ID, s)
		}
	}
}

func TestEvaluate_GapRowsNameTheirCommandAndNeverDecide(t *testing.T) {
	t.Parallel()
	r := Evaluate(healthy())
	for _, id := range []string{"A7", "A8-seats", "A9", "A10"} {
		s := step(t, r, id)
		if s.Result != ResultGap || s.Action == "" {
			t.Errorf("step %s = %+v, want a GAP with an action", id, s)
		}
	}
}

func TestEvaluate_ALoopThatIsNotRunningEndsPartA(t *testing.T) {
	t.Parallel()
	s := healthy()
	s.Loop = Have(LoopState{Running: false})
	s.Stalls = Have([]StallLine{{At: waveStart.Add(time.Hour)}, {At: waveStart.Add(time.Hour)}, {At: waveStart.Add(time.Hour)}})
	r := Evaluate(s)
	wantStep(t, r, "A1", ResultOK, "not running")
	for _, id := range []string{"A2", "A3", "A4", "A5", "A6", "A8-disk", "A8-load"} {
		if got := step(t, r, id).Result; got != ResultSkip {
			t.Errorf("step %s = %s, want SKIP after A1", id, got)
		}
	}
	if r.Outcome != OutcomeContinue {
		t.Errorf("outcome = %s, want CONTINUE", r.Outcome)
	}
}

func TestEvaluate_AHeldBrakeTurnsBrakeIntoHeldAndKeepsFixLocal(t *testing.T) {
	t.Parallel()
	s := healthy()
	s.Loop = Have(LoopState{Running: true, BrakeHeld: true})
	s.Stalls = Have(stallLines(3))
	r := Evaluate(s)
	wantStep(t, r, "A1", ResultOK, "brake held")
	wantStep(t, r, "A5", ResultHeld, "3 ")
	if !strings.Contains(step(t, r, "A5").Action, "do not run the BRAKE procedure again") {
		t.Errorf("a held step must say not to run the BRAKE procedure again: %+v", step(t, r, "A5"))
	}
	if r.Outcome != OutcomeContinue {
		t.Errorf("outcome = %s, want CONTINUE while the brake is held", r.Outcome)
	}
	s.CI = Have(CIState{Available: true, Status: "completed", Conclusion: "failure", RunID: "7"})
	if got := Evaluate(s).Outcome; got != OutcomeFixLocal {
		t.Errorf("outcome with a held brake and a red CI = %s, want FIX-LOCAL", got)
	}
}

func TestEvaluate_ALoopThatIsUnknownIsAGapAndTheOtherStepsStillRun(t *testing.T) {
	t.Parallel()
	s := healthy()
	s.Loop = Known[LoopState]{}
	s.Stalls = Have(stallLines(3))
	r := Evaluate(s)
	wantStep(t, r, "A1", ResultGap, "not gathered")
	wantStep(t, r, "A5", ResultBrake, "3 ")
	if r.Outcome != OutcomeBrake {
		t.Errorf("outcome = %s, want BRAKE", r.Outcome)
	}
}

func TestEvaluate_ALoopOnlySnapshotNeverBrakesOnAMissingReading(t *testing.T) {
	t.Parallel()
	s := Snapshot{Now: checkTime, Settings: settings(), Loop: Have(LoopState{Running: true})}
	r := Evaluate(s)
	for _, id := range []string{"A2", "A3", "A4", "A5", "A6", "A8-disk", "A8-load"} {
		wantStep(t, r, id, ResultGap, "not gathered")
	}
	if r.Outcome != OutcomeContinue {
		t.Errorf("outcome = %s, want CONTINUE", r.Outcome)
	}
}

func wave(age time.Duration, started, shipped int, live Known[int]) Known[WaveFacts] {
	return Have(WaveFacts{Start: checkTime.Add(-age), Started: started, Shipped: shipped, PoolLive: live})
}

func TestEvaluate_A2ShipsAgainstAgeAndLiveLanes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		wave    Known[WaveFacts]
		result  Result
		reading string
	}{
		{"age under the limit is OK", wave(119*time.Minute+59*time.Second, 8, 0, Have(8)), ResultOK, "under"},
		{"age at the limit with 0 ships and 8 lanes is a STALL", wave(120*time.Minute, 8, 0, Have(8)), ResultBrake, "STALL"},
		{"live lanes under the limit is OK", wave(5*time.Hour, 8, 0, Have(3)), ResultOK, "live lanes 3 under 4"},
		{"live lanes at the limit with 0 ships is a STALL", wave(5*time.Hour, 8, 0, Have(4)), ResultBrake, "STALL"},
		{"one ship in 5 hours is SLOW", wave(5*time.Hour, 8, 1, Have(8)), ResultBrake, "SLOW"},
		{"the rate at the limit is OK", wave(4*time.Hour, 8, 1, Have(8)), ResultOK, "0.25 ships/h"},
		{"a healthy rate is OK", wave(5*time.Hour, 8, 3, Have(8)), ResultOK, "0.60 ships/h"},
		{"an unknown pool is a GAP after the age limit", wave(5*time.Hour, 8, 0, Known[int]{}), ResultGap, "no pool signal"},
		{"an unknown pool before the age limit is OK", wave(time.Hour, 8, 0, Known[int]{}), ResultOK, "under"},
		{"an unknown wave is a GAP", Known[WaveFacts]{}, ResultGap, "not gathered"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := healthy()
			s.Wave = c.wave
			wantStep(t, Evaluate(s), "A2", c.result, c.reading)
		})
	}
}

func TestEvaluate_A2ReadsItsThresholdsFromTheSettings(t *testing.T) {
	t.Parallel()
	s := healthy()
	s.Wave = wave(90*time.Minute, 8, 0, Have(8))
	wantStep(t, Evaluate(s), "A2", ResultOK, "under")
	s.Settings.StallMinAgeMinutes = 60
	wantStep(t, Evaluate(s), "A2", ResultBrake, "STALL")
	s.Settings.StallMinLiveLanes = 9
	wantStep(t, Evaluate(s), "A2", ResultOK, "live lanes 8 under 9")
	s.Settings.StallMinLiveLanes = 4
	s.Wave = wave(5*time.Hour, 8, 1, Have(8))
	s.Settings.MinShipsPerHour = 0.1
	wantStep(t, Evaluate(s), "A2", ResultOK, "0.20 ships/h")
}

var digestClasses = []string{"non-progress", "code-build-fail", "non-progress"}

func digests(fingerprints ...string) []DigestAt {
	var out []DigestAt
	for i, f := range fingerprints {
		out = append(out, DigestAt{Digest: core.FailureDigest{Cycle: 100 + i*2, Fingerprint: f, PreClass: "verdict-fail"}, Class: digestClasses[i%3], At: waveStart.Add(time.Duration(i+1) * time.Minute)})
	}
	return out
}

func TestEvaluate_A3UsesTheBlockerBreakerRules(t *testing.T) {
	t.Parallel()
	cfg := core.BlockerBreakerConfig{IdenticalFingerprintCeiling: 3}
	cases := []struct {
		name    string
		digests []DigestAt
		acked   map[string]bool
		result  Result
		reading string
	}{
		{"3 identical fingerprints at ceiling 3 is a REPEAT", digests("a", "a", "a"), nil, ResultBrake, "; REPEAT: rule identical-fingerprint"},
		{"2 identical fingerprints is OK", digests("a", "a"), nil, ResultOK, "no breaker rule matched"},
		{"different fingerprints are OK", digests("a", "b", "c"), nil, ResultOK, "3 failed cycle(s)"},
		{"an acknowledged fingerprint is ignored", digests("a", "a", "a"), map[string]bool{"a": true}, ResultOK, "no breaker rule matched"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := healthy()
			cfg := cfg
			cfg.AckedFingerprints = c.acked
			s.Failures = Have(Failures{Digests: c.digests, Breaker: cfg})
			r := Evaluate(s)
			wantStep(t, r, "A3", c.result, c.reading)
		})
	}
}

func events(kind dispatchevents.EventType, sev dispatchevents.Severity, cycles, each int) []CycleEvent {
	var out []CycleEvent
	for c := 0; c < cycles; c++ {
		for i := 0; i < each; i++ {
			at := waveStart.Add(time.Duration(c*10+i) * time.Minute).Format(time.RFC3339)
			out = append(out, CycleEvent{Cycle: 2582 + c, Event: dispatchevents.Event{EventType: kind, Severity: sev, Timestamp: at}})
		}
	}
	return out
}

func TestEvaluate_A4OneEventKindInManyCycles(t *testing.T) {
	t.Parallel()
	lock := dispatchevents.EventVerifySingleFlightUnavailable
	cases := []struct {
		name    string
		events  []CycleEvent
		result  Result
		reading string
	}{
		{"none", nil, ResultOK, "no WARN or ERROR event"},
		{"15 events in 3 cycles is a SHARED FAULT", events(lock, dispatchevents.SeverityWarn, 3, 5), ResultBrake, "verify-single-flight-unavailable: 15 event(s) in 3 cycle(s); SHARED FAULT"},
		{"15 events in 2 cycles is OK", append(events(lock, dispatchevents.SeverityWarn, 2, 7), events(lock, dispatchevents.SeverityWarn, 1, 1)[0]), ResultOK, "15 event(s) in 2 cycle(s)"},
		{"INFO events do not count", events(dispatchevents.EventPhaseOutputsSurveyed, dispatchevents.SeverityInfo, 9, 9), ResultOK, "no WARN or ERROR event"},
		{"ERROR events count", events(dispatchevents.EventStallDetected, dispatchevents.SeverityError, 3, 5), ResultBrake, "stall-detected: 15 event(s) in 3 cycle(s)"},
		{"the largest kind is the reading", append(events(lock, dispatchevents.SeverityWarn, 3, 5), events("other", dispatchevents.SeverityWarn, 3, 1)...), ResultBrake, "verify-single-flight-unavailable"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := healthy()
			s.Abnormal = Have(c.events)
			wantStep(t, Evaluate(s), "A4", c.result, c.reading)
		})
	}
}

func TestEvaluate_A4BoundaryAndTimeCut(t *testing.T) {
	t.Parallel()
	lock := dispatchevents.EventVerifySingleFlightUnavailable
	s := healthy()
	s.Abnormal = Have(events(lock, dispatchevents.SeverityWarn, 3, 5)[:14])
	wantStep(t, Evaluate(s), "A4", ResultOK, "14 event(s) in 3 cycle(s)")
	late := events(lock, dispatchevents.SeverityWarn, 3, 5)
	for i := range late {
		late[i].Event.Timestamp = checkTime.Add(time.Minute).Format(time.RFC3339)
	}
	s.Abnormal = Have(late)
	wantStep(t, Evaluate(s), "A4", ResultOK, "no WARN or ERROR event")
	for i := range late {
		late[i].Event.Timestamp = "not a time"
	}
	s.Abnormal = Have(late)
	wantStep(t, Evaluate(s), "A4", ResultBrake, "15 event(s) in 3 cycle(s)")
	s.Settings.AbnormalMinEvents = 16
	wantStep(t, Evaluate(s), "A4", ResultOK, "15 event(s) in 3 cycle(s)")
	s.Settings.AbnormalMinEvents = 15
	s.Settings.AbnormalMinCycles = 4
	wantStep(t, Evaluate(s), "A4", ResultOK, "15 event(s) in 3 cycle(s)")
}

func stallLines(n int) []StallLine {
	var out []StallLine
	for i := 0; i < n; i++ {
		out = append(out, StallLine{Lane: fmt.Sprintf("lane-%d", i), At: waveStart.Add(time.Duration(50+i) * time.Minute)})
	}
	return out
}

func TestEvaluate_A5CountsTheLaneStallsOfTheWave(t *testing.T) {
	t.Parallel()
	s := healthy()
	s.Stalls = Have(stallLines(2))
	wantStep(t, Evaluate(s), "A5", ResultOK, "2 ")
	s.Stalls = Have(stallLines(3))
	r := Evaluate(s)
	wantStep(t, r, "A5", ResultBrake, "3 LOOP_ORCH_LANE_STALLED line(s) in the wave; lanes lane-0, lane-1, lane-2; STALL")
	if !strings.Contains(step(t, r, "A5").Reading, "lane-0") {
		t.Errorf("the reading must name the lanes: %q", step(t, r, "A5").Reading)
	}
	s.Settings.StallLines = 4
	wantStep(t, Evaluate(s), "A5", ResultOK, "3 ")
	s.Settings.StallLines = 3
	before := append(stallLines(2), StallLine{Lane: "old", At: waveStart.Add(-time.Minute)})
	s.Stalls = Have(before)
	wantStep(t, Evaluate(s), "A5", ResultOK, "2 ")
	after := append(stallLines(2), StallLine{Lane: "future", At: checkTime.Add(time.Second)})
	s.Stalls = Have(after)
	wantStep(t, Evaluate(s), "A5", ResultOK, "2 ")
}

func TestEvaluate_A5CountsAStallOnlyWhileItsLaneIsStillSilent(t *testing.T) {
	t.Parallel()
	s := healthy()
	lines := stallLines(3)
	s.Stalls = Have(lines)
	wantStep(t, Evaluate(s), "A5", ResultBrake, "; STALL")

	recovered := append([]StallLine(nil), lines...)
	recovered[1].LastEvent = lines[1].At.Add(time.Minute)
	s.Stalls = Have(recovered)
	r := Evaluate(s)
	wantStep(t, r, "A5", ResultOK, "3 LOOP_ORCH_LANE_STALLED line(s) in the wave; 1 line(s) recovered; lanes lane-0, lane-2")
	if strings.HasSuffix(step(t, r, "A5").Reading, "STALL") {
		t.Errorf("two silent lanes are below the threshold: %q", step(t, r, "A5").Reading)
	}

	atStall := append([]StallLine(nil), lines...)
	atStall[1].LastEvent = lines[1].At
	s.Stalls = Have(atStall)
	wantStep(t, Evaluate(s), "A5", ResultBrake, "; STALL")
}

func TestStallLine_RecoveredIsTrueOnlyWhenTheLaneSpokeAfterTheStall(t *testing.T) {
	t.Parallel()
	stalledAt := waveStart.Add(time.Hour)
	cases := []struct {
		name      string
		lastEvent time.Time
		want      bool
	}{
		{"no event since the stall", time.Time{}, false},
		{"last event at the stall", stalledAt, false},
		{"last event before the stall", stalledAt.Add(-time.Second), false},
		{"last event after the stall", stalledAt.Add(time.Second), true},
	}
	for _, c := range cases {
		if got := (StallLine{At: stalledAt, LastEvent: c.lastEvent}).Recovered(); got != c.want {
			t.Errorf("%s: Recovered = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestEvaluate_A5WithoutAWaveCountsEveryLineUpToNow(t *testing.T) {
	t.Parallel()
	s := healthy()
	s.Wave = Known[WaveFacts]{}
	s.Stalls = Have(append(stallLines(2), StallLine{Lane: "old", At: waveStart.Add(-5 * time.Minute)}))
	wantStep(t, Evaluate(s), "A5", ResultBrake, "3 ")
}

func TestEvaluate_A6MainCI(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		ci      CIState
		result  Result
		reading string
		action  string
	}{
		{"success", CIState{Available: true, Status: "completed", Conclusion: "success", RunID: "9"}, ResultOK, "completed success", ""},
		{"in progress", CIState{Available: true, Status: "in_progress", RunID: "9"}, ResultOK, "in_progress", "start no integration batch"},
		{"queued", CIState{Available: true, Status: "queued", RunID: "9"}, ResultOK, "queued", "start no integration batch"},
		{"waiting", CIState{Available: true, Status: "waiting", RunID: "9"}, ResultOK, "waiting", "start no integration batch"},
		{"pending", CIState{Available: true, Status: "pending", RunID: "9"}, ResultOK, "pending", "start no integration batch"},
		{"requested", CIState{Available: true, Status: "requested", RunID: "9"}, ResultOK, "requested", "start no integration batch"},
		{"failure", CIState{Available: true, Status: "completed", Conclusion: "failure", RunID: "9"}, ResultFixLocal, "completed failure", "evolve ci classify 9 --json"},
		{"unavailable", CIState{Available: false}, ResultFixLocal, "unavailable", "evolve doctor probe gh"},
		{"cancelled is unmapped", CIState{Available: true, Status: "completed", Conclusion: "cancelled", RunID: "9"}, ResultBrake, "unmapped reading", ""},
		{"an unknown status is unmapped", CIState{Available: true, Status: "weird", RunID: "9"}, ResultBrake, "unmapped reading", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := healthy()
			s.CI = Have(c.ci)
			r := Evaluate(s)
			wantStep(t, r, "A6", c.result, c.reading)
			if !strings.Contains(step(t, r, "A6").Action, c.action) {
				t.Errorf("A6 action = %q, want it to contain %q", step(t, r, "A6").Action, c.action)
			}
		})
	}
}

func TestEvaluate_A8Disk(t *testing.T) {
	t.Parallel()
	s := healthy()
	floor := s.Settings.DiskFloorBytes
	s.DiskFree = Have(floor)
	wantStep(t, Evaluate(s), "A8-disk", ResultOK, "10.0 GB")
	s.DiskFree = Have(floor - 1)
	r := Evaluate(s)
	wantStep(t, r, "A8-disk", ResultFixLocal, "free 10.0 GB")
	if !strings.Contains(step(t, r, "A8-disk").Action, "evolve gc") {
		t.Errorf("the action must name evolve gc: %q", step(t, r, "A8-disk").Action)
	}
	s.DiskFree = Known[uint64]{}
	wantStep(t, Evaluate(s), "A8-disk", ResultGap, "not gathered")
}

func samples(perCore ...float64) Known[[]LoadSample] {
	var out []LoadSample
	for i, v := range perCore {
		out = append(out, LoadSample{At: checkTime.Add(-time.Duration(len(perCore)-i) * time.Minute), PerCore: v})
	}
	return Have(out)
}

func TestEvaluate_A8LoadMustBeSustained(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		samples Known[[]LoadSample]
		result  Result
		reading string
	}{
		{"every sample at the limit is a BRAKE", samples(3.5, 4.0, 6.4), ResultBrake, "lowest 3.50"},
		{"one low sample is OK", samples(4, 3.49, 5), ResultOK, "lowest 3.49"},
		{"one sample is a GAP", samples(9), ResultGap, "1 sample"},
		{"no sample is a GAP", samples(), ResultGap, "0 sample"},
		{"unknown samples are a GAP", Known[[]LoadSample]{}, ResultGap, "not gathered"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := healthy()
			s.LoadSamples = c.samples
			wantStep(t, Evaluate(s), "A8-load", c.result, c.reading)
		})
	}
}

func TestEvaluate_A8LoadIgnoresSamplesOutsideTheWindow(t *testing.T) {
	t.Parallel()
	s := healthy()
	s.LoadSamples = Have([]LoadSample{
		{At: checkTime.Add(-16 * time.Minute), PerCore: 0.1},
		{At: checkTime.Add(-15 * time.Minute), PerCore: 4},
		{At: checkTime.Add(-time.Minute), PerCore: 4},
		{At: checkTime.Add(time.Second), PerCore: 0.1},
	})
	wantStep(t, Evaluate(s), "A8-load", ResultBrake, "2 sample")
	s.Settings.LoadSustainedMinutes = 16
	wantStep(t, Evaluate(s), "A8-load", ResultOK, "3 sample")
	s.Settings.LoadSustainedMinutes = 17
	wantStep(t, Evaluate(s), "A8-load", ResultOK, "3 sample")
	s.Settings.LoadPerCoreLimit = 5
	wantStep(t, Evaluate(s), "A8-load", ResultOK, "3 sample")
}

func TestEvaluate_TheOutcomeRule(t *testing.T) {
	t.Parallel()
	red := CIState{Available: true, Status: "completed", Conclusion: "failure", RunID: "1"}
	cases := []struct {
		name  string
		edit  func(*Snapshot)
		want  Outcome
		stepA string
	}{
		{"nothing", func(*Snapshot) {}, OutcomeContinue, "A6"},
		{"fix local only", func(s *Snapshot) { s.CI = Have(red) }, OutcomeFixLocal, "A6"},
		{"brake only", func(s *Snapshot) { s.Stalls = Have(stallLines(3)) }, OutcomeBrake, "A5"},
		{"brake beats fix local", func(s *Snapshot) { s.CI = Have(red); s.Stalls = Have(stallLines(3)) }, OutcomeBrake, "A5"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := healthy()
			c.edit(&s)
			r := Evaluate(s)
			if r.Outcome != c.want {
				t.Fatalf("outcome = %s, want %s", r.Outcome, c.want)
			}
			if c.want != OutcomeContinue && step(t, r, c.stepA).Result == ResultOK {
				t.Errorf("step %s must not be OK", c.stepA)
			}
		})
	}
}

func TestEvaluate_Wave185AtTheSOPReading(t *testing.T) {
	t.Parallel()
	s := healthy()
	s.Now = waveStart.Add(2*time.Hour + 5*time.Minute)
	s.Wave = Have(WaveFacts{Start: waveStart, Started: 8, Shipped: 0, PoolLive: Have(8)})
	r := Evaluate(s)
	wantStep(t, r, "A2", ResultBrake, "STALL")
	if r.Outcome != OutcomeBrake {
		t.Fatalf("outcome = %s, want BRAKE", r.Outcome)
	}
}

func TestEvaluate_A3ReadingListsTheClassCountsInOrder(t *testing.T) {
	t.Parallel()
	s := healthy()
	s.Failures = Have(Failures{Digests: digests("a", "b", "c"), Breaker: core.BlockerBreakerConfig{IdenticalFingerprintCeiling: 3}})
	wantStep(t, Evaluate(s), "A3", ResultOK, "classes code-build-fail 1, non-progress 2")
}

func TestEvaluate_A3DropsTheDigestsOfCyclesThatEndAfterNow(t *testing.T) {
	t.Parallel()
	s := healthy()
	d := digests("a", "a", "a")
	d[2].At = checkTime.Add(time.Second)
	s.Failures = Have(Failures{Digests: d, Breaker: core.BlockerBreakerConfig{IdenticalFingerprintCeiling: 3}})
	wantStep(t, Evaluate(s), "A3", ResultOK, "2 failed cycle(s)")
	s.Now = checkTime.Add(time.Second)
	wantStep(t, Evaluate(s), "A3", ResultBrake, "identical-fingerprint")
}

func TestEvaluate_A3WithNoFailureReadsAsNone(t *testing.T) {
	t.Parallel()
	wantStep(t, Evaluate(healthy()), "A3", ResultOK, "0 failed cycle(s)")
}

func TestEvaluate_A3NamesTheCeilingsOfTheBreaker(t *testing.T) {
	t.Parallel()
	s := healthy()
	s.Failures = Have(Failures{Breaker: core.BlockerBreakerConfig{GuardClassCeiling: 2, IdenticalFingerprintCeiling: 3, UnexplainedCeiling: 4, ConsecutiveFailuresCeiling: 5}})
	if got := step(t, Evaluate(s), "A3").Threshold; !strings.Contains(got, "guard-class 2") || !strings.Contains(got, "identical fingerprint 3") || !strings.Contains(got, "unexplained 4") || !strings.Contains(got, "consecutive 5") {
		t.Errorf("A3 threshold = %q, want the four ceilings", got)
	}
}

func TestEvaluate_A5NeedsTheLinesInsideOneWindow(t *testing.T) {
	t.Parallel()
	at := func(min int) StallLine {
		return StallLine{Lane: fmt.Sprintf("l%d", min), At: waveStart.Add(time.Duration(min) * time.Minute)}
	}
	cases := []struct {
		name   string
		lines  []StallLine
		result Result
	}{
		{"3 lines in exactly 60 minutes", []StallLine{at(10), at(40), at(70)}, ResultBrake},
		{"3 lines in 61 minutes", []StallLine{at(10), at(40), at(71)}, ResultOK},
		{"5 lines spread over hours", []StallLine{at(10), at(80), at(150), at(160), at(240)}, ResultOK},
		{"3 close lines after early ones", []StallLine{at(5), at(100), at(110), at(120)}, ResultBrake},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := healthy()
			s.Now = waveStart.Add(5 * time.Hour)
			s.Stalls = Have(c.lines)
			r := Evaluate(s)
			if got := step(t, r, "A5").Result; got != c.result {
				t.Errorf("A5 = %s (%s), want %s", got, step(t, r, "A5").Reading, c.result)
			}
		})
	}
	s := healthy()
	s.Now = waveStart.Add(5 * time.Hour)
	s.Stalls = Have([]StallLine{at(10), at(40), at(71)})
	s.Settings.StallWindowMinutes = 61
	wantStep(t, Evaluate(s), "A5", ResultBrake, "3 LOOP_ORCH_LANE_STALLED")
}

func TestEvaluate_A4KindsWithEqualCountsAreOrderedByName(t *testing.T) {
	t.Parallel()
	s := healthy()
	s.Abnormal = Have(append(events("zeta", dispatchevents.SeverityWarn, 3, 5), events("alpha", dispatchevents.SeverityWarn, 3, 5)...))
	wantStep(t, Evaluate(s), "A4", ResultBrake, "alpha: 15 event(s)")
}

func TestEvaluate_A3AnUnclassifiedDigestIsCountedAsUnclassified(t *testing.T) {
	t.Parallel()
	s := healthy()
	d := digests("a")
	d[0].Class = ""
	s.Failures = Have(Failures{Digests: d, Breaker: core.BlockerBreakerConfig{IdenticalFingerprintCeiling: 3}})
	wantStep(t, Evaluate(s), "A3", ResultOK, "classes unclassified 1")
}

func TestEvaluate_A2AWaveThatStartsAfterNowHasAgeZeroAndRateZero(t *testing.T) {
	t.Parallel()
	s := healthy()
	s.Wave = Have(WaveFacts{Start: checkTime.Add(time.Hour), Started: 1, Shipped: 1, PoolLive: Have(8)})
	wantStep(t, Evaluate(s), "A2", ResultOK, "age 0s, ships 1 of 1, live 8, 0.00 ships/h")
}

func TestEvaluate_A5AZeroLineCountNeverBrakesAndNeverPanics(t *testing.T) {
	t.Parallel()
	s := healthy()
	s.Settings.StallLines = 0
	s.Stalls = Have(stallLines(3))
	wantStep(t, Evaluate(s), "A5", ResultOK, "3 LOOP_ORCH_LANE_STALLED")
	s.Stalls = Have([]StallLine{})
	wantStep(t, Evaluate(s), "A5", ResultOK, "0 LOOP_ORCH_LANE_STALLED")
}
