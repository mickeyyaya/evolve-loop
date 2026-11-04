package cliupdate_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliupdate"
)

const (
	oneFamilyWorstCase  = 18 * time.Minute
	twoFamilyWorstCase  = 36 * time.Minute
	unboundedStubGuard  = 5 * time.Second
	shortBoundaryBudget = 30 * time.Millisecond
)

func TestDefaultBoundaryBudget_AdmitsOneFullFamilyAndCutsTheTwoFamilyWorstCase(t *testing.T) {
	if cliupdate.DefaultBoundaryBudget < oneFamilyWorstCase || cliupdate.DefaultBoundaryBudget >= twoFamilyWorstCase {
		t.Fatalf("DefaultBoundaryBudget = %s, want at least %s (a probe, an updater and a smoke of one family) and below %s (the unbounded two-family stall)",
			cliupdate.DefaultBoundaryBudget, oneFamilyWorstCase, twoFamilyWorstCase)
	}
}

func TestUpdate_EveryStepRunsUnderTheDefaultBoundaryDeadlineWhenTheCallerSetsNoBudget(t *testing.T) {
	h := &fakeHost{versions: map[string][]string{"claude": {"2.1.285", "2.1.286"}}}
	seams := h.seams()
	deadlines := map[string]time.Time{}
	record := func(step string, ctx context.Context) {
		if dl, ok := ctx.Deadline(); ok {
			deadlines[step] = dl
		}
	}
	probe, run, smoke := seams.Probe, seams.Run, seams.Smoke
	seams.Probe = func(ctx context.Context, family string) error { record("probe", ctx); return probe(ctx, family) }
	seams.Run = func(ctx context.Context, f cliupdate.Family) error { record("run", ctx); return run(ctx, f) }
	seams.Smoke = func(ctx context.Context, family string) error { record("smoke", ctx); return smoke(ctx, family) }
	begun := time.Now()

	res := only(t, cliupdate.Update(context.Background(), []cliupdate.Family{claude()}, seams, nil))

	if res.Status != cliupdate.StatusUpdated {
		t.Fatalf("an updated family inside the budget changed: %+v", res)
	}
	for _, step := range []string{"probe", "run", "smoke"} {
		dl, ok := deadlines[step]
		if !ok {
			t.Fatalf("the %s step ran with no deadline: nothing bounds the boundary stall", step)
		}
		if dl.After(begun.Add(cliupdate.DefaultBoundaryBudget + time.Minute)) {
			t.Errorf("the %s deadline %s is later than the default budget %s from the start", step, dl, cliupdate.DefaultBoundaryBudget)
		}
	}
}

func TestUpdate_ANonPositiveBudgetIsTheDefaultDeadlineNotAnExpiredOne(t *testing.T) {
	cases := []struct {
		name   string
		budget time.Duration
	}{
		{"one nanosecond below zero", -time.Nanosecond},
		{"one minute below zero", -time.Minute},
		{"the default budget negated", -cliupdate.DefaultBoundaryBudget},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := &fakeHost{versions: map[string][]string{"claude": {"2.1.285", "2.1.286"}}}
			seams := h.seams()
			seams.Budget = tc.budget
			var smokeDeadline time.Time
			smoke := seams.Smoke
			seams.Smoke = func(ctx context.Context, family string) error {
				smokeDeadline, _ = ctx.Deadline()
				return smoke(ctx, family)
			}
			begun := time.Now()

			res := only(t, cliupdate.Update(context.Background(), []cliupdate.Family{claude()}, seams, nil))

			if res.Status != cliupdate.StatusUpdated {
				t.Fatalf("budget %s cut a healthy family: %+v", tc.budget, res)
			}
			if smokeDeadline.Before(begun.Add(cliupdate.DefaultBoundaryBudget - time.Minute)) {
				t.Errorf("budget %s gave the smoke deadline %s, want about the default %s from the start", tc.budget, smokeDeadline, cliupdate.DefaultBoundaryBudget)
			}
		})
	}
}

