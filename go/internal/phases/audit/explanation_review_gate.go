package audit

import (
	"context"
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/reportdoc"
)

func validateExplanationReview(report string, req core.PhaseRequest) error {
	if req.ExplanationDocumentationVersion == 0 {
		return nil
	}
	body, found, err := reportdoc.Section(report, "Explanation Documentation")
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("audit-report.md is missing ## Explanation Documentation")
	}
	fields, err := reportdoc.Fields(body, "Status", "Build status", "Document", "Document SHA256", "Evidence")
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
	status := fields["status"]
	if status != "VERIFIED" && status != "NEEDS_CORRECTION" {
		return fmt.Errorf("explanation documentation review Status must be VERIFIED or NEEDS_CORRECTION")
	}
	if fields["build status"] != view.Status {
		return fmt.Errorf("explanation documentation review build status does not match the host Build handoff")
	}
	switch view.Status {
	case "required":
		if fields["document"] != view.DocumentPath || fields["document sha256"] != view.DocumentSHA256 {
			return fmt.Errorf("explanation documentation review document does not match the host Build handoff")
		}
	case "not_applicable":
		if fields["document"] != "" || fields["document sha256"] != "" {
			return fmt.Errorf("not-applicable explanation documentation review must omit Document and Document SHA256")
		}
	}
	references := append([]string{view.DocumentPath}, view.MaterialPaths...)
	if err := reportdoc.RequirePathLineEvidenceAt(context.Background(), req.Worktree, req.WorktreeBaseSHA, fields["evidence"], references...); err != nil {
		return err
	}
	if status == "NEEDS_CORRECTION" {
		return fmt.Errorf("explanation documentation review NEEDS_CORRECTION: %s", strings.TrimSpace(fields["evidence"]))
	}
	return nil
}
