package ship

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/selfheal"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
)

func (l *baseRedLane) laneCarries(t *testing.T, name, body string) {
	t.Helper()
	ws := filepath.Join(l.repo, ".evolve", "runs", "cycle-1")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(ws, name), body)
}

func (l *baseRedLane) laneScopeNames(t *testing.T, ids ...string) {
	t.Helper()
	quoted := ""
	for i, id := range ids {
		if i > 0 {
			quoted += ","
		}
		quoted += `"` + id + `"`
	}
	l.laneCarries(t, "lane-scope.json", `{"todo_ids":[`+quoted+`]}`)
}

func (l *baseRedLane) triageCommits(t *testing.T, id string) {
	t.Helper()
	l.laneCarries(t, "triage-decision.json", `{"schema_version":1,"top_n":[{"id":"`+id+`"}]}`)
}

func TestShip_AdmitsTheLaneThatCarriesTheCIStopFixItemAtEnforce(t *testing.T) {
	l := newBaseRedLane(t)
	l.landable(t)
	l.enforceStop(t)
	held := l.holdCIStop(t)
	l.laneScopeNames(t, held.FixItem)
	swapRepoContractTest(t, greenPack())

	if err := l.ship(t); err != nil {
		t.Fatalf("the lane that carries the fix item %s of the CI stop lands at enforce, got %v", held.FixItem, err)
	}

	if l.mainTip(t) == l.base {
		t.Error("the CI fix lane did not land on main")
	}
	if after := l.stop(t); !after.Holds() || !slices.Equal(after.Keys, held.Keys) {
		t.Errorf("stop = %+v, want the CI keys kept: only the poll of a green main closes a CI stop", after)
	}
	if n := len(l.emitted(selfheal.CodeMainRedStopReleased)); n != 0 {
		t.Errorf("the landing emitted %d release signals; a lane cannot prove a CI job green", n)
	}
}

func TestShip_AdmitsTheCIFixLaneThatTheTriageDecisionCommits(t *testing.T) {
	l := newBaseRedLane(t)
	l.landable(t)
	l.enforceStop(t)
	held := l.holdCIStop(t)
	l.triageCommits(t, held.FixItem)
	swapRepoContractTest(t, greenPack())

	if err := l.ship(t); err != nil {
		t.Fatalf("a lane whose triage decision commits the fix item %s lands at enforce, got %v", held.FixItem, err)
	}

	if l.mainTip(t) == l.base {
		t.Error("the CI fix lane did not land on main")
	}
}

func TestShip_KeepsTheCIFixItemPendingWhileTheCIStopHoldsAfterTheFixLaneLands(t *testing.T) {
	l := newBaseRedLane(t)
	l.landable(t)
	l.enforceStop(t)
	held := l.holdCIStop(t)
	l.laneScopeNames(t, held.FixItem)
	swapRepoContractTest(t, greenPack())

	if err := l.ship(t); err != nil {
		t.Fatalf("the CI fix lane must land, got %v", err)
	}

	if !l.fixItemPending(t, held.FixItem) {
		t.Errorf("the fix item %s was consumed by the landing while the CI stop still holds; a CI that stays red then has no item to retry", held.FixItem)
	}
}

func TestShip_RefusesAnotherLaneWhileTheCIStopHoldsEvenWhenItHasAScope(t *testing.T) {
	l := newBaseRedLane(t)
	l.landable(t)
	l.enforceStop(t)
	held := l.holdCIStop(t)
	l.laneScopeNames(t, "some-other-inbox-item")
	swapRepoContractTest(t, greenPack())

	err := l.ship(t)

	wantShipErr(t, err, shiperr.CodeMainCIRed, core.ShipClassPrecondition, failedCIJob)
	if l.mainTip(t) != l.base || !l.stop(t).Holds() {
		t.Errorf("main = %s (base %s), stop holds = %v, want nothing landed and the stop held", l.mainTip(t), l.base, l.stop(t).Holds())
	}
	if !l.fixItemPending(t, held.FixItem) {
		t.Errorf("the fix item %s is not pending after the refusal", held.FixItem)
	}
}

