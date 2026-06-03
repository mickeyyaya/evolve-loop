package core

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/mickeyyaya/evolve-loop/go/internal/failureadapter"
)

func TestReasonFromDiagnostics(t *testing.T) {
	tax := Taxonomy{
		Source:      "audit",
		FailureMode: "egps-red",
		Consequence: failureadapter.CodeAuditFail,
	}
	long := strings.Repeat("x", maxReasonSummaryLen+50)

	tests := []struct {
		name        string
		status      string
		diags       []Diagnostic
		tax         Taxonomy
		wantSummary string
		wantPass    bool
	}{
		{
			name:     "pass with no diagnostics has empty summary",
			status:   VerdictPASS,
			diags:    nil,
			wantPass: true,
		},
		{
			name:        "fail prefers the first error-severity diagnostic",
			status:      VerdictFAIL,
			diags:       []Diagnostic{{Severity: "warning", Message: "w1"}, {Severity: "error", Message: "EGPS: red_count=3"}},
			tax:         tax,
			wantSummary: "EGPS: red_count=3",
		},
		{
			name:        "fail falls back to a warning when no error present",
			status:      VerdictFAIL,
			diags:       []Diagnostic{{Severity: "warning", Message: "only a warning"}},
			wantSummary: "only a warning",
		},
		{
			name:        "non-pass with no diagnostics gets a status-derived default",
			status:      VerdictFAIL,
			diags:       nil,
			wantSummary: "unspecified FAIL",
		},
		{
			name:        "summary is truncated to the cap",
			status:      VerdictFAIL,
			diags:       []Diagnostic{{Severity: "error", Message: long}},
			wantSummary: long[:maxReasonSummaryLen],
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ReasonFromDiagnostics(tc.status, tc.diags, tc.tax)
			if got.Status != tc.status {
				t.Errorf("Status = %q, want %q", got.Status, tc.status)
			}
			if got.Summary != tc.wantSummary {
				t.Errorf("Summary = %q, want %q", got.Summary, tc.wantSummary)
			}
			if got.IsPass() != tc.wantPass {
				t.Errorf("IsPass() = %v, want %v", got.IsPass(), tc.wantPass)
			}
			if got.Taxonomy != tc.tax {
				t.Errorf("Taxonomy = %+v, want %+v", got.Taxonomy, tc.tax)
			}
			if len(got.Summary) > maxReasonSummaryLen {
				t.Errorf("Summary length %d exceeds cap %d", len(got.Summary), maxReasonSummaryLen)
			}
		})
	}
}

func TestReasonFromDiagnosticsTruncatesRuneSafe(t *testing.T) {
	// A multibyte message longer than the cap must be truncated on a rune
	// boundary — never producing invalid UTF-8.
	msg := strings.Repeat("é", maxReasonSummaryLen+10) // 2 bytes/rune
	got := ReasonFromDiagnostics(VerdictFAIL, []Diagnostic{{Severity: "error", Message: msg}}, Taxonomy{})
	if !utf8.ValidString(got.Summary) {
		t.Fatal("truncated summary is not valid UTF-8")
	}
	if n := utf8.RuneCountInString(got.Summary); n > maxReasonSummaryLen {
		t.Errorf("summary rune count %d exceeds cap %d", n, maxReasonSummaryLen)
	}
}

func TestTaxonomyIsZero(t *testing.T) {
	if !(Taxonomy{}).IsZero() {
		t.Error("empty Taxonomy should be zero")
	}
	if (Taxonomy{Source: "audit"}).IsZero() {
		t.Error("populated Taxonomy should not be zero")
	}
}

// TestConsequenceIsClassification documents the WS1↔WS2 contract: Taxonomy.Consequence
// is typed as failureadapter.Classification, so misuse is a compile error rather than a
// runtime mismatch. This test simply pins that the field accepts the canonical constants.
func TestConsequenceIsClassification(t *testing.T) {
	for _, c := range []failureadapter.Classification{
		failureadapter.CodeAuditFail, failureadapter.CodeBuildFail, failureadapter.InfraTransient,
	} {
		tx := Taxonomy{Consequence: c}
		if tx.Consequence != c {
			t.Errorf("Consequence round-trip failed for %q", c)
		}
	}
}