func stallingUntilCut(h *fakeHost, seams *cliupdate.Seams) {
	seams.Run = func(ctx context.Context, f cliupdate.Family) error {
		h.calls = append(h.calls, "run "+strings.Join(f.UpdateArgv, " "))
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(unboundedStubGuard):
			return errors.New("the updater ran unbounded")
		}
	}
}

func TestUpdate_AStalledFamilyIsCutAtTheBudgetAndNoLaterFamilyStarts(t *testing.T) {
	h := &fakeHost{versions: map[string][]string{"claude": {"2.1.285", "2.1.285"}, "agy": {"1.2.17"}}}
	seams := h.seams()
	stallingUntilCut(h, &seams)
	seams.Budget = shortBoundaryBudget

	rep := cliupdate.Update(context.Background(), []cliupdate.Family{claude(), agy()}, seams, nil)

	if len(rep.Results) != 2 {
		t.Fatalf("want one result per family, got %+v", rep.Results)
	}
	for _, res := range rep.Results {
		if res.Status != cliupdate.StatusSkipped || !strings.Contains(res.Detail, "budget") || strings.HasPrefix(res.Detail, "interrupted") {
			t.Errorf("%s: want skipped with a detail naming the boundary budget, not an interrupt: %+v", res.Family, res)
		}
	}
	if h.called("eligible agy") || h.called("probe agy") || h.called("run agy") || h.called("smoke agy") {
		t.Errorf("a family after the cut started: calls=%q", h.calls)
	}
	if len(rep.Failed()) != 0 || len(rep.SmokeFailed()) != 0 {
		t.Errorf("a budget cut is a skip, never a failure or a halt: %+v", rep)
	}
}

func TestUpdate_ABudgetCutAfterTheUpdaterIsNeverASmokeFailure(t *testing.T) {
	h := &fakeHost{versions: map[string][]string{"claude": {"2.1.285", "2.1.286"}}}
	seams := h.seams()
	seams.Budget = shortBoundaryBudget
	seams.Smoke = func(ctx context.Context, family string) error {
		h.calls = append(h.calls, "smoke "+family)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(unboundedStubGuard):
			return errors.New("the smoke ran unbounded")
		}
	}

	rep := cliupdate.Update(context.Background(), []cliupdate.Family{claude()}, seams, nil)

	res := only(t, rep)
	if res.Status != cliupdate.StatusSkipped || !strings.Contains(res.Detail, "budget") || len(rep.SmokeFailed()) != 0 {
		t.Fatalf("a smoke the budget cut is skipped, never smoke-failed: %+v", res)
	}
}

func TestUpdate_ACancelledBoundaryStaysAnInterruptAndNamesNoBudget(t *testing.T) {
	h := &fakeHost{versions: map[string][]string{"claude": {"2.1.285", "2.1.286"}}}
	seams := h.seams()
	seams.Budget = time.Hour
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res := only(t, cliupdate.Update(ctx, []cliupdate.Family{claude()}, seams, nil))

	if res.Status != cliupdate.StatusSkipped || !strings.HasPrefix(res.Detail, "interrupted") || strings.Contains(res.Detail, "budget") {
		t.Fatalf("an operator interrupt keeps its own reading: %+v", res)
	}
}

func TestUpdate_AFamilyThatFinishesInsideTheBudgetIsReportedAsBefore(t *testing.T) {
	h := &fakeHost{versions: map[string][]string{"claude": {"2.1.285", "2.1.286"}}}
	seams := h.seams()
	seams.Budget = time.Hour

	res := only(t, cliupdate.Update(context.Background(), []cliupdate.Family{claude()}, seams, nil))

	if res.Status != cliupdate.StatusUpdated || res.OldVersion != "2.1.285" || res.NewVersion != "2.1.286" || res.Detail != "" {
		t.Fatalf("a budget that is never reached changes nothing: %+v", res)
	}
}
