package wave_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclespend"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
	"github.com/mickeyyaya/evolve-loop/go/internal/wavemeasure"
)

func TestReadCycles_ACycleCarriesItsLLMSpendAndItsAuditRounds(t *testing.T) {
	t.Parallel()
	p := newPlane(t)
	p.write(".evolve/runs/cycle-3/cycle-state.json", `{"cycle_id":3,"phase":"ship","audit_dispatches":4}`)
	p.write(".evolve/runs/cycle-3/llm-calls.ndjson",
		`{"phase":"build","duration_ms":60000,"usage_status":"measured","tokens":{"input":1,"output":2,"cache_read":10,"cache_write":3}}`+"\n"+
			`{"phase":"audit","duration_ms":30000,"usage_status":"unavailable","tokens":{}}`+"\n")

	got, warnings := wave.ReadCycles(p.root, 0, 0)

	want := cyclespend.Spend{LLMMillis: 90_000, FreshTokens: 6, CacheReadTokens: 10, UnmeasuredCalls: 1, AuditRounds: 4}
	if len(got) != 1 || got[0].Spend != want || len(warnings) != 0 {
		t.Errorf("ReadCycles = %+v, warnings %q; want one cycle with spend %+v", got, warnings, want)
	}
}

func TestReadCycles_WarnsOnAnUnreadableLLMCallLedger(t *testing.T) {
	t.Parallel()
	p := newPlane(t)
	p.write(".evolve/runs/cycle-4/cycle-state.json", `{"cycle_id":4,"phase":"build"}`)
	if err := os.MkdirAll(filepath.Join(p.root, ".evolve", "runs", "cycle-4", "llm-calls.ndjson"), 0o755); err != nil {
		t.Fatal(err)
	}

	_, warnings := wave.ReadCycles(p.root, 0, 0)

	if len(warnings) != 1 || !strings.Contains(warnings[0], "cycle-4: llm spend") {
		t.Errorf("warnings = %q, want one that names the llm spend of cycle-4", warnings)
	}
}

func TestMeasureFacts_EachCycleGivesItsSpendAndItsShipOutcome(t *testing.T) {
	t.Parallel()
	spend := cyclespend.Spend{LLMMillis: 5}
	cycles := []wave.Cycle{{ID: 1, Shipped: true, Spend: spend}, {ID: 2}}

	f, _ := wave.MeasureFacts(wave.Record{Number: 1, StartedAt: clock(0)}, nil, cycles, nil, 2)

	want := []wavemeasure.CycleSpend{{Shipped: true, Spend: spend}, {}}
	if len(f.Cycles) != 2 || f.Cycles[0] != want[0] || f.Cycles[1] != want[1] {
		t.Errorf("facts cycles = %+v, want %+v", f.Cycles, want)
	}
}

func TestReadCycles_TheSpendLimitSignalGivesTheDecision(t *testing.T) {
	t.Parallel()
	p := newPlane(t)
	p.write(".evolve/runs/cycle-7/cycle-state.json", `{"cycle_id":7,"phase":"audit"}`)
	p.write(".evolve/runs/cycle-7/signals.ndjson", eventAt(1, "phase.outcome", "CYCLE_SPEND_LIMIT", `"decision":"over"`)+eventAt(2, "phase.outcome", "CYCLE_SPEND_LIMIT", `"decision":"stop"`))

	got, _ := wave.ReadCycles(p.root, 0, 0)

	if len(got) != 1 || got[0].SpendLimit != "stop" {
		t.Errorf("ReadCycles = %+v, want the stop decision", got)
	}
	f, _ := wave.MeasureFacts(wave.Record{Number: 1, StartedAt: clock(0)}, nil, got, nil, 1)
	if f.Cycles[0].SpendLimit != "stop" {
		t.Errorf("facts = %+v, want the decision in the cycle spend", f.Cycles)
	}
}
