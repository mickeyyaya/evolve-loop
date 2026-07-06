package deliverable

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// reviewer_reportsize_ctor_test.go — the catalog-aware constructor
// NewReviewerWithCatalogStageReportSize (cycle-565 Slice S1) actually threads
// the report-size gate + budget onto the Reviewer, mirroring
// TestNewReviewerWithCatalogStage_ThreadsPhaseIO for the phaseIO dial: an
// oversized handoff section is blocked at reportSizeGate=enforce and approved
// (dormant) at off — proving the two new params are wired, not dropped. This is
// the production wiring the cmd_cycle.go call site uses, so it must be exercised
// through the public constructor (apicover named-coverage, cycle-542 lesson).
func TestNewReviewerWithCatalogStageReportSize_ThreadsGate(t *testing.T) {
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
			rev := NewReviewerWithCatalogStageReportSize(
				config.StageEnforce, userCatalogWithFoo(), config.StageOff, tc.reportSizeGate, 2000).(*Reviewer)
			rev.breakerPath = filepath.Join(t.TempDir(), "breaker.json")
			rev.logf = func(string, ...any) {}
			got := rev.Review(context.Background(), core.ReviewInput{Phase: "build", Workspace: ws, ProjectRoot: t.TempDir()})
			if tc.wantBlock && got.Approve {
				t.Errorf("reportSizeGate=%s: want BLOCK on oversized handoff, got approve", tc.reportSizeGate)
			}
			if !tc.wantBlock && !got.Approve {
				t.Errorf("reportSizeGate=%s: want approve (dormant), got block (%s)", tc.reportSizeGate, got.Reason)
			}
		})
	}
}
