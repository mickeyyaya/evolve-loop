package cliupdate_test

import (
	"context"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliupdate"
)

func TestUpdateAmplified_ANegativeBudgetFallsBackToTheDefaultDeadline(t *testing.T) {
	h := &fakeHost{versions: map[string][]string{"claude": {"2.1.285", "2.1.286"}}}
	seams := h.seams()
	seams.Budget = -time.Minute

	res := only(t, cliupdate.Update(context.Background(), []cliupdate.Family{claude()}, seams, nil))

	if res.Status != cliupdate.StatusUpdated {
		t.Fatalf("a negative budget must not cut a healthy family: %+v", res)
	}
}

func TestUpdateAmplified_NoFamiliesYieldsNoResults(t *testing.T) {
	h := &fakeHost{versions: map[string][]string{}}
	seams := h.seams()
	seams.Budget = shortBoundaryBudget

	rep := cliupdate.Update(context.Background(), nil, seams, nil)

	if len(rep.Results) != 0 || len(rep.Failed()) != 0 {
		t.Fatalf("an empty boundary reports nothing: %+v", rep)
	}
}
