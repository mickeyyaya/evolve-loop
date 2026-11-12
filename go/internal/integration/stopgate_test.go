package integration

import (
	"context"
	"errors"
	"slices"
	"testing"
)

type gateRig struct {
	*landRig
	reds     []PR
	redErr   error
	admits   [][2]string
	admitErr error
	landed   int
}

func newGateRig(headSHA string) *gateRig {
	g := &gateRig{landRig: newLandRig(headSHA)}
	gate := StopGate{
		Red: func(pr PR) error {
			g.reds = append(g.reds, pr)
			return g.redErr
		},
		Admit: func(_ context.Context, main, tip string) (func(), error) {
			g.admits = append(g.admits, [2]string{main, tip})
			return func() { g.landed++ }, g.admitErr
		},
	}
	g.merger.CI = gate.CI(g.ci)
	g.merger.Main = gate.Main(g.main)
	return g
}

func TestStopGate_AGreenBatchThatIsAdmittedLandsAndThenReportsTheLanding(t *testing.T) {
	t.Parallel()
	g := newGateRig(sha('0'))

	out, err := g.merger.Land(context.Background(), recs(1, 2))

	if err != nil || !slices.Equal(out.Landed, []int{1, 2}) || !slices.Equal(g.main.landed, []string{sha('b')}) {
		t.Fatalf("err %v landed %v ff %v, want the batch on main", err, out.Landed, g.main.landed)
	}
	if !slices.Equal(g.admits, [][2]string{{sha('0'), sha('b')}}) || g.landed != 1 || len(g.reds) != 0 {
		t.Errorf("admits %v landed %d reds %v, want one admission of the main head and the batch tip, one landing and no red", g.admits, g.landed, g.reds)
	}
}

func TestStopGate_AGreenBatchThatIsNotAdmittedDoesNotMoveMain(t *testing.T) {
	t.Parallel()
	g := newGateRig(sha('0'))
	g.admitErr = errors.New("a stop holds")

	out, err := g.merger.Land(context.Background(), recs(1, 2))

	if !errors.Is(err, g.admitErr) || len(out.Landed) != 0 || len(g.main.landed) != 0 || g.landed != 0 {
		t.Errorf("err %v landed %v ff %v landings %d, want the admission error and a main that did not move", err, out.Landed, g.main.landed, g.landed)
	}
}

func TestStopGate_ARedBatchThatTheStopHoldsGetsNoRerunSplitOrEjection(t *testing.T) {
	t.Parallel()
	g := newGateRig(sha('0'))
	g.ci.red = func(Batch, int) bool { return true }
	g.redErr = errors.New("a stop holds")

	out, err := g.merger.Land(context.Background(), recs(1, 2, 3, 4))

	if !errors.Is(err, g.redErr) || out.ConfirmedRed || len(out.Ejected) != 0 {
		t.Fatalf("err %v out %+v, want the hold error and no confirmed red", err, out)
	}
	if g.ci.reruns != 0 || len(g.calls) != 1 || len(g.ejected) != 0 || len(g.admits) != 0 {
		t.Errorf("reruns %d composes %d ejected %v admits %v, want the first red watch to end the batch", g.ci.reruns, len(g.calls), g.ejected, g.admits)
	}
	if !slices.Equal(g.reds, []PR{{Number: 1, Tip: sha('b')}}) {
		t.Errorf("reds %v, want the pull request of the batch one time", g.reds)
	}
}

func TestStopGate_ARedBatchWithNoHoldSplitsAsBefore(t *testing.T) {
	t.Parallel()
	g := newGateRig(sha('0'))
	g.ci.red = func(Batch, int) bool { return true }

	out, err := g.merger.Land(context.Background(), recs(1, 2))

	if err != nil || !out.ConfirmedRed || !slices.Equal(g.ejected, []int{1, 2}) || g.ci.reruns != 3 {
		t.Fatalf("err %v out %+v ejected %v reruns %d, want a split that ejects each deliverable", err, out, g.ejected, g.ci.reruns)
	}
	if len(g.reds) != 6 || len(g.admits) != 0 {
		t.Errorf("reds %d admits %v, want one report for each red watch and no admission", len(g.reds), g.admits)
	}
}

func TestStopGate_AWatchThatIsNotRedDoesNotAskTheStop(t *testing.T) {
	t.Parallel()
	failed := errors.New("the watch failed")
	for name, watchErr := range map[string]error{"green": nil, "a watch error": failed} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			g := newGateRig(sha('0'))
			g.ci.watchErr = watchErr
			g.redErr = errors.New("a stop holds")

			_, err := g.merger.Land(context.Background(), recs(1))

			if !errors.Is(err, watchErr) || len(g.reds) != 0 {
				t.Errorf("err %v reds %v, want the result of the inner watch and no red report", err, g.reds)
			}
		})
	}
}

func TestStopGate_AFastForwardThatFailsReportsNoLanding(t *testing.T) {
	t.Parallel()
	g := newGateRig(sha('0'))
	g.main.ffErr = errors.New("the push was refused")

	_, err := g.merger.Land(context.Background(), recs(1))

	if !errors.Is(err, g.main.ffErr) || g.landed != 0 || len(g.admits) != 1 {
		t.Errorf("err %v landings %d admits %v, want the push error after one admission and no landing", err, g.landed, g.admits)
	}
}

type headless struct{ Main }

func (headless) Head(context.Context) (string, error) { return "", errors.New("no main head") }

func TestStopGate_AMainHeadThatCannotBeReadAdmitsNothing(t *testing.T) {
	t.Parallel()
	g := newGateRig(sha('0'))
	gated := StopGate{Admit: func(context.Context, string, string) (func(), error) {
		t.Error("the gate asked for an admission with no main head")
		return func() {}, nil
	}}.Main(headless{g.main})

	if err := gated.FastForward(context.Background(), sha('b')); err == nil || len(g.main.landed) != 0 {
		t.Errorf("err %v ff %v, want the head error and a main that did not move", err, g.main.landed)
	}
}

func TestStopGate_TheOtherPortMethodsPassThrough(t *testing.T) {
	t.Parallel()
	g := newGateRig(sha('0'))
	ctx := context.Background()

	pr, err := g.merger.CI.Open(ctx, Batch{Tip: sha('b')})
	if err != nil || pr.Number != 1 || pr.Tip != sha('b') || len(g.ci.opened) != 1 {
		t.Errorf("Open = %+v, %v, want the pull request of the inner port", pr, err)
	}
	if err := g.merger.CI.Rerun(ctx, pr); err != nil || g.ci.reruns != 1 {
		t.Errorf("Rerun = %v with %d inner reruns, want one", err, g.ci.reruns)
	}
	if head, err := g.merger.Main.Head(ctx); err != nil || head != sha('0') {
		t.Errorf("Head = %q, %v, want the head of the inner port", head, err)
	}
	if len(g.reds) != 0 || len(g.admits) != 0 {
		t.Errorf("reds %v admits %v, want no stop call", g.reds, g.admits)
	}
}
