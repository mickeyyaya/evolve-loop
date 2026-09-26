package deliverable

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

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
