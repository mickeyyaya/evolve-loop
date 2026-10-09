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

func validateExplanationReview(report string, req core.PhaseRequest) (advisories []string, err error) {
	if req.ExplanationDocumentationVersion == 0 {
		return nil, nil
	}
	section := phasecontract.ExplanationDocumentation
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
		return review.Advisories, err
	}
	advisories = append(advisories, review.Advisories...)
	if review.Status == "NEEDS_CORRECTION" {
		advisories = append(advisories, "explanation documentation review NEEDS_CORRECTION: "+strings.TrimSpace(fields["evidence"]))
	}
	return advisories, nil
}
