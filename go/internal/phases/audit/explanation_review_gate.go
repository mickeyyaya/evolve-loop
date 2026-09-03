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
// contract (explanationdocs.ValidateReviewedHandoff): a missing handoff must
// be reported as Status: FAIL, and NEEDS_CORRECTION blocks the audit.
func validateExplanationReview(report string, req core.PhaseRequest) error {
	if req.ExplanationDocumentationVersion == 0 {
		return nil
	}
	section := phasecontract.ExplanationDocumentation // the contract's declaration; the deliverable gate reads the same one
	body, found, err := reportdoc.Section(report, section.Title())
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("audit-report.md is missing %s", section.Canonical)
	}
	fields, err := reportdoc.ReviewFields(body, "Status", "Build status", "Document", "Document SHA256", "Evidence")
	if err != nil {
		return err
	}
	if err := reportdoc.RequirePathLineEvidence(fields["evidence"]); err != nil {
		return err
	}
	view := req.BuildExplanation
	if req.BuildExplanationState != core.BuildExplanationAvailable || view == nil {
		if fields["status"] != "FAIL" {
			return fmt.Errorf("missing Build explanation handoff must be reported with Status: FAIL")
		}
		return nil
	}
	status, err := explanationdocs.ValidateReviewedHandoff(context.Background(), fields, view, req.Worktree, req.WorktreeBaseSHA)
	if err != nil {
		return err
	}
	if status == "NEEDS_CORRECTION" {
		return fmt.Errorf("explanation documentation review NEEDS_CORRECTION: %s", strings.TrimSpace(fields["evidence"]))
	}
	return nil
}
