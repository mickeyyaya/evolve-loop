package explanationdocs

import (
	"context"
	"errors"
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/phaseio"
	"github.com/mickeyyaya/evolve-loop/go/internal/reportdoc"
)

// Review is what a reviewed explanation handoff yields. Since 2026-09-13
// (ADR-0102, operator decision) the reviewer's REASONING is the gate and the
// section's SHAPE is advisory: Status is the judgment as written (VERIFIED or
// NEEDS_CORRECTION; "" when the reviewer used another word), Advisories are
// the shape findings — the status enum, the build-status echo, the
// required/not_applicable document match, the path:line citations — which
// the caller reports on the phase record and which never touch a verdict.
type Review struct {
	Status     string
	Advisories []string
}

// AdvisoryPrefix is the ONE marker every advisory carries on a phase record —
// the audit and retro gates both prefix with it — so the operator's grep and
// any Signal Center filter see both phases' advisories the same way.
const AdvisoryPrefix = "explanation documentation review (advisory, ADR-0102): "

// HostReason renders the host's own account of a lost handoff for a finding,
// so a record distinguishes "the Build never delivered" from "the host lost
// the handoff after the audit saw it"; "" when the host has nothing to say.
func HostReason(hostError string) string {
	if hostError == "" {
		return ""
	}
	return " (host: " + hostError + ")"
}

func (r *Review) advise(finding string) { r.Advisories = append(r.Advisories, finding) }

// ValidateReviewedHandoff checks the phase-agnostic core of an explanation
// documentation review section against the Build handoff view and returns
// the Review; each caller applies its own phase policy to it. This is the
// single home of the contract — the audit and retro gates previously each
// restated it and had nothing binding the copies (architecture review
// 2026-09-01). The error is reserved for host-side defects no reviewer wrote
// — a missing view, an unknown handoff status — which must fail loudly here
// rather than fall through both phase gates silently.
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
