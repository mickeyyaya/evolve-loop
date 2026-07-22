package core

// verdict_distinguisher_test.go — cycle-1054/1060 pin: two DIFFERENT tasks'
// agent-graded audit FAILs shared one fingerprint because the verdict-path
// fallback reason was a constant string — three would falsely trip the
// identical-fingerprint breaker rule. The fallback must fold in per-failure
// content that is STABLE across recurrences of the same defect (task ids,
// report defect head) but differs across different defects. Cycle numbers are
// deliberately excluded — they would make every fingerprint unique and blind
// the breaker to real repeats.

import (
	"os"
	"path/filepath"
	"testing"
)

func distWS(t *testing.T, decision, report string) string {
	t.Helper()
	ws := t.TempDir()
	if decision != "" {
		if err := os.WriteFile(filepath.Join(ws, "triage-decision.json"), []byte(decision), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if report != "" {
		if err := os.WriteFile(filepath.Join(ws, "audit-report.md"), []byte(report), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return ws
}

func TestVerdictFailDistinguisher_DifferentTasksDiffer(t *testing.T) {
	a := verdictFailDistinguisher(distWS(t, `{"top_n":[{"id":"task-a"}]}`, ""))
	b := verdictFailDistinguisher(distWS(t, `{"top_n":[{"id":"task-b"}]}`, ""))
	if a == b || a == "" {
		t.Fatalf("different tasks must yield different distinguishers: %q vs %q", a, b)
	}
}

func TestVerdictFailDistinguisher_SameTaskStable(t *testing.T) {
	a := verdictFailDistinguisher(distWS(t, `{"top_n":[{"id":"task-a"}]}`, ""))
	b := verdictFailDistinguisher(distWS(t, `{"top_n":[{"id":"task-a"}]}`, ""))
	if a != b {
		t.Fatalf("same task must stay stable across cycles (real recurrence counting): %q vs %q", a, b)
	}
}

func TestVerdictFailDistinguisher_ReportFallbackWhenNoDecision(t *testing.T) {
	a := verdictFailDistinguisher(distWS(t, "", "## Verdict\nFAIL\n- D1 CRITICAL: make target exits 2 on clean tree\n"))
	b := verdictFailDistinguisher(distWS(t, "", "## Verdict\nFAIL\n- D1 HIGH: predicates never exercise production branch\n"))
	if a == b || a == "" {
		t.Fatalf("report-derived distinguishers must differ for different defects: %q vs %q", a, b)
	}
}

func TestVerdictFailDistinguisher_BareWorkspaceEmpty(t *testing.T) {
	if d := verdictFailDistinguisher(distWS(t, "", "")); d != "" {
		t.Fatalf("no artifacts → empty distinguisher (constant-reason residual, documented), got %q", d)
	}
}
