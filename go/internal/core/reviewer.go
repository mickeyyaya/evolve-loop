package core

import (
	"context"
	"io"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type ReviewInput struct {
	Cycle                           int
	RunID                           string
	ExplanationDocumentationVersion int
	WorktreeBaseSHA                 string

	Phase          string
	Response       PhaseResponse
	Workspace      string
	Worktree       string
	ProjectRoot    string
	ChallengeToken string
	BreakerExempt  bool
	Protected      func(string) bool
	Stderr         io.Writer
	Signals        *signalcenter.Center
}

func (in ReviewInput) errOut() io.Writer {
	if in.Stderr == nil {
		return os.Stderr
	}
	return in.Stderr
}

type ReviewResult struct {
	Approve           bool
	Reason            string
	Retry             bool
	Demoted           bool
	Blocks            int
	Remediation       string
	DeliverableAbsent bool
	Failures          []string
}

type DeliverableReviewer interface {
	Review(ctx context.Context, in ReviewInput) ReviewResult
}

type mandatoryExplanationReviewer struct {
	next  DeliverableReviewer
	floor BuildHandoffFloor
}

func withMandatoryExplanationReviewer(next DeliverableReviewer) DeliverableReviewer {
	if next == nil {
		next = noopReviewer{}
	}
	return mandatoryExplanationReviewer{next: next, floor: MandatoryBuildHandoffFloor()}
}

func (r mandatoryExplanationReviewer) Review(ctx context.Context, in ReviewInput) ReviewResult {
	if in.Phase != string(PhaseBuild) || in.ExplanationDocumentationVersion == 0 {
		return r.next.Review(ctx, in)
	}
	if floor := r.floor.Review(ctx, in); !floor.Approve {
		return floor
	}
	optional := r.next.Review(ctx, in)
	if !optional.Approve {
		return optional
	}
	sealed := NewExplanationLifecycleReviewer().Review(ctx, in)
	if !sealed.Approve {
		return carryDemotion(sealed, optional)
	}
	return optional
}

type ContractVerification struct {
	OK           bool
	ArtifactPath string
	Violations   []string
}

type ContractVerifier interface {
	VerifyDeliverable(ctx context.Context, in ReviewInput) (ContractVerification, error)
}

func ChainReviewers(reviewers ...DeliverableReviewer) DeliverableReviewer {
	return chainReviewer(reviewers)
}

type chainReviewer []DeliverableReviewer

func (c chainReviewer) Review(ctx context.Context, in ReviewInput) ReviewResult {
	out := ReviewResult{Approve: true}
	for _, r := range c {
		if r == nil {
			continue
		}
		res := r.Review(ctx, in)
		if !res.Approve {
			return carryDemotion(res, out)
		}
		if res.Demoted && !out.Demoted {
			out.Demoted = true
			out.Reason = res.Reason
			out.Blocks = res.Blocks
		}
	}
	return out
}

func carryDemotion(decision, seen ReviewResult) ReviewResult {
	if !seen.Demoted {
		return decision
	}
	decision.Demoted = true
	if decision.Reason == "" {
		decision.Reason = seen.Reason
	}
	if decision.Blocks == 0 {
		decision.Blocks = seen.Blocks
	}
	return decision
}

type noopReviewer struct{}

func (noopReviewer) Review(_ context.Context, _ ReviewInput) ReviewResult {
	return ReviewResult{Approve: true}
}

type declaredDeliverablesGate interface{ VerifiesDeclaredDeliverables() bool }

func (o *Orchestrator) DeclaredDeliverablesGateWired() bool {
	return reviewerChainHas(o.reviewer, func(r DeliverableReviewer) bool {
		g, ok := r.(declaredDeliverablesGate)
		return ok && g.VerifiesDeclaredDeliverables()
	})
}

func reviewerChainHas(r DeliverableReviewer, pred func(DeliverableReviewer) bool) bool {
	switch v := r.(type) {
	case nil:
		return false
	case mandatoryExplanationReviewer:
		return reviewerChainHas(v.next, pred)
	case chainReviewer:
		for _, m := range v {
			if reviewerChainHas(m, pred) {
				return true
			}
		}
		return false
	default:
		return pred(r)
	}
}

type contractGateSignals interface {
	VerifiesDeclaredDeliverables() bool
	SignalsWired() bool
}

func (o *Orchestrator) ContractGateSignalsWired() bool {
	return reviewerChainHas(o.reviewer, func(r DeliverableReviewer) bool {
		g, ok := r.(contractGateSignals)
		return ok && g.VerifiesDeclaredDeliverables() && g.SignalsWired()
	})
}
