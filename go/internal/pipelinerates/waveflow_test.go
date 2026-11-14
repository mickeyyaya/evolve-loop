package pipelinerates

import (
	"errors"
	"fmt"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/lanetime"
	"github.com/mickeyyaya/evolve-loop/go/internal/qualitypanel"
)

func TestSlotUtilization_GivesLiveSlotHoursOverCapacityForEachWave(t *testing.T) {
	t.Parallel()
	flows := []WaveFlow{
		{Wave: 163, Slots: lanetime.Slots{Window: time.Hour, UsedSlotHours: 3.28, CapacitySlotHours: 4, Utilization: 0.82}},
		{Wave: 164, Slots: lanetime.Slots{Window: time.Hour, UsedSlotHours: 1.96, CapacitySlotHours: 4, Utilization: 0.49}},
		{Wave: 169},
	}

	readings := SlotUtilization(flows)

	if len(readings) != 3 {
		t.Fatalf("readings = %d, want one for each wave", len(readings))
	}
	for i, want := range []struct {
		window string
		v      float64
		n      int
	}{{"wave:163", 0.82, 1}, {"wave:164", 0.49, 1}, {"wave:169", 0, 0}} {
		r := readings[i]
		if r.Index != "PQ7" || r.Window != want.window || r.Interval.Value != want.v || r.N != want.n || r.Threshold != nil {
			t.Errorf("reading %d = %+v, want PQ7 %s %v n=%d", i, r, want.window, want.v, want.n)
		}
	}
}

func TestGateShare_GivesGateTimeOverLaneTimeForEachWave(t *testing.T) {
	t.Parallel()
	phases := lanetime.PhaseReport{Lane: 10 * time.Hour, Phases: []lanetime.PhaseStat{{Phase: "build", GateTotal: time.Hour}, {Phase: "audit", GateTotal: 2 * time.Hour}}}

	readings := GateShare([]WaveFlow{{Wave: 165, Phases: phases}, {Wave: 166}})

	if len(readings) != 2 {
		t.Fatalf("readings = %d, want one for each wave", len(readings))
	}
	got, empty := readings[0], readings[1]
	if got.Index != "PQ9" || got.Window != "wave:165" || got.Interval.Value != 0.3 || got.N != 1 || got.Threshold != nil || got.Status != "" {
		t.Errorf("reading = %+v, want PQ9 0.3 with no threshold", got)
	}
	if empty.N != 0 || empty.Interval.Value != 0 {
		t.Errorf("a wave with no lane time = %+v, want no data", empty)
	}
}

func writeLines(t *testing.T, path string, lines ...string) {
	t.Helper()
	fixtures.MustWrite(t, path, strings.Join(lines, "\n")+"\n")
}

type orchEvent struct {
	ts, code, cause   string
	pid, live, target int
}

func (e orchEvent) line() string {
	extra := ""
	if e.cause != "" {
		extra = fmt.Sprintf(`,"cause":%q`, e.cause)
	}
	return fmt.Sprintf(`{"ts":%q,"code":%q,"pid":%d,"fields":{"live":"%d","target":"%d"%s}}`, e.ts, e.code, e.pid, e.live, e.target, extra)
}

func flowFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	evolve := filepath.Join(root, ".evolve")
	writeJSON(t, filepath.Join(evolve, "waves", "wave-1.json"), map[string]any{"number": 1, "cycle_floor": 0, "pid": 11, "started_at": "2026-11-13T10:00:00Z", "ended_at": "2026-11-13T12:00:00Z"})
	writeJSON(t, filepath.Join(evolve, "waves", "wave-2.json"), map[string]any{"number": 2, "cycle_floor": 2, "pid": 12, "started_at": "2026-11-13T12:00:00Z"})
	writeLines(t, filepath.Join(evolve, "signals.ndjson"),
		orchEvent{ts: "2026-11-13T10:00:00Z", code: "LOOP_ORCH_POOL_FILLED", pid: 11, live: 2, target: 2, cause: ""}.line(),
		orchEvent{ts: "2026-11-13T11:00:00Z", code: "LOOP_ORCH_POOL_UNDERFILLED", pid: 11, live: 1, target: 2, cause: "run_cap"}.line(),
		"not json",
		`{"ts":"2026-11-13T11:30:00Z","code":"LOOP_ORCH_POOL_IDLE","pid":11,"fields":{"live":"x","target":"2"}}`)
	run := filepath.Join(evolve, "runs")
	writeLines(t, filepath.Join(run, "cycle-1", "signals.ndjson"),
		`{"ts":"2026-11-13T10:00:00Z","phase":"build","kind":"phase.dispatched"}`,
		`{"ts":"2026-11-13T11:00:00Z","phase":"build","kind":"phase.outcome","fields":{"verdict":"PASS"}}`)
	writeLines(t, filepath.Join(run, "cycle-1", "llm-calls.ndjson"), `{"phase":"build","cli":"claude","started_at":"2026-11-13T10:00:00Z","ended_at":"2026-11-13T10:30:00Z"}`)
	return root
}

