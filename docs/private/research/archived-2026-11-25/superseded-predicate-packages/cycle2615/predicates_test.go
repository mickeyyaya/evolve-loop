//go:build acs

package cycle2615

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
)

const (
	settleStep          = 5 * time.Second
	registersAfter      = 2 * time.Minute
	neverRegistersBound = 3 * time.Minute
	batchBase           = "1111111111111111111111111111111111111111"
	batchTip            = "2222222222222222222222222222222222222222"
)

type fakeClock struct {
	now      time.Time
	settles  int
	nowCalls int
}

func (c *fakeClock) Now() time.Time {
	c.nowCalls++
	return c.now
}

func (c *fakeClock) Settle(context.Context) error {
	c.settles++
	c.now = c.now.Add(settleStep)
	return nil
}

type fakeRunLister struct {
	clock        *fakeClock
	registerAt   time.Time
	never        bool
	listCalls    int
	watchedRunID string
}

func (l *fakeRunLister) gh(_ context.Context, _ string, args ...string) (string, int, error) {
	joined := " " + strings.Join(args, " ") + " "
	switch {
	case strings.Contains(joined, " pr list "):
		return "[]\n", 0, nil
	case strings.Contains(joined, " pr create "):
		return "https://github.com/o/r/pull/42\n", 0, nil
	case strings.Contains(joined, " run list "):
		l.listCalls++
		if l.never || l.clock.now.Before(l.registerAt) {
			return "[]\n", 0, nil
		}
		return `[{"databaseId":9001}]` + "\n", 0, nil
	case strings.Contains(joined, " run watch "):
		l.watchedRunID = args[2]
		return "", 0, nil
	case strings.Contains(joined, " run view "):
		return `{"conclusion":"success","status":"completed"}` + "\n", 0, nil
	}
	return "", 0, nil
}

type fakeMain struct {
	fastForwards []string
}

func (m *fakeMain) Head(context.Context) (string, error) { return batchBase, nil }

func (m *fakeMain) FastForward(_ context.Context, tip string) error {
	m.fastForwards = append(m.fastForwards, tip)
	return nil
}

type batchRig struct {
	clock  *fakeClock
	lister *fakeRunLister
	main   *fakeMain
	merger integration.Merger
}

func newBatchRig(t *testing.T, never bool, bound time.Duration) *batchRig {
	t.Helper()
	clock := &fakeClock{now: time.Date(2026, 11, 23, 9, 0, 0, 0, time.UTC)}
	lister := &fakeRunLister{clock: clock, registerAt: clock.now.Add(registersAfter), never: never}
	main := &fakeMain{}
	repo := integration.Repo{
		Dir: t.TempDir(),
		Git: func(context.Context, string, string, ...string) (string, error) { return "", nil },
	}
	ci := integration.GhCI{
		Repo:           repo,
		Gh:             lister.gh,
		Settle:         clock.Settle,
		Now:            clock.Now,
		RegisterWithin: bound,
	}
	batch := integration.Batch{Wave: "1", Index: 1, Base: batchBase, Branch: "integration/w1-b1", Tip: batchTip}
	merger := integration.Merger{
		CI:   ci,
		Main: main,
		Compose: func(_ context.Context, recs []integration.DeliverableRecord) (integration.Composition, error) {
			return integration.Composition{Batch: batch, Kept: recs}, nil
		},
		Eject: func(context.Context, integration.DeliverableRecord, error) error { return nil },
	}
	return &batchRig{clock: clock, lister: lister, main: main, merger: merger}
}

func oneDeliverable() []integration.DeliverableRecord {
	return []integration.DeliverableRecord{{Cycle: 7}}
}

func TestC2615_001_BatchWaitsForARunThatRegistersTwoMinutesAfterThePush(t *testing.T) {
	if integration.DefaultRegisterWithin <= registersAfter {
		t.Fatalf("DefaultRegisterWithin = %v, want more than the %v a late run needs", integration.DefaultRegisterWithin, registersAfter)
	}
	rig := newBatchRig(t, false, 0)
	out, err := rig.merger.Land(context.Background(), oneDeliverable())
	if err != nil {
		t.Fatalf("Land of a batch whose run registers after %v = %v, want the batch to wait and land", registersAfter, err)
	}
	if len(out.Landed) != 1 || len(rig.main.fastForwards) != 1 || rig.main.fastForwards[0] != batchTip {
		t.Errorf("landed %v fast-forwards %v, want the batch tip landed once", out.Landed, rig.main.fastForwards)
	}
	if rig.lister.watchedRunID != "9001" {
		t.Errorf("watched run %q, want the late run 9001", rig.lister.watchedRunID)
	}
	if waited := time.Duration(rig.clock.settles) * settleStep; waited < registersAfter {
		t.Errorf("waited %v on the injected clock, want at least %v", waited, registersAfter)
	}
}

func TestC2615_002_BatchWhoseRunNeverRegistersStopsAndNamesTheBound(t *testing.T) {
	rig := newBatchRig(t, true, neverRegistersBound)
	out, err := rig.merger.Land(context.Background(), oneDeliverable())
	if err == nil {
		t.Fatal("Land of a batch whose run never registers returned nil, want a stop")
	}
	if !errors.Is(err, integration.ErrNoRun) {
		t.Errorf("err = %v, want it to wrap ErrNoRun", err)
	}
	if !strings.Contains(err.Error(), neverRegistersBound.String()) {
		t.Errorf("err = %q, want the message to name the bound %s", err.Error(), neverRegistersBound)
	}
	if len(out.Landed) != 0 || len(rig.main.fastForwards) != 0 {
		t.Errorf("landed %v fast-forwards %v, want nothing landed", out.Landed, rig.main.fastForwards)
	}
	waited := time.Duration(rig.clock.settles) * settleStep
	if waited < neverRegistersBound || waited > neverRegistersBound+settleStep {
		t.Errorf("waited %v on the injected clock, want the bound %v to the nearest %v step", waited, neverRegistersBound, settleStep)
	}
}

func TestC2615_003_BothCasesRunOnTheInjectedClockAndFakeRunLister(t *testing.T) {
	cases := []struct {
		name    string
		never   bool
		bound   time.Duration
		wantErr bool
	}{
		{"late registration lands", false, 0, false},
		{"never registers stops", true, neverRegistersBound, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rig := newBatchRig(t, c.never, c.bound)
			_, err := rig.merger.Land(context.Background(), oneDeliverable())
			if (err != nil) != c.wantErr {
				t.Fatalf("Land err = %v, want failure=%v", err, c.wantErr)
			}
			if rig.clock.nowCalls == 0 {
				t.Error("the injected clock was never read, so the wait is not clock-driven")
			}
			if rig.lister.listCalls < 2 {
				t.Errorf("run lister called %d times, want it polled until the clock decided", rig.lister.listCalls)
			}
			if rig.clock.settles == 0 {
				t.Error("the injected settle was never used, so the wait is not injected")
			}
		})
	}
}

func TestC2615_004_ZeroBoundFallsBackToTheDefaultAndNamesIt(t *testing.T) {
	rig := newBatchRig(t, true, 0)
	_, err := rig.merger.Land(context.Background(), oneDeliverable())
	if err == nil || !errors.Is(err, integration.ErrNoRun) {
		t.Fatalf("Land = %v, want a stop that wraps ErrNoRun", err)
	}
	if !strings.Contains(err.Error(), integration.DefaultRegisterWithin.String()) {
		t.Errorf("err = %q, want the default bound %s named", err.Error(), integration.DefaultRegisterWithin)
	}
}
