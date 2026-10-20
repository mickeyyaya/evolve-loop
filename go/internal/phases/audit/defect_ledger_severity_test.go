package audit

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/defectledger"
)

func TestEmitDefectLedger_ARowTakesTheSeverityOfItsFindingInTheReport(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	report := strings.Replace(failingReportWithDefects("M1 docs drift from the code", "L2 a nit"),
		"## Verdict", "## Issues\n\n### M1 (HIGH) — docs drift\n\n## Verdict", 1)
	if diags := emitDefectLedger(report, core.PhaseRequest{Cycle: 1572, Workspace: ws}); len(diags) != 0 {
		t.Fatalf("emitDefectLedger: %v", diags)
	}
	doc, _, err := defectledger.Read(ws)
	if err != nil || len(doc.Entries) != 2 {
		t.Fatalf("ledger = %+v, %v", doc, err)
	}
	got := map[string]string{}
	for _, e := range doc.Entries {
		got[e.Text] = e.Severity
	}
	if got["M1 docs drift from the code"] != "HIGH" || got["L2 a nit"] != "LOW" {
		t.Errorf("severities = %v; want M1 HIGH from its finding and L2 LOW from its id", got)
	}
}
