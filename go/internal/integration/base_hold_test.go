package integration

import (
	"context"
	"errors"
	"slices"
	"testing"
)

func stepsOf(r *landRig) *[]Step {
	var steps []Step
	r.merger.Progress = func(s Step) { steps = append(steps, s) }
	return &steps
}

func kinds(steps []Step) []StepKind {
	out := make([]StepKind, len(steps))
	for i, s := range steps {
		out[i] = s.Kind
	}
	return out
}

func TestMergerHoldsARedBatchWhenTheBasePortRefuses(t *testing.T) {
	t.Parallel()
	r := newLandRig(sha('0'))
	r.ci.red = func(Batch, int) bool { return true }
	steps := stepsOf(r)
	refusal := &MainRefusal{SHA: sha('0'), Reason: "red"}
	var asked []string
	r.merger.Base = func(_ context.Context, base string) error {
		asked = append(asked, base)
		return refusal
	}
	out, err := r.merger.Land(context.Background(), recs(1, 2))
	if !errors.Is(err, refusal) {
		t.Fatalf("Land = %v, want the refusal", err)
	}
	if !slices.Equal(asked, []string{sha('0')}) {
		t.Errorf("the port was asked %v, want the base of the batch once", asked)
	}
	if len(r.calls) != 1 || len(r.ejected) != 0 || len(out.Landed) != 0 || len(r.main.landed) != 0 {
		t.Errorf("composes %v ejected %v out %+v: a hold must not split, eject or land", r.calls, r.ejected, out)
	}
	if !errors.Is((*steps)[len(*steps)-1].Err, refusal) {
		t.Errorf("the hold step carries %v, want the refusal", (*steps)[len(*steps)-1].Err)
	}
	if !out.ConfirmedRed {
		t.Error("the batch was confirmed red before the hold")
	}
	last := (*steps)[len(*steps)-1]
	if slices.Contains(kinds(*steps), StepSplit) || last.Kind != StepHold || last.Base != sha('0') || last.Batch != 1 || last.Tip != r.ci.opened[0].Tip || !slices.Equal(last.PRs, []int{1, 2}) {
		t.Errorf("steps %+v, want a last hold step with the base, the batch and the tip, and no split", *steps)
	}
}

func TestMergerStopsOnAnyErrorOfTheBasePort(t *testing.T) {
	t.Parallel()
	r := newLandRig(sha('0'))
	r.ci.red = func(Batch, int) bool { return true }
	steps := stepsOf(r)
	boom := errors.New("gh down")
	r.merger.Base = func(context.Context, string) error { return boom }
	if _, err := r.merger.Land(context.Background(), recs(1, 2)); !errors.Is(err, boom) {
		t.Fatalf("Land = %v, want the port error", err)
	}
	if slices.Contains(kinds(*steps), StepSplit) || kinds(*steps)[len(*steps)-1] != StepHold || !errors.Is((*steps)[len(*steps)-1].Err, boom) {
		t.Errorf("steps %v, want a hold and no split", kinds(*steps))
	}
}

func TestMergerDoesNotAskTheBasePortOnAGreenVerdictOrAFlake(t *testing.T) {
	t.Parallel()
	for _, red := range []func(Batch, int) bool{nil, func(_ Batch, run int) bool { return run == 1 }} {
		r := newLandRig(sha('0'))
		r.ci.red = red
		asked := 0
		r.merger.Base = func(context.Context, string) error { asked++; return errors.New("must not be asked") }
		out, err := r.merger.Land(context.Background(), recs(1, 2))
		if err != nil || asked != 0 || !slices.Equal(out.Landed, []int{1, 2}) {
			t.Errorf("Land = %+v, %v, asked %d: a green or flaky batch must land and never ask the base", out, err, asked)
		}
	}
}

func TestMergerSplitsAsBeforeWhenTheBasePortIsNilOrAccepts(t *testing.T) {
	t.Parallel()
	for _, port := range []func(context.Context, string) error{nil, func(context.Context, string) error { return nil }} {
		r := newLandRig(sha('0'))
		r.merger.Base = port
		r.ci.red = func(b Batch, _ int) bool { return len(r.calls) == 1 }
		out, err := r.merger.Land(context.Background(), recs(1, 2))
		if err != nil || len(r.calls) != 3 || !slices.Equal(out.Landed, []int{1, 2}) || !out.ConfirmedRed {
			t.Errorf("Land = %+v, %v, composes %v, want the split of today", out, err, r.calls)
		}
	}
}

func TestMergerKeepsTheLandedFirstHalfWhenTheSecondHalfHolds(t *testing.T) {
	t.Parallel()
	r := newLandRig(sha('0'))
	badTips := map[string]bool{}
	r.merger.Compose = func(_ context.Context, rs []DeliverableRecord) (Composition, error) {
		r.calls = append(r.calls, cyclesOf(rs))
		head, _ := r.main.Head(context.Background())
		tip := sha(byte('a' + len(r.calls)))
		if slices.Contains(cyclesOf(rs), 3) {
			badTips[tip] = true
		}
		return Composition{Batch: Batch{Wave: "1", Index: len(r.calls), Base: head, Branch: "integration/w1-b1", Tip: tip}, Kept: rs}, nil
	}
	r.ci.red = func(b Batch, _ int) bool { return badTips[b.Tip] || len(r.calls) == 1 }
	var asked []string
	refusal := &MainRefusal{SHA: sha('9'), Reason: "red"}
	r.merger.Base = func(_ context.Context, base string) error {
		asked = append(asked, base)
		if len(asked) == 2 {
			return refusal
		}
		return nil
	}
	out, err := r.merger.Land(context.Background(), recs(1, 2, 3, 4))
	if !errors.Is(err, refusal) {
		t.Fatalf("Land = %v, want the refusal of the second level", err)
	}
	if !slices.Equal(out.Landed, []int{1, 2}) || len(r.ejected) != 0 {
		t.Errorf("landed %v ejected %v, want the first half landed and nothing ejected", out.Landed, r.ejected)
	}
	if len(asked) != 2 {
		t.Errorf("the port was asked %d times, want once per red level", len(asked))
	}
}

func TestMergerStepsNameTheTipOfTheirBatch(t *testing.T) {
	t.Parallel()
	r := newLandRig(sha('0'))
	r.ci.red = func(_ Batch, run int) bool { return run == 1 }
	steps := stepsOf(r)
	if _, err := r.merger.Land(context.Background(), recs(1)); err != nil {
		t.Fatal(err)
	}
	tip := r.ci.opened[0].Tip
	for _, s := range *steps {
		if s.Tip != tip {
			t.Errorf("step %+v has no tip %s", s, tip)
		}
	}
}
