package failureadapter

import (
	"testing"
	"time"
)

func TestListPendingByClass_EmptyEntries(t *testing.T) {
	got := ListPendingByClass(nil, CodeAuditFail, time.Now())
	if len(got) != 0 {
		t.Errorf("empty input must return empty, got %d entries", len(got))
	}
}

func TestListPendingByClass_FiltersByClassification(t *testing.T) {
	now := time.Date(2026, 5, 26, 0, 0, 0, 0, time.UTC)
	recent := now.Add(-1 * time.Hour).Format(time.RFC3339)
	future := now.Add(24 * time.Hour).Format(time.RFC3339)
	entries := []Entry{
		{Cycle: 87, Classification: CodeAuditFail, RecordedAt: recent, ExpiresAt: future, Verdict: "FAIL", Summary: "first audit fail"},
		{Cycle: 88, Classification: CodeBuildFail, RecordedAt: recent, ExpiresAt: future, Verdict: "FAIL", Summary: "build fail (skip)"},
		{Cycle: 89, Classification: CodeAuditFail, RecordedAt: recent, ExpiresAt: future, Verdict: "FAIL", Summary: "second audit fail"},
		{Cycle: 90, Classification: InfraTransient, RecordedAt: recent, ExpiresAt: future, Verdict: "FAIL", Summary: "infra (skip)"},
	}
	got := ListPendingByClass(entries, CodeAuditFail, now)
	if len(got) != 2 {
		t.Fatalf("expected 2 audit-fail entries, got %d", len(got))
	}
	if got[0].Cycle != 87 || got[1].Cycle != 89 {
		t.Errorf("expected cycles [87,89], got [%d,%d]", got[0].Cycle, got[1].Cycle)
	}
}

func TestListPendingByClass_SkipsExpiredEntries(t *testing.T) {
	now := time.Date(2026, 5, 26, 0, 0, 0, 0, time.UTC)
	pastExpiry := now.Add(-1 * time.Hour).Format(time.RFC3339)
	futureExpiry := now.Add(1 * time.Hour).Format(time.RFC3339)
	entries := []Entry{
		{Cycle: 50, Classification: CodeAuditFail, ExpiresAt: pastExpiry, Verdict: "FAIL", Summary: "expired"},
		{Cycle: 51, Classification: CodeAuditFail, ExpiresAt: futureExpiry, Verdict: "FAIL", Summary: "live"},
	}
	got := ListPendingByClass(entries, CodeAuditFail, now)
	if len(got) != 1 {
		t.Fatalf("expected 1 live entry, got %d", len(got))
	}
	if got[0].Cycle != 51 {
		t.Errorf("expected live cycle=51, got %d", got[0].Cycle)
	}
}

func TestListPendingByClass_PreservesOrder(t *testing.T) {
	now := time.Date(2026, 5, 26, 0, 0, 0, 0, time.UTC)
	future := now.Add(24 * time.Hour).Format(time.RFC3339)
	entries := []Entry{
		{Cycle: 99, Classification: CodeAuditFail, ExpiresAt: future},
		{Cycle: 5, Classification: CodeAuditFail, ExpiresAt: future},
		{Cycle: 42, Classification: CodeAuditFail, ExpiresAt: future},
	}
	got := ListPendingByClass(entries, CodeAuditFail, now)
	if len(got) != 3 {
		t.Fatalf("expected 3, got %d", len(got))
	}
	if got[0].Cycle != 99 || got[1].Cycle != 5 || got[2].Cycle != 42 {
		t.Errorf("input order must be preserved, got [%d,%d,%d]", got[0].Cycle, got[1].Cycle, got[2].Cycle)
	}
}

func TestListPendingByClass_WorksForAnyClassification(t *testing.T) {
	now := time.Date(2026, 5, 26, 0, 0, 0, 0, time.UTC)
	future := now.Add(24 * time.Hour).Format(time.RFC3339)
	entries := []Entry{
		{Cycle: 1, Classification: CodeBuildFail, ExpiresAt: future},
		{Cycle: 2, Classification: InfraSystemic, ExpiresAt: future},
		{Cycle: 3, Classification: CodeBuildFail, ExpiresAt: future},
	}
	got := ListPendingByClass(entries, CodeBuildFail, now)
	if len(got) != 2 {
		t.Errorf("expected 2 build-fail entries, got %d", len(got))
	}
}
