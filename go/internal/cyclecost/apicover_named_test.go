package cyclecost

import (
	"path/filepath"
	"reflect"
	"testing"
)

// TestSummarizeCycle_StructShape names Summary and PhaseCost explicitly and
// pins the frozen aggregate shape: SummarizeCycle must return a Summary whose
// Cycle echoes the arg, whose Phases hold one PhaseCost per *-events.ndjson
// (alphabetically sorted), and whose Total sums every field. The whole-struct
// equality locks the field set the downstream JSON consumers depend on.
func TestSummarizeCycle_StructShape(t *testing.T) {
	ws := filepath.Join(t.TempDir(), "cycle-7")
	writeLog(t, ws, "auditor-events.ndjson", resultEnvelope(0.10, 100, 20, 3, 4))
	writeLog(t, ws, "scout-events.ndjson", resultEnvelope(0.25, 400, 80, 7, 9))

	got, err := SummarizeCycle(ws, 7)
	if err != nil {
		t.Fatalf("SummarizeCycle: %v", err)
	}

	want := Summary{
		Cycle: 7,
		Phases: []PhaseCost{
			{Phase: "auditor", CostUSD: 0.10, InputTokens: 100, OutputTokens: 20, CacheReadInputTokens: 3, CacheCreationInputTokens: 4},
			{Phase: "scout", CostUSD: 0.25, InputTokens: 400, OutputTokens: 80, CacheReadInputTokens: 7, CacheCreationInputTokens: 9},
		},
		Total: PhaseCost{CostUSD: 0.35, InputTokens: 500, OutputTokens: 100, CacheReadInputTokens: 10, CacheCreationInputTokens: 13},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Summary mismatch:\n got=%+v\nwant=%+v", got, want)
	}
}
