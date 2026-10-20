package lanetime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

const fixture = "testdata/evolve"

func at(hhmm string) time.Time {
	t, err := time.Parse(time.RFC3339, "2026-10-19T"+hhmm+":00Z")
	if err != nil {
		panic(err)
	}
	return t
}

func loadAll(t *testing.T) []Cycle {
	t.Helper()
	cycles, _, err := ReadCycles(fixture, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	return cycles
}

func TestReadCycles_ReadsEveryRunDirWithSignalsAndWarnsAboutTheOthers(t *testing.T) {
	cycles, warnings, err := ReadCycles(fixture, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	var ids []int
	for _, c := range cycles {
		ids = append(ids, c.ID)
	}
	if !reflect.DeepEqual(ids, []int{1, 2, 3, 5, 6}) {
		t.Errorf("ids = %v, want [1 2 3 5 6]", ids)
	}
	if want := []string{"1 run dir(s) had no signals.ndjson"}; !reflect.DeepEqual(warnings, want) {
		t.Errorf("warnings = %v, want %v", warnings, want)
	}
	if got := len(cycles[0].Spans); got != 3 {
		t.Errorf("cycle 1 spans = %d, want 3", got)
	}
	if got := len(cycles[2].Calls); got != 1 || cycles[2].Calls[0].CLI != "agy-tmux" {
		t.Errorf("cycle 3 calls = %+v, want one agy-tmux call", cycles[2].Calls)
	}
}

func TestReadCycles_FloorIsExclusiveAndCeilingInclusive(t *testing.T) {
	cycles, _, err := ReadCycles(fixture, 1, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(cycles) != 2 || cycles[0].ID != 2 || cycles[1].ID != 3 {
		t.Errorf("cycles = %+v, want ids 2 and 3", cycles)
	}
}

func TestReadCycles_ReportsAnUnreadableRunsDir(t *testing.T) {
	if _, _, err := ReadCycles(filepath.Join(t.TempDir(), "absent"), 0, 0); err == nil {
		t.Error("a missing evolve dir must be an error, not an empty report")
	}
}

func TestReadCycles_CountsMalformedLinesAsAWarning(t *testing.T) {
	dir := t.TempDir()
	run := filepath.Join(dir, "runs", "cycle-9")
	if err := os.MkdirAll(run, 0o755); err != nil {
		t.Fatal(err)
	}
	signals := "{not json\n" + `{"ts":"2026-10-19T10:00:00Z","phase":"build","kind":"phase.dispatched"}` + "\n" +
		`{"ts":"2026-10-19T10:10:00Z","phase":"build","kind":"phase.aborted"}` + "\n"
	if err := os.WriteFile(filepath.Join(run, "signals.ndjson"), []byte(signals), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(run, "llm-calls.ndjson"), []byte("{bad\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cycles, warnings, err := ReadCycles(dir, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"2 malformed line(s) were skipped"}; !reflect.DeepEqual(warnings, want) {
		t.Errorf("warnings = %v, want %v", warnings, want)
	}
	if len(cycles) != 1 || len(cycles[0].Spans) != 1 || cycles[0].Spans[0].Verdict != "" {
		t.Errorf("an aborted phase must close its span: %+v", cycles)
	}
}

func TestReadCycles_AnUnreadableCallsFileIsAnError(t *testing.T) {
	dir := t.TempDir()
	run := filepath.Join(dir, "runs", "cycle-9")
	if err := os.MkdirAll(filepath.Join(run, "llm-calls.ndjson"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(run, "signals.ndjson"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ReadCycles(dir, 0, 0); err == nil {
		t.Error("an llm-calls path that cannot be read must fail loudly")
	}
}

func TestReadCycles_AnUnreadableSignalsFileIsAnError(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "runs", "cycle-9", "signals.ndjson"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ReadCycles(dir, 0, 0); err == nil {
		t.Error("a signals path that cannot be read must fail loudly")
	}
}

func TestCycle_LaneTimeSpansFirstDispatchToLastOutcome(t *testing.T) {
	cycles := loadAll(t)
	for id, want := range map[int]time.Duration{1: 50 * time.Minute, 2: 45 * time.Minute, 3: 70 * time.Minute, 5: 30 * time.Minute, 6: 5 * time.Minute} {
		var got time.Duration
		for _, c := range cycles {
			if c.ID == id {
				got = c.Lane()
			}
		}
		if got != want {
			t.Errorf("cycle %d lane = %v, want %v", id, got, want)
		}
	}
	if (Cycle{}).Lane() != 0 {
		t.Error("a cycle with no span has no lane time")
	}
}

func TestCycle_PassedMeansTheLastAuditVerdictIsPASS(t *testing.T) {
	got := map[int]bool{}
	for _, c := range loadAll(t) {
		got[c.ID] = c.Passed()
	}
	want := map[int]bool{1: true, 2: false, 3: true, 5: true, 6: false}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("passed = %v, want %v", got, want)
	}
}

func TestCycle_BuildFamilyStripsTheTmuxSuffixOfTheLastBuildCall(t *testing.T) {
	got := map[int]string{}
	for _, c := range loadAll(t) {
		got[c.ID] = c.BuildFamily()
	}
	want := map[int]string{1: "claude", 2: "claude", 3: "agy", 5: "claude", 6: ""}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("families = %v, want %v", got, want)
	}
}

func TestReadSamples_KeepsOnlyOrchestrationEventsWithALiveField(t *testing.T) {
	samples, malformed, err := ReadSamples(filepath.Join(fixture, "signals.ndjson"))
	if err != nil || malformed != 0 {
		t.Fatalf("err = %v malformed = %d", err, malformed)
	}
	want := []Sample{{At: at("10:00"), Live: 1, Target: 4}, {At: at("10:30"), Live: 3, Target: 4}, {At: at("11:00"), Live: 2, Target: 4}, {At: at("11:30"), Live: 0, Target: 4}}
	if !reflect.DeepEqual(samples, want) {
		t.Errorf("samples = %+v, want %+v", samples, want)
	}
}

func TestReadSamples_ANonNumericLiveFieldIsMalformed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "signals.ndjson")
	body := `{"ts":"2026-10-19T10:00:00Z","code":"LOOP_ORCH_POOL_IDLE","fields":{"live":"x","target":"4"}}` + "\n" +
		`{"ts":"2026-10-19T10:00:00Z","code":"LOOP_ORCH_POOL_IDLE","fields":{"live":"2","target":"y"}}` + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	samples, malformed, err := ReadSamples(path)
	if err != nil || len(samples) != 0 || malformed != 2 {
		t.Errorf("samples = %v malformed = %d err = %v, want none, 2, nil", samples, malformed, err)
	}
}

func TestReadSamples_AMissingFileIsAnError(t *testing.T) {
	if _, _, err := ReadSamples(filepath.Join(t.TempDir(), "absent")); err == nil {
		t.Error("a missing signals file must be an error")
	}
}

func TestSlotUse_WeightsLiveAndTargetByTime(t *testing.T) {
	samples, _, _ := ReadSamples(filepath.Join(fixture, "signals.ndjson"))
	got := SlotUse(samples, time.Time{}, time.Time{})
	want := Slots{Window: 90 * time.Minute, UsedSlotHours: 3.0, CapacitySlotHours: 6.0, AvgLive: 2.0, Utilization: 0.5}
	if got != want {
		t.Errorf("slots = %+v, want %+v", got, want)
	}
}

func TestSlotUse_AWindowClipsTheSamplesAndHoldsTheLastLiveToItsEnd(t *testing.T) {
	samples, _, _ := ReadSamples(filepath.Join(fixture, "signals.ndjson"))
	got := SlotUse(samples, at("10:30"), at("12:00"))
	want := Slots{Window: 90 * time.Minute, UsedSlotHours: 2.5, CapacitySlotHours: 6.0, AvgLive: 2.5 / 1.5, Utilization: 2.5 / 6.0}
	if got != want {
		t.Errorf("slots = %+v, want %+v", got, want)
	}
}

func TestSlotUse_NoSampleInTheWindowIsZero(t *testing.T) {
	samples, _, _ := ReadSamples(filepath.Join(fixture, "signals.ndjson"))
	if got := SlotUse(samples, at("13:00"), at("14:00")); got != (Slots{}) {
		t.Errorf("slots = %+v, want zero", got)
	}
	if got := SlotUse(nil, time.Time{}, time.Time{}); got != (Slots{}) {
		t.Errorf("slots = %+v, want zero", got)
	}
	if got := SlotUse(samples[:1], time.Time{}, time.Time{}); got != (Slots{}) {
		t.Errorf("a single sample spans no time: %+v", got)
	}
}

func TestPhases_SharesAreOfTheTotalLaneTimeWithTheGapsCounted(t *testing.T) {
	r := Phases(loadAll(t))
	if r.Lane != 200*time.Minute || r.Gap != 5*time.Minute || r.GapShare != 0.025 || r.Cycles != 5 {
		t.Errorf("lane %v gap %v share %v cycles %d, want 200m 5m 0.025 5", r.Lane, r.Gap, r.GapShare, r.Cycles)
	}
	want := []PhaseStat{
		{Phase: "build", Spans: 4, Time: 130 * time.Minute, Share: 0.65, Model: 100 * time.Minute, Host: 30 * time.Minute, Gates: 4, GateTotal: 30 * time.Minute, GateMedian: 7*time.Minute + 30*time.Second},
		{Phase: "audit", Spans: 4, Time: 40 * time.Minute, Share: 0.2, Model: 9 * time.Minute, Host: 31 * time.Minute, Gates: 2, GateTotal: 11 * time.Minute, GateMedian: 5*time.Minute + 30*time.Second},
		{Phase: "scout", Spans: 3, Time: 25 * time.Minute, Share: 0.125, Model: 0, Host: 25 * time.Minute},
	}
	if !reflect.DeepEqual(r.Phases, want) {
		t.Errorf("phases =\n%+v\nwant\n%+v", r.Phases, want)
	}
}

func TestPhases_NoCyclesIsAnEmptyReport(t *testing.T) {
	r := Phases(nil)
	if r.Lane != 0 || r.GapShare != 0 || len(r.Phases) != 0 {
		t.Errorf("report = %+v, want empty", r)
	}
}

func TestPhases_AnOddNumberOfGatesTakesTheMiddleOne(t *testing.T) {
	c := Cycle{ID: 1, Spans: []Span{{Phase: "build", Start: at("10:00"), End: at("10:30")}}, Calls: []Call{
		{Phase: "build", Start: at("10:00"), End: at("10:05")},
		{Phase: "build", Start: at("10:10"), End: at("10:15")},
		{Phase: "build", Start: at("10:20"), End: at("10:28")},
	}}
	r := Phases([]Cycle{c})
	if r.Phases[0].Gates != 3 || r.Phases[0].GateMedian != 5*time.Minute || r.Phases[0].GateTotal != 12*time.Minute {
		t.Errorf("gates = %+v, want 3 gates of 5m, 5m, 2m with median 5m", r.Phases[0])
	}
}

func TestPhases_ACallThatEndsAfterItsSpanHasNoGateAndOnlyItsOverlapIsModelTime(t *testing.T) {
	c := Cycle{ID: 1, Spans: []Span{{Phase: "build", Start: at("10:00"), End: at("10:10")}}, Calls: []Call{
		{Phase: "build", Start: at("10:05"), End: at("10:20")},
		{Phase: "tdd", Start: at("10:00"), End: at("10:10")},
		{Phase: "build", Start: at("09:00"), End: at("09:30")},
	}}
	p := Phases([]Cycle{c}).Phases[0]
	if p.Model != 5*time.Minute || p.Host != 5*time.Minute || p.Gates != 1 || p.GateTotal != 0 {
		t.Errorf("phase = %+v, want 5m model, 5m host, one zero gate", p)
	}
}

func TestLaneTimeToPass_DividesTheLaneTimeOfTheLastBuildsByTheirPasses(t *testing.T) {
	got := LaneTimeToPass(loadAll(t), 20)
	want := []FamilyLane{
		{Family: "agy", Builds: 1, Passes: 1, Lane: 70 * time.Minute, PerPass: 70 * time.Minute},
		{Family: "claude", Builds: 3, Passes: 2, Lane: 125 * time.Minute, PerPass: 62*time.Minute + 30*time.Second},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("families =\n%+v\nwant\n%+v", got, want)
	}
}

func TestLaneTimeToPass_OnlyTheLastNBuildsOfAFamilyCount(t *testing.T) {
	got := LaneTimeToPass(loadAll(t), 2)
	claude := got[1]
	want := FamilyLane{Family: "claude", Builds: 2, Passes: 1, Lane: 75 * time.Minute, PerPass: 75 * time.Minute}
	if claude != want {
		t.Errorf("claude = %+v, want %+v", claude, want)
	}
}

func TestLaneTimeToPass_AFamilyWithoutAPassHasNoMeasure(t *testing.T) {
	c := Cycle{ID: 1, Spans: []Span{{Phase: "build", Start: at("10:00"), End: at("10:30")}, {Phase: "audit", Start: at("10:30"), End: at("10:40"), Verdict: "FAIL"}},
		Calls: []Call{{Phase: "build", CLI: "agy-claude-tmux", Start: at("10:00"), End: at("10:20")}}}
	got := LaneTimeToPass([]Cycle{c}, 20)
	want := []FamilyLane{{Family: "agy-claude", Builds: 1, Passes: 0, Lane: 40 * time.Minute}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("families = %+v, want %+v", got, want)
	}
}

func TestLaneTimeToPass_TheOrderOfTheInputDoesNotMatter(t *testing.T) {
	cycles := loadAll(t)
	reversed := make([]Cycle, len(cycles))
	for i, c := range cycles {
		reversed[len(cycles)-1-i] = c
	}
	if !reflect.DeepEqual(LaneTimeToPass(cycles, 2), LaneTimeToPass(reversed, 2)) {
		t.Error("the last N builds are the highest cycle ids, whatever the input order")
	}
}

func TestSpanOf_IsTheEarliestStartAndTheLatestEndOfTheCycles(t *testing.T) {
	from, to := SpanOf(loadAll(t))
	if !from.Equal(at("10:00")) || !to.Equal(at("13:05")) {
		t.Errorf("span = %v to %v, want 10:00 to 13:05", from, to)
	}
	if from, to := SpanOf(nil); !from.IsZero() || !to.IsZero() {
		t.Errorf("an empty list has no span: %v %v", from, to)
	}
}

func TestReadCycles_IgnoresDirsThatAreNotCycleRunDirs(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"notes", "cycle-99999999999999999999", "cycle-4"} {
		if err := os.MkdirAll(filepath.Join(dir, "runs", name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "runs", "cycle-4", "signals.ndjson"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	cycles, warnings, err := ReadCycles(dir, 0, 0)
	if err != nil || len(cycles) != 1 || cycles[0].ID != 4 || len(warnings) != 0 {
		t.Errorf("cycles = %+v warnings = %v err = %v, want only cycle 4", cycles, warnings, err)
	}
}

func TestReadCycles_AnOutcomeWithoutADispatchIsSkippedAndCounted(t *testing.T) {
	dir := t.TempDir()
	run := filepath.Join(dir, "runs", "cycle-9")
	if err := os.MkdirAll(run, 0o755); err != nil {
		t.Fatal(err)
	}
	signals := `{"ts":"2026-10-19T10:00:00Z","phase":"build","kind":"phase.outcome","fields":{"verdict":"PASS"}}` + "\n" +
		`{"ts":"2026-10-19T10:01:00Z","phase":"build","kind":"phase.outcome"}` + "\n" +
		`{"ts":"2026-10-19T10:02:00Z","phase":"audit","kind":"phase.dispatched"}` + "\n" +
		`{"ts":"2026-10-19T10:03:00Z","phase":"audit","kind":"phase.dispatched"}` + "\n" +
		`{"ts":"2026-10-19T10:09:00Z","phase":"audit","kind":"phase.outcome","fields":{"verdict":"FAIL"}}` + "\n"
	if err := os.WriteFile(filepath.Join(run, "signals.ndjson"), []byte(signals), 0o644); err != nil {
		t.Fatal(err)
	}
	cycles, warnings, err := ReadCycles(dir, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"1 phase outcome(s) had no dispatch signal (older run dirs) and were skipped"}; !reflect.DeepEqual(warnings, want) {
		t.Errorf("warnings = %v, want %v", warnings, want)
	}
	if len(cycles[0].Spans) != 1 || !cycles[0].Spans[0].Start.Equal(at("10:02")) {
		t.Errorf("a repeated dispatch keeps the first start: %+v", cycles[0].Spans)
	}
}

func TestPhases_PhasesOfEqualTimeAreOrderedByName(t *testing.T) {
	c := Cycle{ID: 1, Spans: []Span{
		{Phase: "tdd", Start: at("10:00"), End: at("10:10")},
		{Phase: "scout", Start: at("10:10"), End: at("10:20")},
	}}
	r := Phases([]Cycle{c})
	if r.Phases[0].Phase != "scout" || r.Phases[1].Phase != "tdd" {
		t.Errorf("order = %v %v, want scout then tdd", r.Phases[0].Phase, r.Phases[1].Phase)
	}
}

func TestForPID_KeepsOnlyTheSamplesOfThatLoopProcess(t *testing.T) {
	samples := []Sample{{At: at("10:00"), Live: 1, PID: 7}, {At: at("10:10"), Live: 4, PID: 8}, {At: at("10:20"), Live: 2, PID: 7}}
	got := ForPID(samples, 7)
	want := []Sample{{At: at("10:00"), Live: 1, PID: 7}, {At: at("10:20"), Live: 2, PID: 7}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("samples = %+v, want %+v", got, want)
	}
	if len(ForPID(samples, 9)) != 0 {
		t.Error("an unknown pid owns no sample")
	}
}

func TestReadSamples_CarriesThePIDOfTheLoopThatEmittedTheEvent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "signals.ndjson")
	body := `{"pid":73972,"ts":"2026-10-19T10:00:00Z","code":"LOOP_ORCH_POOL_IDLE","fields":{"live":"2","target":"4"}}` + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	samples, _, err := ReadSamples(path)
	if err != nil || len(samples) != 1 || samples[0].PID != 73972 {
		t.Errorf("samples = %+v err = %v, want one sample of pid 73972", samples, err)
	}
}

func TestPhaseReport_TheJSONNamesAreStableForMachines(t *testing.T) {
	var report PhaseReport = Phases(loadAll(t))
	body, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil {
		t.Fatal(err)
	}
	var phases []map[string]json.RawMessage
	if err := json.Unmarshal(top["phases"], &phases); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"cycles", "lane", "gap", "gap_share", "phases"} {
		if _, ok := top[key]; !ok {
			t.Errorf("report lacks the key %q", key)
		}
	}
	for _, key := range []string{"phase", "spans", "time", "share", "model", "host", "gates", "gate_total", "gate_median"} {
		if _, ok := phases[0][key]; !ok {
			t.Errorf("phase stat lacks the key %q", key)
		}
	}
}

func TestPhases_ACallWithoutAnEndIsIgnoredAndParallelCallsNeverMakeHostNegative(t *testing.T) {
	c := Cycle{ID: 1, Spans: []Span{{Phase: "audit", Start: at("10:00"), End: at("10:10")}}, Calls: []Call{
		{Phase: "audit", Start: at("10:00")},
		{Phase: "audit", Start: at("10:00"), End: at("10:08")},
		{Phase: "audit", Start: at("10:01"), End: at("10:09")},
	}}
	p := Phases([]Cycle{c}).Phases[0]
	if p.Model != 10*time.Minute || p.Host != 0 || p.Gates != 2 {
		t.Errorf("phase = %+v, want model clamped to 10m, host 0 and 2 gates (the unfinished call is ignored)", p)
	}
}

func TestCycle_BuildFamilyIsTheFirstBuildCallAndDropsEverySuffixOfTheBaseCLI(t *testing.T) {
	c := Cycle{Calls: []Call{{Phase: "build", CLI: " claude-p-tmux "}, {Phase: "build", CLI: "agy-tmux"}}}
	if got := c.BuildFamily(); got != "claude" {
		t.Errorf("family = %q, want claude: the first dispatch owns the cycle", got)
	}
}

func TestLaneTimeToPass_ACycleThatIsStillRunningIsNotAFailedBuild(t *testing.T) {
	dir := t.TempDir()
	run := filepath.Join(dir, "runs", "cycle-9")
	if err := os.MkdirAll(run, 0o755); err != nil {
		t.Fatal(err)
	}
	signals := `{"ts":"2026-10-19T10:00:00Z","phase":"build","kind":"phase.dispatched"}` + "\n" +
		`{"ts":"2026-10-19T10:10:00Z","phase":"build","kind":"phase.outcome","fields":{"verdict":"PASS"}}` + "\n" +
		`{"ts":"2026-10-19T10:11:00Z","phase":"audit","kind":"phase.dispatched"}` + "\n"
	calls := `{"phase":"build","cli":"claude-tmux","started_at":"2026-10-19T10:00:00Z","ended_at":"2026-10-19T10:05:00Z"}` + "\n"
	for name, body := range map[string]string{"signals.ndjson": signals, "llm-calls.ndjson": calls} {
		if err := os.WriteFile(filepath.Join(run, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cycles, _, err := ReadCycles(dir, 0, 0)
	if err != nil || len(cycles) != 1 || cycles[0].Open != 1 {
		t.Fatalf("cycles = %+v err = %v, want one cycle with one open span", cycles, err)
	}
	if got := LaneTimeToPass(cycles, DefaultBuilds); len(got) != 0 {
		t.Errorf("families = %+v, want none: a live cycle has neither passed nor failed", got)
	}
}

func TestReadCycles_AnOutcomeWithoutAVerdictNeverClosesASpan(t *testing.T) {
	dir := t.TempDir()
	run := filepath.Join(dir, "runs", "cycle-9")
	if err := os.MkdirAll(run, 0o755); err != nil {
		t.Fatal(err)
	}
	signals := `{"ts":"2026-10-19T10:00:00Z","phase":"audit","kind":"phase.dispatched"}` + "\n" +
		`{"ts":"2026-10-19T10:01:00Z","phase":"audit","kind":"phase.outcome"}` + "\n" +
		`{"ts":"2026-10-19T10:09:00Z","phase":"audit","kind":"phase.outcome","fields":{"verdict":"PASS"}}` + "\n"
	if err := os.WriteFile(filepath.Join(run, "signals.ndjson"), []byte(signals), 0o644); err != nil {
		t.Fatal(err)
	}
	cycles, warnings, err := ReadCycles(dir, 0, 0)
	if err != nil || len(warnings) != 0 || len(cycles[0].Spans) != 1 || cycles[0].Spans[0].Verdict != "PASS" || cycles[0].Lane() != 9*time.Minute {
		t.Errorf("cycles = %+v warnings = %v err = %v, want one PASS span of 9m", cycles, warnings, err)
	}
}

func TestLaneTimeToPass_ACycleWithABuildCallButNoClosedSpanIsLeftOut(t *testing.T) {
	c := Cycle{ID: 1, Calls: []Call{{Phase: "build", CLI: "claude-tmux", Start: at("10:00"), End: at("10:05")}}}
	if got := LaneTimeToPass([]Cycle{c}, DefaultBuilds); len(got) != 0 {
		t.Errorf("families = %+v, want none: a cycle without a lane time says nothing", got)
	}
}
