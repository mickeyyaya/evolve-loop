package lanetime

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

var checkBounds = Bounds{MinLiveShare: 0.8, MaxGateShare: 0.25}

func checkWave(live, target int, modelMinutes int, marks ...Mark) WaveInput {
	start := at("10:00")
	end := at("12:00")
	spanEnd := start.Add(100 * time.Minute)
	return WaveInput{
		Number: 1, PID: 7, Start: start, End: end,
		Samples: []Sample{{At: start, Live: live, Target: target, PID: 7}},
		Cycles: []Cycle{{
			ID:    1,
			Spans: []Span{{Phase: "build", Start: start, End: spanEnd, Verdict: "PASS"}},
			Calls: []Call{{Phase: "build", Start: start, End: start.Add(time.Duration(modelMinutes) * time.Minute)}},
		}},
		Marks: marks,
	}
}

func TestCheck_TheLiveRuleBreachesBelowTheFloorAndTheSupplyGuardSkipsIt(t *testing.T) {
	if got := Check(checkWave(4, 4, 90), checkBounds); got.Status != StatusOK {
		t.Errorf("full lanes: status %q, want ok", got.Status)
	}
	low := Check(checkWave(2, 4, 90), checkBounds)
	if low.Status != StatusBreach || low.LiveShare != 0.5 {
		t.Errorf("2 of 4 lanes: status %q live %v, want a breach at 0.5", low.Status, low.LiveShare)
	}
	short := Check(checkWave(2, 4, 90, Mark{At: at("10:20"), PID: 7, Code: CodeSupplyShort}), checkBounds)
	if short.Status != StatusOK {
		t.Errorf("short supply: status %q, want ok", short.Status)
	}
}

func TestCheck_TheGateShareBreachesOnlyStrictlyAboveTheCeiling(t *testing.T) {
	if got := Check(checkWave(4, 4, 75), checkBounds); got.Status != StatusOK || got.GateShare != 0.25 {
		t.Errorf("exactly 25%%: status %q share %v, want ok", got.Status, got.GateShare)
	}
	if got := Check(checkWave(4, 4, 74), checkBounds); got.Status != StatusBreach {
		t.Errorf("26%%: status %q, want breach", got.Status)
	}
}

func TestCheck_APauseAStopReasonTheBrakeAndAnOpenWaveLeaveTheWaveOut(t *testing.T) {
	cases := map[string]WaveInput{
		"quota mark":  checkWave(2, 4, 90, Mark{At: at("10:30"), PID: 7, Code: CodeQuotaPaused}),
		"stop reason": checkWave(2, 4, 90, Mark{At: at("10:30"), PID: 7, Kind: kindLoopHalt, StopReason: "disk_space_halt"}),
		"bare halt":   checkWave(2, 4, 90, Mark{At: at("10:30"), PID: 7, Kind: kindLoopHalt}),
		"brake":       checkWave(2, 4, 90, Mark{At: at("10:30"), PID: 7, Kind: kindLoopHalt, StopReason: BrakeStopReason}),
		"by window":   checkWave(2, 4, 90, Mark{At: at("11:00"), PID: 99, Code: CodeQuotaPaused}),
	}
	paused := checkWave(2, 4, 90)
	paused.Cycles[0].Paused = true
	cases["paused cycle"] = paused
	open := checkWave(2, 4, 90)
	open.End = time.Time{}
	cases["open wave"] = open
	for name, in := range cases {
		if got := Check(in, checkBounds); got.Status != StatusLeftOut || len(got.Reasons) == 0 {
			t.Errorf("%s: status %q reasons %v, want left_out with a reason", name, got.Status, got.Reasons)
		}
	}
	foreign := checkWave(2, 4, 90, Mark{At: at("15:00"), PID: 99, Code: CodeQuotaPaused})
	if got := Check(foreign, checkBounds); got.Status != StatusBreach {
		t.Errorf("a pause of another process outside the wave: status %q, want breach", got.Status)
	}
	if got := Check(WaveInput{Number: 3, Start: at("10:00"), End: at("11:00")}, checkBounds); got.Status != StatusLeftOut {
		t.Errorf("a wave with no data: status %q, want left_out", got.Status)
	}
}

