package failurelog

// summary_fallback_test.go — a failure-log entry with an empty summary is a
// diagnostic that tells an operator nothing.
//
// The summary is extracted from orchestrator-report.md, which has NO production
// writer: 14 test files write it, zero non-test call sites do, and 0 of 241
// live cycle workspaces contain one. So `summary` has been empty on every
// recorded failure, silently, for as long as the file has been absent — the
// read tolerated it (correctly, it must not brick the log) and nothing noticed
// the diagnostic had gone hollow.
//
// The cycle's real verdict prose lives in audit-report.md and build-report.md,
// which the same workspace always has.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractSummaryForCycle_FallsBackToTheReportsThatExist(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	// The canonical source is absent, exactly as in production.
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

// The canonical report still wins when it exists, so nothing regresses if a
// producer is ever restored.
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

// An empty workspace stays empty rather than inventing a summary.
func TestExtractSummaryForCycle_InventsNothing(t *testing.T) {
	t.Parallel()
	if got := extractSummaryForCycle(t.TempDir()); got != "" {
		t.Errorf("with no report at all the summary must stay empty, got %q", got)
	}
}

// TestRecord_SummaryReachesTheEntryThroughTheRealPath — the review's MEDIUM.
//
// The three tests above exercise the helper. Deleting the CALL SITE in Record
// left every one of them green, so the wiring — the only thing that makes the
// fix real — was unpinned. This drives Record itself and asserts the recorded
// entry carries a summary, which is the property an operator actually depends
// on when they open the failure log.
func TestRecord_SummaryReachesTheEntryThroughTheRealPath(t *testing.T) {
	root := t.TempDir()
	ws := filepath.Join(root, ".evolve", "runs", "cycle-77")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	// Only the fallback source exists — production's actual shape.
	body := "# Audit Report\n\n## Verdict\n**FAIL**\n\nthe retry budget is never consumed\n"
	if err := os.WriteFile(filepath.Join(ws, "audit-report.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	// Record appends to the failure log inside state.json, so the file has to exist.
	if err := os.WriteFile(filepath.Join(root, ".evolve", "state.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Record(filepath.Join(root, ".evolve", "state.json"), filepath.Join(root, ".evolve", "runs"), RecordRequest{
		Cycle:          77,
		Classification: "audit-fail",
		ReportPath:     filepath.Join(ws, "orchestrator-report.md"), // absent, as in production
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