func TestLoadWaveFlows_ReadsSlotsGatesAndIdleHoursForEachWave(t *testing.T) {
	t.Parallel()

	set, err := LoadWaveFlows(flowFixture(t))
	flows := set.Flows

	if err != nil || set.SamplesErr != nil || len(flows) != 2 {
		t.Fatalf("flows %+v, %v; want 2 waves", flows, err)
	}
	one, two := flows[0], flows[1]
	if one.Wave != 1 || one.Slots.Utilization != 0.75 || one.Phases.Lane != time.Hour || one.IdleHours["run_cap"] != 1 {
		t.Errorf("wave 1 = %+v, want utilization 0.75, lane 1h, 1 idle slot-hour of run_cap", one)
	}
	if got := SlotUtilization(flows)[0].Interval.Value; got != 0.75 {
		t.Errorf("PQ7 = %v, want 0.75", got)
	}
	if got := GateShare(flows)[0].Interval.Value; got != 0.5 {
		t.Errorf("PQ9 = %v, want 0.5", got)
	}
	if two.Wave != 2 || two.Slots.Window != 0 || two.Phases.Cycles != 0 || len(two.IdleHours) != 0 {
		t.Errorf("wave 2 = %+v, want no samples", two)
	}
}

func TestLoadWaveFlows_KeepsOnlyTheLastRollingWaves(t *testing.T) {
	t.Parallel()
	root := flowFixture(t)
	for n := 3; n <= 14; n++ {
		writeJSON(t, filepath.Join(root, ".evolve", "waves", fmt.Sprintf("wave-%d.json", n)), map[string]any{"number": n, "cycle_floor": n, "started_at": "2026-11-13T12:00:00Z"})
	}

	set, err := LoadWaveFlows(root)
	flows := set.Flows

	if err != nil || len(flows) != rollingWaves || flows[0].Wave != 5 || flows[len(flows)-1].Wave != 14 {
		t.Errorf("flows = %d first %d, %v; want the last %d waves", len(flows), flows[0].Wave, err, rollingWaves)
	}
}

func TestLoadWaveFlows_ASampleWithoutAPIDFallsBackToTheWaveTimeWindow(t *testing.T) {
	t.Parallel()
	root := flowFixture(t)
	writeJSON(t, filepath.Join(root, ".evolve", "waves", "wave-1.json"), map[string]any{"number": 1, "cycle_floor": 0, "started_at": "2026-11-13T10:00:00Z", "ended_at": "2026-11-13T12:00:00Z"})

	set, err := LoadWaveFlows(root)
	flows := set.Flows

	if err != nil || flows[0].Slots.Utilization != 0.75 {
		t.Errorf("flows %+v, %v; want utilization 0.75 from the time window", flows, err)
	}
}

func TestLoadWaveFlows_NoWaveRecordIsASourceAbsentError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	_, err := LoadWaveFlows(root)

	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("err = %v, want one that wraps fs.ErrNotExist", err)
	}
}

func TestLoadWaveFlows_AMissingOrchestrationStreamLeavesThePhaseReadingsAndNamesTheAbsence(t *testing.T) {
	t.Parallel()
	root := flowFixture(t)
	if err := os.Remove(filepath.Join(root, ".evolve", "signals.ndjson")); err != nil {
		t.Fatal(err)
	}

	set, err := LoadWaveFlows(root)

	if err != nil || !errors.Is(set.SamplesErr, fs.ErrNotExist) {
		t.Fatalf("err = %v, samples err = %v; want the absence on the samples only", err, set.SamplesErr)
	}
	if got := GateShare(set.Flows)[0].Interval.Value; got != 0.5 {
		t.Errorf("PQ9 = %v, want 0.5 without the orchestration stream", got)
	}
	if _, err := set.WithSamples(); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("WithSamples err = %v, want fs.ErrNotExist", err)
	}
}

func TestFlowSet_WithSamplesGivesThePhaseFlowsTogetherWithTheSamplesError(t *testing.T) {
	t.Parallel()
	boom := errors.New("stream unreadable")
	set := FlowSet{Flows: []WaveFlow{{Wave: 7}}, SamplesErr: boom}

	flows, err := set.WithSamples()

	if len(flows) != 1 || flows[0].Wave != 7 || !errors.Is(err, boom) {
		t.Errorf("WithSamples = %v, %v; want wave 7 with the samples error", flows, err)
	}
}

