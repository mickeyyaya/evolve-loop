package integration

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
)

func redBatchRig(t *testing.T) (*trainRig, Plan) {
	t.Helper()
	r := newTrainRig(t)
	r.pr(t, 1, "feat: a", map[string]string{"a.txt": "a\n"})
	r.pr(t, 2, "feat: b", map[string]string{"b.txt": "b\n"})
	r.train.CI = StopGate{
		Red:   func(PR) error { return nil },
		Admit: func(context.Context, string, string) (func(), error) { return func() {}, nil },
	}.CI(r.ci)
	r.ci.red = func(b Batch, _ int) bool {
		return strings.Contains(r.f.g.Git("ls-tree", "-r", "--name-only", b.Tip), "b.txt")
	}
	plan, err := r.train.Plan(context.Background(), []int{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	return r, plan
}

func assertHeld(t *testing.T, r *trainRig, out Outcome, err error) *MainRefusal {
	t.Helper()
	var refusal *MainRefusal
	if !errors.As(err, &refusal) {
		t.Fatalf("Land = %v, want a main refusal", err)
	}
	if len(out.Landed) != 0 || len(out.Ejected) != 0 || len(r.ci.opened) != 1 || len(r.host.comments) != 0 || len(r.host.closed) != 0 {
		t.Errorf("out %+v opened %d comments %v closed %v: a hold must not split, eject, comment or close", out, len(r.ci.opened), r.host.comments, r.host.closed)
	}
	return refusal
}

func TestTrainLandHoldsARedBatchWhenTheBaseIsRed(t *testing.T) {
	t.Parallel()
	r, plan := redBatchRig(t)
	stub := &mainCIStub{}
	r.train.MainCI = stub
	stub.verdict = Red
	before := r.originMain()
	out, err := r.train.Land(context.Background(), plan)
	refusal := assertHeld(t, r, out, err)
	if refusal.SHA != before || refusal.Reason != "red" || r.originMain() != before {
		t.Errorf("refusal %+v, main %s: want the base %s held at red and main unmoved", refusal, r.originMain(), before)
	}
	if !slices.Equal(stub.asked, []string{before}) {
		t.Errorf("the base was asked %v, want one check of the base", stub.asked)
	}
}

func TestTrainLandHoldsWhenThePendingBaseRunEndsRed(t *testing.T) {
	t.Parallel()
	r, plan := redBatchRig(t)
	polls := 0
	view := func() string {
		polls++
		if polls == 1 {
			return `{"conclusion":"","status":"in_progress"}` + "\n"
		}
		return `{"conclusion":"failure","status":"completed"}` + "\n"
	}
	script := &ghScript{reply: func(args []string) (string, int, error) {
		if has(args, "run", "view") {
			return view(), 0, nil
		}
		return runListJSON("31"), 0, nil
	}}
	settles := 0
	ci := newGhCI(t, script)
	ci.Settle = func(context.Context) error { settles++; return nil }
	r.train.MainCI = ci
	out, err := r.train.Land(context.Background(), plan)
	refusal := assertHeld(t, r, out, err)
	if refusal.Reason != "red" || settles != 1 {
		t.Errorf("refusal %+v settles %d, want a red hold after one wait on the pending run", refusal, settles)
	}
}

func TestTrainLandHoldsWhileThePeekedBaseIsStillPending(t *testing.T) {
	t.Parallel()
	r, plan := redBatchRig(t)
	r.train.MainCI = &mainCIStub{err: fmt.Errorf("%w: run 9", ErrRunPending)}
	out, err := r.train.Land(context.Background(), plan)
	if refusal := assertHeld(t, r, out, err); refusal.Reason != "pending" {
		t.Errorf("refusal %+v, want the reason pending", refusal)
	}
}

func TestTrainLandStopsWithoutASplitWhenTheBaseWaitTimesOut(t *testing.T) {
	t.Parallel()
	r, plan := redBatchRig(t)
	r.train.MainCI = &mainCIStub{err: ErrRunTimeout}
	out, err := r.train.Land(context.Background(), plan)
	var refusal *MainRefusal
	if !errors.Is(err, ErrRunTimeout) || errors.As(err, &refusal) {
		t.Fatalf("Land = %v, want the timeout and no main refusal", err)
	}
	if len(out.Landed) != 0 || len(out.Ejected) != 0 || len(r.ci.opened) != 1 {
		t.Errorf("out %+v opened %d, want no split", out, len(r.ci.opened))
	}
}

func TestTrainLandSplitsAsBeforeWhenTheBaseIsGreenOrOverridden(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name string
		stub mainCIStub
		fix  bool
	}{
		{"green", mainCIStub{verdict: Green}, false},
		{"red with the fix flag", mainCIStub{verdict: Red}, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			r, plan := redBatchRig(t)
			stub := c.stub
			r.train.MainCI, r.train.MainRedFix = &stub, c.fix
			out, err := r.train.Land(context.Background(), plan)
			if err != nil || !slices.Equal(out.Landed, []int{1}) || !slices.Equal(out.Ejected, []int{2}) {
				t.Errorf("Land = %+v, %v, want PR 1 landed and PR 2 ejected by the split", out, err)
			}
		})
	}
}