func TestBreachRun_OnlyTheTrailingBreachesCountAndLeftOutWavesAreSkipped(t *testing.T) {
	v := func(status string) Verdict { return Verdict{Status: status} }
	cases := []struct {
		name string
		in   []Verdict
		want int
	}{
		{"empty", nil, 0},
		{"ok at the end", []Verdict{v(StatusBreach), v(StatusBreach), v(StatusOK)}, 0},
		{"breach ok breach", []Verdict{v(StatusBreach), v(StatusOK), v(StatusBreach)}, 1},
		{"left out between", []Verdict{v(StatusBreach), v(StatusLeftOut), v(StatusBreach)}, 2},
		{"left out at the end", []Verdict{v(StatusBreach), v(StatusBreach), v(StatusLeftOut)}, 2},
	}
	for _, c := range cases {
		if got := len(BreachRun(c.in)); got != c.want {
			t.Errorf("%s: run length %d, want %d", c.name, got, c.want)
		}
	}
	if BreachRunToAlarm != 2 {
		t.Errorf("BreachRunToAlarm = %d, want 2", BreachRunToAlarm)
	}
}

func TestReadStream_KeepsThePauseSupplyAndHaltLinesWithTheirStopReason(t *testing.T) {
	path := filepath.Join(t.TempDir(), "signals.ndjson")
	body := `{"ts":"2026-10-19T10:00:00Z","pid":7,"kind":"quota.paused","code":"ORCHESTRATOR_QUOTA_PAUSED"}
{"ts":"2026-10-19T10:01:00Z","pid":7,"kind":"loop.wave","code":"LOOP_ORCH_SUPPLY_SHORT"}
{"ts":"2026-10-19T10:02:00Z","pid":7,"kind":"loop.halt","code":"LOOP_HALT","fields":{"stop_reason":"loop_operator_brake"}}
{"ts":"2026-10-19T10:03:00Z","pid":7,"kind":"loop.wave","code":"LOOP_ORCH_POOL_IDLE"}
not json
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	stream, malformed, err := ReadStream(path)
	marks := stream.Marks
	if err != nil || malformed != 1 || len(marks) != 3 || len(stream.Samples) != 0 {
		t.Fatalf("stream %+v malformed %d err %v, want 3 marks, no sample and 1 malformed line", stream, malformed, err)
	}
	if marks[2].StopReason != BrakeStopReason || marks[2].Kind != kindLoopHalt {
		t.Errorf("halt mark = %+v, want the brake stop_reason", marks[2])
	}
}

func TestReadCycles_ACycleStreamWithAQuotaPauseMarksTheCyclePaused(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "runs", "cycle-1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"ts":"2026-10-19T10:00:00Z","kind":"phase.dispatched","phase":"build"}
{"ts":"2026-10-19T10:05:00Z","kind":"quota.paused","code":"ORCHESTRATOR_QUOTA_PAUSED"}
`
	if err := os.WriteFile(filepath.Join(dir, "signals.ndjson"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cycles, _, err := ReadCycles(filepath.Dir(filepath.Dir(dir)), 0, 0)
	if err != nil || len(cycles) != 1 || !cycles[0].Paused {
		t.Fatalf("cycles %+v err %v, want one paused cycle", cycles, err)
	}
}

func TestStream_CarriesSamplesAndMarksFromOneSignalFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "signals.ndjson")
	body := `{"ts":"2026-10-19T10:00:00Z","pid":7,"kind":"loop.wave","code":"LOOP_ORCH_POOL_IDLE","fields":{"live":"2","target":"4"}}
{"ts":"2026-10-19T10:01:00Z","pid":7,"kind":"quota.paused","code":"ORCHESTRATOR_QUOTA_PAUSED"}
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	var stream Stream
	stream, _, err := ReadStream(path)
	if err != nil || len(stream.Samples) != 1 || len(stream.Marks) != 1 {
		t.Fatalf("stream %+v err %v, want 1 sample and 1 mark", stream, err)
	}
	if stream.Samples[0].Live != 2 || stream.Samples[0].Target != 4 {
		t.Errorf("sample = %+v, want live 2 target 4", stream.Samples[0])
	}
}
