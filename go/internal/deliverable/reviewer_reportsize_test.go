package deliverable

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

// reviewer_reportsize_test.go — RED contract for wiring the new report-size
// budget check into the host-side contract gate (Reviewer). Mirrors the
// existing phaseIO threading exactly (TestReviewer_FailureContextPhaseIO_BlocksOnlyAtBothEnforce
// in reviewer_test.go): a new *Reviewer field pair, set directly in tests the
// same way newTestReviewerPhaseIO sets r.phaseIO.
//
// RED today: Reviewer has no reportSizeGate/reportSizeBudgetTokens fields
// (compile failure).

func newTestReviewerReportSize(stage, reportSizeGate config.Stage, budgetTokens int, breakerPath string, threshold int) *Reviewer {
	r := newTestReviewer(stage, breakerPath, threshold)
	r.reportSizeGate = reportSizeGate
	r.reportSizeBudgetTokens = budgetTokens
	return r
}

func TestReviewer_ReportSizeGate_BlocksOnlyAtEnforce(t *testing.T) {
	big := strings.Repeat("word ", 5000)
	report := "## Changes\n- x\nVerdict: PASS\n## Handoff Summary\n" + big
	for _, tc := range []struct {
		reportSizeGate config.Stage
		wantBlock      bool
	}{
		{config.StageOff, false},
		{config.StageShadow, false},
		{config.StageEnforce, true},
	} {
		t.Run(tc.reportSizeGate.String(), func(t *testing.T) {
			ws := t.TempDir()
			writeFile(t, ws, "build-report.md", report)
			r := newTestReviewerReportSize(config.StageEnforce, tc.reportSizeGate, 2000, filepath.Join(t.TempDir(), "b.json"), 3)
			got := r.Review(context.Background(), reviewInput("build", ws, t.TempDir()))
			if tc.wantBlock && got.Approve {
				t.Fatalf("reportSizeGate=%s: want BLOCK on oversized handoff section, got approve", tc.reportSizeGate)
			}
			if !tc.wantBlock && !got.Approve {
				t.Fatalf("reportSizeGate=%s: budget check must be dormant/log-only, want approve, got block (%s)", tc.reportSizeGate, got.Reason)
			}
		})
	}
}

func TestReviewer_ReportSizeGate_UnderBudgetNeverBlocks(t *testing.T) {
	report := "## Changes\n- x\nVerdict: PASS\n## Handoff Summary\nshort decision\n"
	ws := t.TempDir()
	writeFile(t, ws, "build-report.md", report)
	r := newTestReviewerReportSize(config.StageEnforce, config.StageEnforce, 2000, filepath.Join(t.TempDir(), "b.json"), 3)
	got := r.Review(context.Background(), reviewInput("build", ws, t.TempDir()))
	if !got.Approve {
		t.Errorf("a handoff section within budget must approve even at reportSizeGate=enforce; got %+v", got)
	}
}

// TestReviewer_ReportSizeGate_DefaultOff_ByteIdentical pins the rollout
// safety net: a Reviewer built through the existing constructors (zero-value
// reportSizeGate == StageOff) must never block on report size, whatever the
// content — the new dial is opt-in only until explicitly wired to "shadow"/
// "enforce" via policy.json, exactly like every other gate axis in this repo.
func TestReviewer_ReportSizeGate_DefaultOff_ByteIdentical(t *testing.T) {
	big := strings.Repeat("word ", 5000)
	report := "## Changes\n- x\nVerdict: PASS\n## Handoff Summary\n" + big
	ws := t.TempDir()
	writeFile(t, ws, "build-report.md", report)
	r := newTestReviewer(config.StageEnforce, filepath.Join(t.TempDir(), "b.json"), 3)
	got := r.Review(context.Background(), reviewInput("build", ws, t.TempDir()))
	if !got.Approve {
		t.Errorf("reportSizeGate defaults to off (zero value) — must not block; got %+v", got)
	}
}
