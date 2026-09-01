package explanationdocs

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phaseio"
)

// TestValidateReviewedHandoff_ContractCore names the single home of the
// review contract the audit and retro gates both delegate to (architecture
// review 2026-09-01: previously each phase restated the enum, build-status
// echo, and document-match switch, and the copies had nothing binding them).
// The happy path through the evidence check is exercised end-to-end by the
// audit and retro package suites; this table pins the contract rejections,
// including the rejecting default for an unknown handoff status — new
// behavior: before the extraction an unknown status fell through both phase
// switches silently.
func TestValidateReviewedHandoff_ContractCore(t *testing.T) {
	required := &phaseio.ExplanationView{Status: "required", DocumentPath: "d.md", DocumentSHA256: "abc"}
	na := &phaseio.ExplanationView{Status: "not_applicable"}
	cases := []struct {
		name    string
		fields  map[string]string
		view    *phaseio.ExplanationView
		wantErr string
	}{
		{
			name:    "unknown review status rejected",
			fields:  map[string]string{"status": "MAYBE"},
			view:    required,
			wantErr: "must be VERIFIED or NEEDS_CORRECTION",
		},
		{
			name:    "build status echo mismatch",
			fields:  map[string]string{"status": "VERIFIED", "build status": "not_applicable"},
			view:    required,
			wantErr: "does not match the host Build handoff",
		},
		{
			name:    "required document mismatch",
			fields:  map[string]string{"status": "VERIFIED", "build status": "required", "document": "other.md", "document sha256": "abc"},
			view:    required,
			wantErr: "document does not match the host Build handoff",
		},
		{
			name:    "not-applicable must omit document",
			fields:  map[string]string{"status": "VERIFIED", "build status": "not_applicable", "document": "d.md"},
			view:    na,
			wantErr: "must omit Document and Document SHA256",
		},
		{
			name:    "unknown handoff status fails loudly",
			fields:  map[string]string{"status": "VERIFIED", "build status": "weird"},
			view:    &phaseio.ExplanationView{Status: "weird"},
			wantErr: "unknown status",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ValidateReviewedHandoff(context.Background(), tc.fields, tc.view, t.TempDir(), "")
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("want error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}
