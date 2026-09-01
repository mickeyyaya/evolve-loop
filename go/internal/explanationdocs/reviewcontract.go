package explanationdocs

import (
	"context"
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/phaseio"
	"github.com/mickeyyaya/evolve-loop/go/internal/reportdoc"
)

// ValidateReviewedHandoff checks the phase-agnostic core of an explanation
// documentation review section against the Build handoff view: the
// VERIFIED/NEEDS_CORRECTION status enum, the build-status echo, the
// required/not_applicable document match, and path:line evidence over the
// document plus material paths. It returns the validated status so each caller
// can apply its own phase policy (audit blocks on NEEDS_CORRECTION; retro
// accepts it when a carryover todo backs it). This is the single home of the
// contract — the audit and retro gates previously each restated it and had
// nothing binding the copies (architecture review 2026-09-01). The rejecting
// default is deliberate: an unknown handoff status must fail loudly here, not
// fall through both phase gates silently.
func ValidateReviewedHandoff(ctx context.Context, fields map[string]string, view *phaseio.ExplanationView, worktree, baseSHA string) (string, error) {
	status := fields["status"]
	if status != "VERIFIED" && status != "NEEDS_CORRECTION" {
		return "", fmt.Errorf("explanation documentation review Status must be VERIFIED or NEEDS_CORRECTION")
	}
	if fields["build status"] != view.Status {
		return "", fmt.Errorf("explanation documentation review build status does not match the host Build handoff")
	}
	switch view.Status {
	case statusRequired:
		if fields["document"] != view.DocumentPath || fields["document sha256"] != view.DocumentSHA256 {
			return "", fmt.Errorf("explanation documentation review document does not match the host Build handoff")
		}
	case statusNA:
		if fields["document"] != "" || fields["document sha256"] != "" {
			return "", fmt.Errorf("not-applicable explanation documentation review must omit Document and Document SHA256")
		}
	default:
		return "", fmt.Errorf("build explanation handoff carries unknown status %q", view.Status)
	}
	references := append([]string{view.DocumentPath}, view.MaterialPaths...)
	if err := reportdoc.RequirePathLineEvidenceAt(ctx, worktree, baseSHA, fields["evidence"], references...); err != nil {
		return "", err
	}
	return status, nil
}
