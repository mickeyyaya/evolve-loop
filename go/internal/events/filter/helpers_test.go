package filter

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func testCatalog() Catalog {
	return Catalog{
		Kinds: []signalcenter.Kind{
			"cycle.sealed", "loop.exit", "loop.lost", "phase.outcome", "ship.error", "ship.landed",
		},
		Modules: []signalcenter.Module{"loop", "orchestrator", "ship"},
		Codes:   []signalcenter.Code{"LOOP_LOST", "SHIP_GATE_RED", "SKILLS_DRIFT_STALE"},
	}
}

func mustParse(t *testing.T, expr string) Filter {
	t.Helper()
	f, warnings, err := Parse(expr, testCatalog())
	if err != nil {
		t.Fatalf("Parse(%q): unexpected error %v", expr, err)
	}
	if len(warnings) != 0 {
		t.Fatalf("Parse(%q): unexpected warnings %v", expr, warnings)
	}
	return f
}

func sealedEvent() signalcenter.Event {
	return signalcenter.Event{
		SchemaVersion: signalcenter.SchemaVersion,
		Seq:           41,
		PID:           9055,
		TS:            "2026-10-09T17:46:02.114Z",
		Cycle:         1841,
		RunID:         "run-7",
		Phase:         "audit",
		Attempt:       2,
		Module:        "orchestrator",
		Origin:        "cycleRun.completeCycle",
		Kind:          "cycle.sealed",
		Code:          "SHIP_GATE_RED",
		Severity:      signalcenter.SeverityWarn,
		Reason:        "final verdict PASS",
		Fields:        map[string]string{"final_verdict": "PASS"},
	}
}

func signalRecord(e signalcenter.Event) Record {
	return Record{Source: "loop", Signal: &e}
}

func gapRecord() Record {
	return Record{Source: "loop"}
}
