package core

import (
	"context"
	"testing"
)

type stubReviewer struct {
	result ReviewResult
	calls  *int
}

func (s stubReviewer) Review(_ context.Context, _ ReviewInput) ReviewResult {
	if s.calls != nil {
		*s.calls++
	}
	return s.result
}

// ChainReviewers composes multiple DeliverableReviewers (e.g. evalgate then the
// deliverable-contract gate). It approves only when ALL approve; the first
// rejection short-circuits with its reason. ADR-0034.

func TestChainReviewers_AllApprove(t *testing.T) {
	t.Parallel()
	c := ChainReviewers(
		stubReviewer{result: ReviewResult{Approve: true}},
		stubReviewer{result: ReviewResult{Approve: true}},
	)
	if got := c.Review(context.Background(), ReviewInput{}); !got.Approve {
		t.Errorf("all approve → approve; got %+v", got)
	}
}

func TestChainReviewers_FirstRejectionShortCircuits(t *testing.T) {
	t.Parallel()
	second := 0
	c := ChainReviewers(
		stubReviewer{result: ReviewResult{Approve: false, Reason: "nope"}},
		stubReviewer{result: ReviewResult{Approve: true}, calls: &second},
	)
	got := c.Review(context.Background(), ReviewInput{})
	if got.Approve || got.Reason != "nope" {
		t.Errorf("first rejection should win; got %+v", got)
	}
	if second != 0 {
		t.Errorf("short-circuit: later reviewers must not run after a rejection; ran %d times", second)
	}
}

func TestChainReviewers_SkipsNil(t *testing.T) {
	t.Parallel()
	c := ChainReviewers(nil, stubReviewer{result: ReviewResult{Approve: true}}, nil)
	if got := c.Review(context.Background(), ReviewInput{}); !got.Approve {
		t.Errorf("nil reviewers skipped; got %+v", got)
	}
}