func TestLoadWaveFlows_ACycleOnTheFloorOfTheNextWaveBelongsToTheWaveBeforeIt(t *testing.T) {
	t.Parallel()
	root := flowFixture(t)
	writeLines(t, filepath.Join(root, ".evolve", "runs", "cycle-2", "signals.ndjson"),
		`{"ts":"2026-11-13T12:00:00Z","phase":"build","kind":"phase.dispatched"}`,
		`{"ts":"2026-11-13T12:30:00Z","phase":"build","kind":"phase.outcome","fields":{"verdict":"PASS"}}`)

	set, err := LoadWaveFlows(root)

	if err != nil || set.Flows[0].Phases.Cycles != 2 || set.Flows[1].Phases.Cycles != 0 {
		t.Errorf("flows %+v, %v; want cycles 1 and 2 in wave 1: cycle 2 is the floor of wave 2 and Bounds.Holds gives it to wave 1", set.Flows, err)
	}
}

func TestLoadWaveFlows_APIDWaveCountsASampleBeforeItsStart(t *testing.T) {
	t.Parallel()
	root := flowFixture(t)
	writeLines(t, filepath.Join(root, ".evolve", "signals.ndjson"),
		orchEvent{ts: "2026-11-13T09:00:00Z", code: "LOOP_ORCH_POOL_UNDERFILLED", pid: 11, live: 1, target: 2, cause: "run_cap"}.line(),
		orchEvent{ts: "2026-11-13T10:00:00Z", code: "LOOP_ORCH_POOL_FILLED", pid: 11, live: 2, target: 2}.line())

	set, err := LoadWaveFlows(root)

	if err != nil || set.Flows[0].IdleHours["run_cap"] != 1 || set.Flows[0].Slots.Window != 3*time.Hour {
		t.Errorf("wave 1 = %+v, %v; want the sample before started_at to count", set.Flows[0], err)
	}
}

func TestLoadWaveFlows_AnUnreadableRunsDirIsAnError(t *testing.T) {
	t.Parallel()
	root := flowFixture(t)
	if err := os.RemoveAll(filepath.Join(root, ".evolve", "runs")); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadWaveFlows(root); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a missing runs dir: err = %v, want fs.ErrNotExist", err)
	}
}

func TestLoadWaveFlows_AnUnreadableOrchestrationStreamIsAnError(t *testing.T) {
	t.Parallel()
	root := flowFixture(t)
	path := filepath.Join(root, ".evolve", "signals.ndjson")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadWaveFlows(root); err == nil || errors.Is(err, fs.ErrNotExist) {
		t.Errorf("err = %v, want a read error that is not an absence", err)
	}
}

func TestLoadWaveFlows_APIDWaveCountsNoSampleAfterItsEndInEitherIndex(t *testing.T) {
	t.Parallel()
	root := flowFixture(t)
	writeLines(t, filepath.Join(root, ".evolve", "signals.ndjson"),
		orchEvent{ts: "2026-11-13T10:00:00Z", code: "LOOP_ORCH_POOL_UNDERFILLED", pid: 11, live: 1, target: 2, cause: "run_cap"}.line(),
		orchEvent{ts: "2026-11-13T13:00:00Z", code: "LOOP_ORCH_POOL_IDLE", pid: 11, live: 0, target: 2, cause: "bench"}.line())

	set, err := LoadWaveFlows(root)

	one := set.Flows[0]
	if err != nil || one.IdleHours["run_cap"] != 2 || len(one.IdleHours) != 1 || one.Slots.Window != 2*time.Hour {
		t.Errorf("wave 1 = %+v, %v; want both indexes to cut at the wave end", one, err)
	}
}

func TestSlotUtilization_MarksEachWaveAgainstTheChartOfTheWavesBeforeIt(t *testing.T) {
	t.Parallel()
	utilization := map[int]float64{151: 0.98, 152: 0.92, 154: 0.97, 155: 0.97, 156: 0.94, 157: 0.98, 158: 0.97, 159: 0.90, 160: 0.97, 161: 0.96, 162: 0.84, 163: 0.82, 164: 0.49}
	var flows []WaveFlow
	for _, w := range []int{151, 152, 154, 155, 156, 157, 158, 159, 160, 161, 162, 163, 164} {
		flows = append(flows, WaveFlow{Wave: w, Slots: lanetime.Slots{Window: time.Hour, Utilization: utilization[w]}})
	}
	flows = append(flows[:2:2], append([]WaveFlow{{Wave: 153}}, flows[2:]...)...)
	flows = append(flows, WaveFlow{Wave: 169})

	readings := SlotUtilization(flows)

	marks := map[string]string{}
	for _, r := range readings {
		marks[r.Window] = r.Extra[qualitypanel.XmRRule1Field]
	}
	for window, want := range map[string]string{"wave:151": "", "wave:152": "", "wave:153": "", "wave:154": "in_control", "wave:161": "in_control", "wave:162": "outside", "wave:163": "outside", "wave:164": "outside", "wave:169": ""} {
		if marks[window] != want {
			t.Errorf("%s mark = %q, want %q (all marks: %v)", window, marks[window], want, marks)
		}
	}
}
