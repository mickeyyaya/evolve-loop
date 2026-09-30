package core

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
	a := verdictFailDistinguisher("audit", distWS(t, `{"top_n":[{"id":"task-a"}]}`, ""))
	b := verdictFailDistinguisher("audit", distWS(t, `{"top_n":[{"id":"task-b"}]}`, ""))
	if a == b || a == "" {
		t.Fatalf("different tasks must yield different distinguishers: %q vs %q", a, b)
	}
}

func TestVerdictFailDistinguisher_SameTaskStable(t *testing.T) {
	a := verdictFailDistinguisher("audit", distWS(t, `{"top_n":[{"id":"task-a"}]}`, ""))
	b := verdictFailDistinguisher("audit", distWS(t, `{"top_n":[{"id":"task-a"}]}`, ""))
	if a != b {
		t.Fatalf("same task must stay stable across cycles (real recurrence counting): %q vs %q", a, b)
	}
}

func TestVerdictFailDistinguisher_ReportFallbackWhenNoDecision(t *testing.T) {
	a := verdictFailDistinguisher("audit", distWS(t, "", "## Verdict\nFAIL\n- D1 CRITICAL: make target exits 2 on clean tree\n"))
	b := verdictFailDistinguisher("audit", distWS(t, "", "## Verdict\nFAIL\n- D1 HIGH: predicates never exercise production branch\n"))
	if a == b || a == "" {
		t.Fatalf("report-derived distinguishers must differ for different defects: %q vs %q", a, b)
	}
}

func TestVerdictFailDistinguisher_BareWorkspaceEmpty(t *testing.T) {
	if d := verdictFailDistinguisher("audit", distWS(t, "", "")); d != "" {
		t.Fatalf("no artifacts → empty distinguisher (constant-reason residual, documented), got %q", d)
	}
}
