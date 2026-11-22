package ship

import (
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/mainstop"
	"github.com/mickeyyaya/evolve-loop/go/internal/selfheal"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
)

const failedCIJob = "build+test=failure"

func (l *baseRedLane) holdCIStop(t *testing.T) mainstop.Stop {
	t.Helper()
	if _, err := core.OpenMainCIRedStop(l.repo, core.MainCIRed{Main: l.base, Failed: []string{failedCIJob}}); err != nil {
		t.Fatalf("open the CI stop: %v", err)
	}
	return l.stop(t)
}

func (l *baseRedLane) releaseCIStop(t *testing.T) {
	t.Helper()
	opts := l.options(nil)
	err := selfheal.StopRelease{
		EvolveDir: opts.evolveDir(), Now: time.Now(), Emit: l.signals.Emit,
		Consume: func(string) error { return nil },
	}.Keys(l.stop(t).Keys, "main "+l.base)
	if err != nil {
		t.Fatalf("release the CI stop: %v", err)
	}
}

func TestShip_RefusesLandingWhileOriginMainCIRed(t *testing.T) {
	l := newBaseRedLane(t)
	l.landable(t)
	l.enforceStop(t)
	held := l.holdCIStop(t)
	swapRepoContractTest(t, greenPack())

	err := l.ship(t)

	se := wantShipErr(t, err, shiperr.CodeMainCIRed, core.ShipClassPrecondition, failedCIJob)
	if se.Code == shiperr.CodeMainRedPark {
		t.Errorf("code = %s, want the CI cause and not the lane park", se.Code)
	}
	if l.mainTip(t) != l.base || !l.stop(t).Holds() {
		t.Errorf("main = %s (base %s) stop holds = %v, want nothing landed and the stop held", l.mainTip(t), l.base, l.stop(t).Holds())
	}
	if !l.fixItemPending(t, held.FixItem) {
		t.Errorf("the fix item %s is not pending after a refusal", held.FixItem)
	}
}

func TestShip_RefusesLandingWhileOriginMainCIRedWithoutRunningTheKeysOfTheStop(t *testing.T) {
	t.Parallel()
	l := newBaseRedLane(t)
	l.enforceStop(t)
	l.holdCIStop(t)
	run := &scriptedBase{answers: [][]baseattr.Key{nil}}

	err, log := stopChecked(t, l.options(run.run))

	wantShipErr(t, err, shiperr.CodeMainCIRed, core.ShipClassPrecondition, failedCIJob)
	if run.calls != 0 || strings.Contains(log, "could not be run in the lane") {
		t.Errorf("the proof ran %d times, log = %q; a CI key names a job, not a test, so no lane can run it", run.calls, log)
	}
}

func TestShip_RefusesALaneOffTheMainTipWithTheCICauseNotARebase(t *testing.T) {
	t.Parallel()
	l := newBaseRedLane(t)
	l.holdCIStop(t)
	l.landOnMain(t, "main moves on", map[string]string{"docs/note.md": "a note\n"})
	l.enforceStop(t)

	err, _ := stopChecked(t, l.options((&scriptedBase{answers: [][]baseattr.Key{nil}}).run))

	wantShipErr(t, err, shiperr.CodeMainCIRed, core.ShipClassPrecondition, failedCIJob)
}

func TestShip_ShadowLandsWhileOriginMainCIRedAndRecordsTheRefusal(t *testing.T) {
	l := newBaseRedLane(t)
	l.landable(t)
	l.holdCIStop(t)
	swapRepoContractTest(t, greenPack())

	if err := l.ship(t); err != nil {
		t.Fatalf("at shadow the lane lands, got %v", err)
	}

	if l.mainTip(t) == l.base {
		t.Error("the lane did not land at shadow")
	}
	got := l.emitted(selfheal.CodeMainRedStopWouldRefuse)
	if len(got) != 1 || !strings.Contains(got[0].Fields["keys"], failedCIJob) {
		t.Errorf("would-refuse signals = %+v, want one that names the failed job %s", got, failedCIJob)
	}
}

func TestShip_LandingsResumeWhenAGreenMainClosesTheCIStop(t *testing.T) {
	l := newBaseRedLane(t)
	l.landable(t)
	l.enforceStop(t)
	l.holdCIStop(t)
	l.releaseCIStop(t)
	swapRepoContractTest(t, greenPack())

	if err := l.ship(t); err != nil {
		t.Fatalf("a lane lands once the CI stop is closed, got %v", err)
	}

	if l.mainTip(t) == l.base {
		t.Error("the lane did not land after the stop closed")
	}
	if l.stop(t).Holds() {
		t.Error("the closed stop holds again")
	}
}