func TestShip_ShadowRecordsNoRefusalForTheCIFixLane(t *testing.T) {
	l := newBaseRedLane(t)
	l.landable(t)
	held := l.holdCIStop(t)
	l.laneScopeNames(t, held.FixItem)
	swapRepoContractTest(t, greenPack())

	if err := l.ship(t); err != nil {
		t.Fatalf("at shadow the lane lands, got %v", err)
	}

	if got := l.emitted(selfheal.CodeMainRedStopWouldRefuse); len(got) != 0 {
		t.Errorf("would-refuse signals = %+v, want none: enforce admits the CI fix lane, so shadow must not predict a refusal", got)
	}
}

func TestShip_CarryingTheFixItemOfALaneKeyStopStillNeedsTheKeyProof(t *testing.T) {
	l := newBaseRedLane(t)
	l.landable(t)
	l.enforceStop(t)
	held := l.holdStop(t, redKey)
	l.laneScopeNames(t, held.FixItem)
	swapRepoContractTest(t, greenPack())

	err := l.ship(t)

	wantShipErr(t, err, shiperr.CodeMainRedPark, core.ShipClassPrecondition, redPkg+"."+redTest)
	if l.mainTip(t) != l.base || !l.stop(t).Holds() {
		t.Errorf("main = %s (base %s), stop holds = %v, want a lane that turns no key green to wait whatever it carries", l.mainTip(t), l.base, l.stop(t).Holds())
	}
}

func TestShip_CarryingTheFixItemOfAMixedStopStillNeedsTheKeyProof(t *testing.T) {
	l := newBaseRedLane(t)
	l.landable(t)
	l.enforceStop(t)
	held := l.holdStop(t, redKey)
	if _, err := core.OpenMainCIRedStop(l.repo, core.MainCIRed{Main: l.base, Failed: []string{failedCIJob}}); err != nil {
		t.Fatalf("add a CI key to the held stop: %v", err)
	}
	if mixed := l.stop(t); core.StopCIOnly(mixed) || len(core.StopCIKeys(mixed)) == 0 || len(core.StopLaneKeys(mixed)) == 0 {
		t.Fatalf("precondition: the stop %+v must hold one lane key and one CI key", mixed)
	}
	l.laneScopeNames(t, held.FixItem)
	swapRepoContractTest(t, greenPack())

	err := l.ship(t)

	wantShipErr(t, err, shiperr.CodeMainRedPark, core.ShipClassPrecondition, redPkg+"."+redTest)
	if l.mainTip(t) != l.base {
		t.Error("a lane that turns no lane key green landed on a mixed stop")
	}
}

func TestShipStop_TheCIFixLaneIsAdmittedByTheStopCheckWithNoKeyRun(t *testing.T) {
	t.Parallel()
	l := newBaseRedLane(t)
	l.enforceStop(t)
	held := l.holdCIStop(t)
	l.laneScopeNames(t, held.FixItem)
	run := &scriptedBase{answers: [][]baseattr.Key{nil}}
	opts := l.options(run.run)
	opts.WorkspacePath = filepath.Join(l.repo, ".evolve", "runs", "cycle-1")

	got := stopCheckOf(t, opts)

	if got.err != nil {
		t.Errorf("the stop check refused the CI fix lane: %v", got.err)
	}
	if run.calls != 0 {
		t.Errorf("the proof ran %d times; a CI key names a job, so no lane can run it", run.calls)
	}
}

func TestHeldCIFixItem_NamesTheFixItemOfAHeldCIStopWithANilWriter(t *testing.T) {
	t.Parallel()
	l := newBaseRedLane(t)
	held := l.holdCIStop(t)
	opts := &Options{ProjectRoot: l.repo}

	if got := heldCIFixItem(opts, nil); got != held.FixItem {
		t.Errorf("heldCIFixItem = %q, want %q", got, held.FixItem)
	}
	if got := heldCIFixItem(nil, nil); got != "" {
		t.Errorf("heldCIFixItem(nil) = %q, want empty", got)
	}
	if got := heldCIFixItem(&Options{}, nil); got != "" {
		t.Errorf("heldCIFixItem with no project root = %q, want empty", got)
	}
}

func TestHeldCIFixItem_NamesNothingWhenNoStopHolds(t *testing.T) {
	t.Parallel()
	l := newBaseRedLane(t)

	if got := heldCIFixItem(&Options{ProjectRoot: l.repo}, nil); got != "" {
		t.Errorf("heldCIFixItem with no stop = %q, want empty", got)
	}
}
