package failurelog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractSummaryForCycle_FallsBackToTheReportsThatExist(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	audit := "# Audit Report\n\n## Verdict\n**FAIL**\n\n## Defects\n\n### C1 — CRITICAL · the retry budget is never consumed\n\nThe branch is unreachable.\n"
	if err := os.WriteFile(filepath.Join(ws, "audit-report.md"), []byte(audit), 0o644); err != nil {
		t.Fatal(err)
	}

	got := extractSummaryForCycle(ws)
	if strings.TrimSpace(got) == "" {
		t.Fatal("summary is empty although the workspace carries a verdict-bearing report — every failure-log entry has been hollow")
	}
	if !strings.Contains(got, "CRITICAL") && !strings.Contains(got, "FAIL") {
		t.Errorf("the summary must carry something an operator can act on; got %q", got)
	}
}

func TestExtractSummaryForCycle_PrefersTheCanonicalReport(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "orchestrator-report.md"), []byte("## Verdict\ncanonical source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "audit-report.md"), []byte("## Verdict\n**FAIL**\nfallback source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := extractSummaryForCycle(ws); !strings.Contains(got, "canonical") {
		t.Errorf("the canonical report must win when present; got %q", got)
	}
}

func TestExtractSummaryForCycle_InventsNothing(t *testing.T) {
	t.Parallel()
	if got := extractSummaryForCycle(t.TempDir()); got != "" {
		t.Errorf("with no report at all the summary must stay empty, got %q", got)
	}
}

func TestRecord_SummaryReachesTheEntryThroughTheRealPath(t *testing.T) {
	root := t.TempDir()
	ws := filepath.Join(root, ".evolve", "runs", "cycle-77")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "# Audit Report\n\n## Verdict\n**FAIL**\n\nthe retry budget is never consumed\n"
	if err := os.WriteFile(filepath.Join(ws, "audit-report.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "state.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Record(filepath.Join(root, ".evolve", "state.json"), filepath.Join(root, ".evolve", "runs"), RecordRequest{
		Cycle:          77,
		Classification: "audit-fail",
		ReportPath:     filepath.Join(ws, "orchestrator-report.md"),
	})
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	if strings.TrimSpace(got.Summary) == "" {
		t.Fatal("the recorded entry has an empty summary — the fallback is not wired into Record, only into its helper")
	}
	if !strings.Contains(got.Summary, "FAIL") && !strings.Contains(got.Summary, "retry budget") {
		t.Errorf("summary carries nothing actionable: %q", got.Summary)
	}
}
