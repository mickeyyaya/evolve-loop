package audit

import (
	"context"
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/explanationdocs"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/reportdoc"
)

// validateExplanationReview applies audit's policy around the shared review
// contract (explanationdocs.ValidateReviewedHandoff). Since 2026-09-13
// (ADR-0102, operator decision) the reviewer's reasoning is the gate and the
// section's shape is advisory: the returned advisories ride the phase record
// as warnings and never touch the verdict. The error — the only blocking
// outcome — is reserved for a missing reasoning (the Evidence floor; a
// missing or duplicated review section is no review text at all), a missing
// Build delivery reviewed as anything but FAIL, and host-side defects in the
// handoff itself. Before this, cycles 1638 and 1640 (2026-09-13) were burned
// by a PASS narrative overridden on citation form alone.
func validateExplanationReview(report string, req core.PhaseRequest) (advisories []string, err error) {
	if req.ExplanationDocumentationVersion == 0 {
		return nil, nil
	}
	section := phasecontract.ExplanationDocumentation // the contract's declaration; the deliverable gate reads the same one
	fields, advisories, err := reportdoc.ReasonedReview(report, "audit-report.md", section.Title(), "Status", "Build status", "Document", "Document SHA256", "Evidence")
	if err != nil || fields == nil {
		return advisories, err
	}
	view := req.BuildExplanation
	if req.BuildExplanationState != core.BuildExplanationAvailable || view == nil {
		if fields["status"] != "FAIL" {
			return nil, fmt.Errorf("missing Build explanation handoff must be reported with Status: FAIL%s", explanationdocs.HostReason(req.BuildExplanationError))
		}
		return nil, nil
	}
	review, err := explanationdocs.ValidateReviewedHandoff(context.Background(), fields, view, req.Worktree, req.WorktreeBaseSHA)
	if err != nil {
		return review.Advisories, err // the findings made before the host defect still ride the record
	}
	advisories = append(advisories, review.Advisories...)
	if review.Status == "NEEDS_CORRECTION" {
		advisories = append(advisories, "explanation documentation review NEEDS_CORRECTION: "+strings.TrimSpace(fields["evidence"]))
	}
	return advisories, nil
}
