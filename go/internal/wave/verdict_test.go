package wave_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclespend"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

func callRow(startMinute, endMinute int) string {
	return fmt.Sprintf(`{"phase":"build","started_at":%q,"ended_at":%q,"duration_ms":%d,"usage_status":"measured","tokens":{"input":1,"output":1}}`+"\n",
		clock(startMinute).Format("2006-01-02T15:04:05.000000Z"), clock(endMinute).Format("2006-01-02T15:04:05.000000Z"), (endMinute-startMinute)*60_000)
}

func (p plane) seedFailedCycle(id int, rejection string) {
	p.t.Helper()
	dir := fmt.Sprintf(".evolve/runs/cycle-%d/", id)
	p.write(dir+"llm-calls.ndjson", callRow(0, 10)+callRow(10, 30)+callRow(30, 50))
	p.write(dir+"signals.ndjson", eventAt(0, "ledger.appended", "", "")+rejection+eventAt(50, "cycle.sealed", "", `"final_verdict":"FAIL"`))
}

func wantVerdictMinutes(t *testing.T, got cyclespend.Verdict, toVerdict, failPoint, waste, ratio float64) {
	t.Helper()
	ok := func(g *float64, w float64) bool { return g != nil && math.Abs(*g-w) < 1e-9 }
	if got.LLMMinutesToVerdict != toVerdict || !ok(got.FailPointMinutes, failPoint) || !ok(got.WasteMinutes, waste) || !ok(got.FailFastRatio, ratio) {
		t.Errorf("LLMVerdict = %+v, want %v to verdict, fail point %v, waste %v, ratio %v", got, toVerdict, failPoint, waste, ratio)
	}
}

func TestReadCycles_AFloorRejectionAtMinute30OfAFiftyMinuteFailedCycleGivesFailPoint30Waste20Ratio06(t *testing.T) {
	t.Parallel()
	p := newPlane(t)
	p.seedFailedCycle(21, eventAt(30, "gate.rejected", "ORCHESTRATOR_BUILD_FLOOR", ""))

	got, warnings := wave.ReadCycles(p.root, 0, 0)

	if len(got) != 1 || len(warnings) != 0 {
		t.Fatalf("ReadCycles = %+v, warnings %q; want one cycle", got, warnings)
	}
	wantVerdictMinutes(t, got[0].LLMVerdict, 50, 30, 20, 0.6)
}

func TestReadCycles_TheFirstBlockingSignalSetsTheFailPointAmongSeveral(t *testing.T) {
	t.Parallel()
	p := newPlane(t)
	p.seedFailedCycle(22,
		eventAt(40, "gate.rejected", "ORCHESTRATOR_BUILD_FLOOR", "")+
			eventAt(30, "phase.outcome", "ORCHESTRATOR_PHASE_VERDICT_FAIL", `"verdict":"FAIL"`))

	got, _ := wave.ReadCycles(p.root, 0, 0)

	if len(got) != 1 {
		t.Fatalf("ReadCycles = %+v, want one cycle", got)
	}
	wantVerdictMinutes(t, got[0].LLMVerdict, 50, 30, 20, 0.6)
}

func TestReadCycles_AShippedCycleGivesMinutesToShipEvenAfterARejectionThatWasCorrected(t *testing.T) {
	t.Parallel()
	p := newPlane(t)
	p.write(".evolve/runs/cycle-23/llm-calls.ndjson", callRow(0, 15)+callRow(15, 40))
	p.write(".evolve/runs/cycle-23/signals.ndjson", eventAt(0, "ledger.appended", "", "")+eventAt(15, "gate.rejected", "ORCHESTRATOR_BUILD_FLOOR", "")+
		eventAt(40, "cycle.sealed", "", `"final_verdict":"PASS"`))
	p.write(".evolve/landing/cycle-23.json", `{"cycle":23,"status":"complete"}`)

	got, _ := wave.ReadCycles(p.root, 0, 0)

	v := got[0].LLMVerdict
	if len(got) != 1 || !got[0].Shipped || v.LLMMinutesToVerdict != 40 || v.FailPointMinutes != nil || v.WasteMinutes != nil || v.FailFastRatio != nil {
		t.Errorf("cycle = %+v, want shipped with 40 minutes and no fail point", got)
	}
}

func TestReadCycles_ACycleWithNoLedgerKeepsAZeroLLMVerdict(t *testing.T) {
	t.Parallel()
	p := newPlane(t)
	p.write(".evolve/runs/cycle-24/signals.ndjson", eventAt(0, "ledger.appended", "", "")+eventAt(5, "gate.rejected", "ORCHESTRATOR_BUILD_FLOOR", ""))

	got, _ := wave.ReadCycles(p.root, 0, 0)

	if len(got) != 1 || got[0].LLMVerdict != (cyclespend.Verdict{}) {
		t.Errorf("cycles = %+v, want one cycle with the zero LLMVerdict", got)
	}
}

func TestMeasureFacts_EachCycleCarriesItsLLMVerdictIntoTheCostFacts(t *testing.T) {
	t.Parallel()
	verdict := cyclespend.Verdict{LLMMinutesToVerdict: 50}
	rec := wave.Record{StartedAt: clock(0)}

	facts, _ := wave.MeasureFacts(rec, nil, []wave.Cycle{{ID: 1, LLMVerdict: verdict}}, nil, 2)

	if len(facts.Cycles) != 1 || facts.Cycles[0].Verdict != verdict {
		t.Errorf("Cycles = %+v, want the verdict %+v carried into the cost facts", facts.Cycles, verdict)
	}
}
