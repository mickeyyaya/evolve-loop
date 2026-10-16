package explanationdocs

import (
	"context"
	"errors"
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/phaseio"
	"github.com/mickeyyaya/evolve-loop/go/internal/reportdoc"
)

type Review struct {
	Status     string
	Advisories []string
}

const AdvisoryPrefix = "explanation documentation review (advisory, ADR-0102): "

func HostReason(hostError string) string {
	if hostError == "" {
		return ""
	}
	return " (host: " + hostError + ")"
}

func (r *Review) advise(finding string) { r.Advisories = append(r.Advisories, finding) }

func ValidateReviewedHandoff(ctx context.Context, fields map[string]string, view *phaseio.ExplanationView, worktree, baseSHA string) (Review, error) {
	if view == nil {
		return Review{}, errors.New("build explanation handoff view is missing")
	}
	var r Review
	switch status := fields["status"]; status {
	case "VERIFIED", "NEEDS_CORRECTION":
		r.Status = status
	default:
		r.advise("explanation documentation review Status must be VERIFIED or NEEDS_CORRECTION")
	}
	if fields["build status"] != view.Status {
		r.advise("explanation documentation review build status does not match the host Build handoff")
	}
	switch view.Status {
	case statusRequired:
		if fields["document"] != view.DocumentPath || fields["document sha256"] != view.DocumentSHA256 {
			r.advise("explanation documentation review document does not match the host Build handoff")
		}
	case statusNA:
		if fields["document"] != "" || fields["document sha256"] != "" {
			r.advise("not-applicable explanation documentation review must omit Document and Document SHA256")
		}
	default:
		return r, fmt.Errorf("build explanation handoff carries unknown status %q", view.Status)
	}
	references := append([]string{view.DocumentPath}, view.MaterialPaths...)
	if err := reportdoc.RequirePathLineEvidenceAt(ctx, worktree, baseSHA, fields["evidence"], references...); err != nil {
		r.advise(err.Error())
	}
	return r, nil
}
