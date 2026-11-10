package ship

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/covchanged"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxfile"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover/lifecycle"
	"github.com/mickeyyaya/evolve-loop/go/internal/mainstop"
	"github.com/mickeyyaya/evolve-loop/go/internal/selfheal"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const enforceStopPolicy = `{"gates":{"main_red_stop":"enforce"}}`

var otherKey = baseattr.Key{Package: redPkg, Test: "TestOther"}

func (l *baseRedLane) holdStop(t *testing.T, keys ...baseattr.Key) mainstop.Stop {
	t.Helper()
	if _, err := core.OpenMainRedStop(l.repo, baseattr.Verdict{BaseRed: keys, MainRed: keys, Main: l.base}); err != nil {
		t.Fatalf("open the stop: %v", err)
	}
	return l.stop(t)
}

func (l *baseRedLane) stop(t *testing.T) mainstop.Stop {
	t.Helper()
	stop, found, err := mainstop.Read(filepath.Join(l.repo, ".evolve"))
	if err != nil || !found {
		t.Fatalf("read the stop: found=%v err=%v", found, err)
	}
	return stop
}

func (l *baseRedLane) enforceStop(t *testing.T) {
	t.Helper()
	mustWrite(t, filepath.Join(l.repo, ".evolve", "policy.json"), enforceStopPolicy)
}

func (l *baseRedLane) fixInLane(t *testing.T) {
	t.Helper()
	mustWrite(t, filepath.Join(l.wt, filepath.FromSlash(redTestFile)), greenBody)
	runGit(t, l.wt, "add", "-A")
}

func (l *baseRedLane) landable(t *testing.T) {
	t.Helper()
	addRemote(t, l.repo)
	seedAuditWithBoundTree(t, l.repo, "PASS", strings.TrimSpace(runGitOut(t, l.wt, "write-tree")))
}

func (l *baseRedLane) mainTip(t *testing.T) string {
	t.Helper()
	return strings.TrimSpace(runGitOut(t, l.repo, "rev-parse", "main"))
}

func (l *baseRedLane) fixItemPending(t *testing.T, id string) bool {
	t.Helper()
	_, err := inboxmover.Locate(filepath.Join(l.repo, ".evolve", "inbox"), id)
	if err != nil && !errors.Is(err, inboxmover.ErrNotFound) {
		t.Fatalf("locate %s: %v", id, err)
	}
	return err == nil
}

func (l *baseRedLane) fixItemFiled(t *testing.T, id string) bool {
	t.Helper()
	filed, err := inboxfile.FiledIDs(filepath.Join(l.repo, ".evolve", "inbox"))
	if err != nil {
		t.Fatalf("filed ids: %v", err)
	}
	return filed[id]
}

func stopChecked(t *testing.T, opts *Options) (error, string) {
	t.Helper()
	c := stopCheckOf(t, opts)
	return c.err, c.log
}

type checkedStop struct {
	green []baseattr.Key
	err   error
	log   string
}

func stopCheckOf(t *testing.T, opts *Options) checkedStop {
	t.Helper()
	var log strings.Builder
	green, err := checkMainRedStop(context.Background(), opts, &log)
	return checkedStop{green: green, err: err, log: log.String()}
}

func TestShipStop_AFixLandsAndReleasesTheStop(t *testing.T) {
	l := newBaseRedLane(t)
	l.fixInLane(t)
	l.landable(t)
	l.enforceStop(t)
	held := l.holdStop(t, redKey)
	if !l.fixItemPending(t, held.FixItem) {
		t.Fatalf("precondition: the fix item %s is pending", held.FixItem)
	}
	swapRepoContractTest(t, greenPack())

	if err := l.ship(t); err != nil {
		t.Fatalf("a lane that turns the key of the stop green lands, got %v", err)
	}

	landed := l.mainTip(t)
	if landed == l.base {
		t.Fatal("the fix did not land on main")
	}
	stop := l.stop(t)
	if stop.Holds() || stop.ReleasedAt == nil || stop.ReleasedBy != "cycle 1 "+landed || len(stop.Keys) != 0 {
		t.Fatalf("stop = %+v, want a release by cycle 1 at %s with no key left", stop, landed)
	}
	if time.Since(*stop.ReleasedAt) > time.Hour || stop.ReleasedAt.Nanosecond() != 0 {
		t.Errorf("released_at = %v, want the ship clock at whole seconds", stop.ReleasedAt)
	}
	got := l.emitted(core.CodeMainRedStopReleased)
	if len(got) != 1 || got[0].Cycle != 1 || got[0].RunID != "test-run" || got[0].Phase != phaseName || got[0].Module != signalcenter.ModuleGateRepo ||
		got[0].Kind != signalcenter.KindGateCorrected || got[0].Severity != signalcenter.SeverityInfo ||
		got[0].Fields["main"] != l.base || got[0].Fields["fix_item"] != held.FixItem || got[0].Fields["released_by"] != "cycle 1 "+landed || got[0].Fields["fix_item_error"] != "" {
		t.Errorf("signals = %+v, want one info GATE_REPO_MAIN_RED_STOP_RELEASED with main, fix_item and released_by", got)
	}
	if l.fixItemPending(t, held.FixItem) || !l.fixItemFiled(t, held.FixItem) {
		t.Errorf("the fix item %s is pending=%v filed=%v, want it consumed", held.FixItem, l.fixItemPending(t, held.FixItem), l.fixItemFiled(t, held.FixItem))
	}
	if _, err := os.Stat(filepath.Join(l.repo, ".evolve", "inbox", "processed", "cycle-1")); err != nil {
		t.Errorf("the fix item is not in processed/cycle-1: %v", err)
	}
	if refused := l.emitted(core.CodeMainRedStopWouldRefuse); len(refused) != 0 {
		t.Errorf("a fix got %d would-refuse signals", len(refused))
	}
}

func TestShipStop_ANonFixParksAtEnforceAndLandsAtShadow(t *testing.T) {
	t.Run("enforce parks the lane", func(t *testing.T) {
		l := newBaseRedLane(t)
		l.landable(t)
		l.enforceStop(t)
		held := l.holdStop(t, redKey)
		swapRepoContractTest(t, greenPack())

		err := l.ship(t)

		se := wantShipErr(t, err, shiperr.CodeMainRedPark, core.ShipClassPrecondition, redPkg+"."+redTest)
		if se.Debug["keys"] != redPkg+"."+redTest || se.Debug["main"] != held.MainSHA || se.Debug["base"] != l.base {
			t.Errorf("debug %v: the park carries the keys of the stop, the lane base and the main of the stop", se.Debug)
		}
		if v := baseattr.VerdictOfDebug(se.Debug); len(v.MainRed) != 1 || v.MainRed[0] != redKey {
			t.Errorf("verdict of the park = %+v, want the key red on main, so the park opens no empty stop", v)
		}
		if !strings.Contains(err.Error(), "ship main-red stop") {
			t.Errorf("err = %v, want the stop named as the refuser", err)
		}
		if l.mainTip(t) != l.base || !l.stop(t).Holds() {
			t.Errorf("main = %s (base %s), stop holds = %v; a parked lane lands nothing and releases nothing", l.mainTip(t), l.base, l.stop(t).Holds())
		}
		if n := len(l.emitted(core.CodeMainRedStopWouldRefuse)) + len(l.emitted(core.CodeMainRedStopReleased)); n != 0 {
			t.Errorf("an enforced refusal emitted %d shadow or release signals", n)
		}
	})
	t.Run("shadow lands the lane and records the refusal", func(t *testing.T) {
		l := newBaseRedLane(t)
		l.landable(t)
		held := l.holdStop(t, redKey)
		swapRepoContractTest(t, greenPack())

		if err := l.ship(t); err != nil {
			t.Fatalf("at shadow the lane lands, got %v", err)
		}

		if l.mainTip(t) == l.base {
			t.Error("the lane did not land at shadow")
		}
		got := l.emitted(core.CodeMainRedStopWouldRefuse)
		if len(got) != 1 || got[0].Cycle != 1 || got[0].RunID != "test-run" || got[0].Phase != phaseName || got[0].Module != signalcenter.ModuleGateRepo ||
			got[0].Kind != signalcenter.KindShipWarning || got[0].Severity != signalcenter.SeverityWarn ||
			got[0].Fields["admission"] != "not-a-fix" || got[0].Fields["keys"] != redPkg+"."+redTest || got[0].Fields["base"] != l.base || got[0].Fields["main"] != held.MainSHA || got[0].Fields["fix_item"] != held.FixItem ||
			!strings.Contains(got[0].Reason, "not-a-fix") {
			t.Errorf("signals = %+v, want one warn GATE_REPO_MAIN_RED_STOP_WOULD_REFUSE with the admission not-a-fix", got)
		}
		if !l.stop(t).Holds() || len(l.emitted(core.CodeMainRedStopReleased)) != 0 || !l.fixItemPending(t, held.FixItem) {
			t.Error("a lane that is not a fix released the stop or consumed the fix item")
		}
	})
}

func TestShipStop_ALaneThatIsNotOnTheMainTipIsAskedToRebase(t *testing.T) {
	t.Parallel()
	l := newBaseRedLane(t)
	l.holdStop(t, redKey)
	l.landOnMain(t, "main moves on", map[string]string{"docs/note.md": "a note\n"})
	run := &scriptedBase{answers: [][]baseattr.Key{nil}}

	shadowErr, _ := stopChecked(t, l.options(run.run))
	l.enforceStop(t)
	err, _ := stopChecked(t, l.options(run.run))

	se := wantShipErr(t, err, shiperr.CodeGitFleetRebaseNeeded, core.ShipClassTransient, redPkg+"."+redTest)
	if se.Debug["base"] != l.base || se.Debug["main"] != l.mainTip(t) || se.Debug["keys"] != redPkg+"."+redTest {
		t.Errorf("debug %v: the rebase carries the lane base, the main tip and the keys", se.Debug)
	}
	if v := baseattr.VerdictOfDebug(se.Debug); len(v.MainRed) != 0 {
		t.Errorf("verdict of the rebase = %+v, want no key judged red on main", v)
	}
	if run.calls != 0 {
		t.Errorf("the proof ran %d times for a lane that is not on the main tip, want 0", run.calls)
	}
	got := l.emitted(core.CodeMainRedStopWouldRefuse)
	if shadowErr != nil || len(got) != 1 || got[0].Fields["admission"] != "rebase" || got[0].Fields["main"] != l.mainTip(t) || got[0].Fields["base"] != l.base {
		t.Errorf("shadow err = %v, signals = %+v, want one would-refuse with the admission rebase", shadowErr, got)
	}
}

func TestShipStop_TheProofRunsTheKeysOfTheStopInTheLaneWithTheFloorTags(t *testing.T) {
	t.Parallel()
	l := newBaseRedLane(t)
	l.enforceStop(t)
	l.holdStop(t, redKey, otherKey)
	var dirs []string
	var asked []baseattr.Reds
	opts := l.options(func(_ context.Context, dir string, r baseattr.Reds) ([]baseattr.Key, error) {
		dirs, asked = append(dirs, dir), append(asked, r)
		return []baseattr.Key{otherKey}, nil
	})

	checked := stopCheckOf(t, opts)

	if checked.err != nil || checked.log != "" {
		t.Fatalf("a lane that turns one key green is a fix, got err=%v log=%q", checked.err, checked.log)
	}
	if len(dirs) != 1 || dirs[0] != filepath.Join(l.wt, "go") {
		t.Errorf("the proof ran in %v, want once in the module of the lane tree", dirs)
	}
	if !slices.Equal(asked[0].Keys, []baseattr.Key{redKey, otherKey}) || !slices.Equal(asked[0].Flags, covchanged.TagFlags(covchanged.DefaultTags)) || len(asked[0].Flags) == 0 {
		t.Errorf("the proof asked %+v, want the two keys of the stop with the tags of the floor", asked[0])
	}
	if !slices.Equal(checked.green, []baseattr.Key{redKey}) {
		t.Errorf("green keys = %v, want only %v", checked.green, redKey)
	}

	res := stopRelease{opts: opts, warn: opts.Stderr}.landedFix(checked.green, RunResult{CommitSHA: "abc1234"}, nil)

	stop := l.stop(t)
	if len(res.Logs) != 0 || !stop.Holds() || !slices.Equal(stop.Keys, []baseattr.Key{otherKey}) || len(l.emitted(core.CodeMainRedStopReleased)) != 0 {
		t.Errorf("logs = %v, stop = %+v; a fix of one key keeps the stop on the other key and signals no release", res.Logs, stop)
	}
}

func TestShipStop_AProofThatCannotRunIsNotAFix(t *testing.T) {
	t.Parallel()
	l := newBaseRedLane(t)
	l.enforceStop(t)
	l.holdStop(t, redKey)
	opts := l.options((&scriptedBase{err: errors.New("go test did not start")}).run)

	checked := stopCheckOf(t, opts)
	log := checked.log

	wantShipErr(t, checked.err, shiperr.CodeMainRedPark, core.ShipClassPrecondition, "")
	if !strings.Contains(log, "could not be run in the lane") || !strings.Contains(log, "go test did not start") || len(checked.green) != 0 {
		t.Errorf("log = %q, green = %v, want a warning with the cause and no green key", log, checked.green)
	}
}

func TestShipStop_AMainThatCannotBeResolvedAsksForTheRebase(t *testing.T) {
	t.Parallel()
	l := newBaseRedLane(t)
	l.enforceStop(t)
	l.holdStop(t, redKey)
	runGit(t, l.repo, "branch", "-m", "main", "trunk")
	run := &scriptedBase{answers: [][]baseattr.Key{nil}}

	err, log := stopChecked(t, l.options(run.run))

	se := wantShipErr(t, err, shiperr.CodeGitFleetRebaseNeeded, core.ShipClassTransient, "")
	if se.Debug["main"] != "" || run.calls != 0 || !strings.Contains(log, "main could not be resolved") {
		t.Errorf("debug = %v, proof runs = %d, log = %q, want no main, no proof run and a warning", se.Debug, run.calls, log)
	}
}

func TestShipStop_NoStopOrAnUnreadableStopHoldsNoLane(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		prepare func(t *testing.T, l *baseRedLane)
		warning string
	}{
		"no record": {func(*testing.T, *baseRedLane) {}, ""},
		"a released stop": {func(t *testing.T, l *baseRedLane) {
			l.holdStop(t, redKey)
			if err := (stopRelease{opts: l.options(nil), warn: os.Stderr}).release([]baseattr.Key{redKey}, "main abc"); err != nil {
				t.Fatal(err)
			}
		}, ""},
		"a record that does not parse": {func(t *testing.T, l *baseRedLane) {
			mustWrite(t, mainstop.Path(filepath.Join(l.repo, ".evolve")), "{not json")
		}, "the record could not be read"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			l := newBaseRedLane(t)
			l.enforceStop(t)
			c.prepare(t, l)
			run := &scriptedBase{answers: [][]baseattr.Key{nil}}

			err, log := stopChecked(t, l.options(run.run))

			if err != nil || run.calls != 0 {
				t.Errorf("err = %v, proof runs = %d, want the lane free and no proof run", err, run.calls)
			}
			if (c.warning == "") != (log == "") || !strings.Contains(log, c.warning) {
				t.Errorf("log = %q, want the warning %q", log, c.warning)
			}
		})
	}
}

func TestShipStop_ALandingTreeThatCannotBeResolvedIsAnError(t *testing.T) {
	t.Parallel()
	l := newBaseRedLane(t)
	l.holdStop(t, redKey)
	opts := l.options(nil)
	opts.ActiveWorktree = filepath.Join(t.TempDir(), "gone")

	err, _ := stopChecked(t, opts)

	wantShipErr(t, err, core.CodeWorktreeResolve, core.ShipClassPrecondition, "")
}

func TestReleaseLandedFix_OnlyALandedFixReleasesTheStop(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		res   RunResult
		err   error
		green []baseattr.Key
	}{
		"ship failed":          {RunResult{CommitSHA: "abc1234"}, errors.New("push refused"), []baseattr.Key{redKey}},
		"ship exited non-zero": {RunResult{CommitSHA: "abc1234", ExitCode: ExitOK + 1}, nil, []baseattr.Key{redKey}},
		"nothing landed":       {RunResult{}, nil, []baseattr.Key{redKey}},
		"the lane is no fix":   {RunResult{CommitSHA: "abc1234"}, nil, nil},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			l := newBaseRedLane(t)
			l.holdStop(t, redKey)
			opts := l.options(nil)

			res := stopRelease{opts: opts, warn: opts.Stderr}.landedFix(c.green, c.res, c.err)

			if !l.stop(t).Holds() || len(res.Logs) != 0 {
				t.Errorf("stop holds = %v, logs = %v; want the stop held and no log", l.stop(t).Holds(), res.Logs)
			}
		})
	}
}

func TestReleaseLandedFix_AReleaseThatFailsIsAWarningOfTheLandedShip(t *testing.T) {
	t.Parallel()
	l := newBaseRedLane(t)
	l.holdStop(t, redKey)
	opts := l.options(nil)
	mustWrite(t, mainstop.Path(filepath.Join(l.repo, ".evolve")), "{not json")
	prior := []string{"[ship] landed"}

	res := stopRelease{opts: opts, warn: opts.Stderr}.landedFix([]baseattr.Key{redKey}, RunResult{CommitSHA: "abc1234", Logs: prior}, nil)

	if len(res.Logs) != 2 || !strings.Contains(res.Logs[1], "WARN: main-red stop release") || !strings.Contains(res.Logs[1], "mainstop: parse") || len(prior) != 1 {
		t.Errorf("logs = %v, want the landing line and one release warning with the cause", res.Logs)
	}
}

func TestReleaseMainRedStop_AStopThatDoesNotHoldIsNotWrittenOrSignalled(t *testing.T) {
	t.Parallel()
	l := newBaseRedLane(t)
	opts := l.options(nil)
	fixed := time.Date(2026, 11, 10, 8, 0, 0, 0, time.UTC)
	opts.NowFn = func() Now { return Now{Unix: fixed.Unix()} }

	if err := (stopRelease{opts: opts, warn: os.Stderr}).release([]baseattr.Key{redKey}, "main abc"); err != nil {
		t.Fatalf("no stop: %v", err)
	}
	if _, found, _ := mainstop.Read(filepath.Join(l.repo, ".evolve")); found {
		t.Error("a release with no stop wrote a record")
	}

	l.holdStop(t, redKey)
	if err := (stopRelease{opts: opts, warn: os.Stderr}).release(nil, "main abc"); err != nil || !l.stop(t).Holds() {
		t.Errorf("a release with no green key: err = %v, holds = %v", err, l.stop(t).Holds())
	}
	if err := (stopRelease{opts: opts, warn: os.Stderr}).release([]baseattr.Key{redKey}, "main abc"); err != nil {
		t.Fatal(err)
	}
	first := l.stop(t)
	if first.ReleasedAt == nil || !first.ReleasedAt.Equal(fixed) || first.ReleasedBy != "main abc" {
		t.Fatalf("stop = %+v, want a release at the ship clock by main abc", first)
	}
	if err := (stopRelease{opts: opts, warn: os.Stderr}).release([]baseattr.Key{redKey}, "main def"); err != nil {
		t.Fatal(err)
	}
	if again := l.stop(t); again.ReleasedBy != "main abc" || len(l.emitted(core.CodeMainRedStopReleased)) != 1 {
		t.Errorf("a second release changed the record to %+v or signalled again (%d signals)", again, len(l.emitted(core.CodeMainRedStopReleased)))
	}
}

func TestReleaseMainRedStop_AFixItemThatCannotMoveMakesTheReleaseAWarning(t *testing.T) {
	t.Parallel()
	type brokenInbox struct {
		breakInbox  func(t *testing.T, inbox string)
		moveSignals int
	}
	cases := map[string]brokenInbox{
		"the inbox cannot be read": {breakInbox: func(t *testing.T, inbox string) {
			if err := os.Chmod(inbox, 0); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(inbox, 0o755) })
		}},
		"the processed directory is a file": {moveSignals: 1, breakInbox: func(t *testing.T, inbox string) {
			mustWrite(t, filepath.Join(inbox, "processed"), "")
		}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			l := newBaseRedLane(t)
			held := l.holdStop(t, redKey)
			c.breakInbox(t, filepath.Join(l.repo, ".evolve", "inbox"))
			var log strings.Builder

			err := (stopRelease{opts: l.options(nil), warn: &log}).release([]baseattr.Key{redKey}, "main abc")

			if err == nil || !strings.Contains(err.Error(), "fix item "+held.FixItem) || l.stop(t).Holds() {
				t.Fatalf("err = %v, holds = %v, want the stop released and an error that names the fix item", err, l.stop(t).Holds())
			}
			got := l.emitted(core.CodeMainRedStopReleased)
			if len(got) != 1 || got[0].Severity != signalcenter.SeverityWarn || got[0].Fields["fix_item_error"] != err.Error() {
				t.Errorf("signals = %+v, want one warn release with fix_item_error", got)
			}
			if moved := len(l.emitted(lifecycle.CodePromoteMoveFailed)); moved != c.moveSignals {
				t.Errorf("the mover emitted %d move-failed signals through the center of ship, want %d", moved, c.moveSignals)
			}
		})
	}
}

func TestReleaseMainRedStop_AFixItemThatIsConsumedAlreadyStaysWhereItIs(t *testing.T) {
	t.Parallel()
	l := newBaseRedLane(t)
	held := l.holdStop(t, redKey)
	held = held.WithParked(mainstop.Parked{Cycle: 7}).WithParked(mainstop.Parked{Cycle: 9})
	if _, err := mainstop.Update(filepath.Join(l.repo, ".evolve"), func(mainstop.Stop, bool) (mainstop.Stop, error) { return held, nil }); err != nil {
		t.Fatal(err)
	}
	inbox := filepath.Join(l.repo, ".evolve", "inbox")
	items, err := filepath.Glob(filepath.Join(inbox, "*.json"))
	if err != nil || len(items) != 1 {
		t.Fatalf("precondition: one inbox item, got %v, %v", items, err)
	}
	item := filepath.Base(items[0])
	consumed := filepath.Join(inbox, "consumed", "cycle-3")
	mustMkdir(t, consumed)
	if err := os.Rename(items[0], filepath.Join(consumed, item)); err != nil {
		t.Fatal(err)
	}

	if err := (stopRelease{opts: l.options(nil), warn: os.Stderr}).release([]baseattr.Key{redKey}, "main abc"); err != nil {
		t.Fatalf("release: %v", err)
	}

	if _, err := os.Stat(filepath.Join(consumed, item)); err != nil {
		t.Errorf("the consumed fix item moved: %v", err)
	}
	got := l.emitted(core.CodeMainRedStopReleased)
	if len(got) != 1 || got[0].Severity != signalcenter.SeverityInfo || got[0].Fields["parked"] != "7, 9" || !strings.Contains(got[0].Reason, "main abc") {
		t.Errorf("signals = %+v, want one info release that names the parked cycles 7, 9", got)
	}
}

func TestAttributeContractRed_KeysThatAreGreenOnMainReleaseTheStop(t *testing.T) {
	t.Parallel()
	l := newBaseRedLane(t)
	held := l.holdStop(t, redKey)
	l.landOnMain(t, "main fixes the test", map[string]string{redTestFile: greenBody})
	run := &scriptedBase{answers: [][]baseattr.Key{{redKey}, nil}}

	err, log := attributed(t, l.options(run.run), redPack(redPkg+"."+redTest))

	wantShipErr(t, err, shiperr.CodeGitFleetRebaseNeeded, core.ShipClassTransient, "")
	stop := l.stop(t)
	if stop.Holds() || stop.ReleasedBy != "main "+l.mainTip(t) || strings.Contains(log, "WARN") {
		t.Errorf("stop = %+v, log = %q, want a release by the main tip %s", stop, log, l.mainTip(t))
	}
	if l.fixItemPending(t, held.FixItem) || len(l.emitted(core.CodeMainRedStopReleased)) != 1 {
		t.Errorf("fix item pending = %v, release signals = %d, want the item consumed and one signal", l.fixItemPending(t, held.FixItem), len(l.emitted(core.CodeMainRedStopReleased)))
	}
}

func TestAttributeContractRed_AMainThatIsNotJudgedReleasesNoStop(t *testing.T) {
	t.Parallel()
	l := newBaseRedLane(t)
	l.holdStop(t, redKey)
	runGit(t, l.repo, "branch", "-m", "main", "trunk")
	run := &scriptedBase{answers: [][]baseattr.Key{{redKey}}}

	err, _ := attributed(t, l.options(run.run), redPack(redPkg+"."+redTest))

	wantShipErr(t, err, shiperr.CodeMainRedPark, core.ShipClassPrecondition, "")
	if !l.stop(t).Holds() {
		t.Error("a red that main did not judge released the stop")
	}
}

func TestAttributeContractRed_AReleaseThatFailsIsAWarning(t *testing.T) {
	t.Parallel()
	l := newBaseRedLane(t)
	l.landOnMain(t, "main fixes the test", map[string]string{redTestFile: greenBody})
	mustWrite(t, mainstop.Path(filepath.Join(l.repo, ".evolve")), "{not json")
	run := &scriptedBase{answers: [][]baseattr.Key{{redKey}, nil}}

	err, log := attributed(t, l.options(run.run), redPack(redPkg+"."+redTest))

	wantShipErr(t, err, shiperr.CodeGitFleetRebaseNeeded, core.ShipClassTransient, "")
	if !strings.Contains(log, "WARN main-red stop release") || !strings.Contains(log, "mainstop: parse") {
		t.Errorf("log = %q, want the release warning with its cause", log)
	}
}

func TestOptionsClock_UsesTheInjectedClockOrTheDefault(t *testing.T) {
	t.Parallel()
	fixed := time.Date(2026, 11, 10, 8, 0, 0, 0, time.UTC)
	if got := (&Options{NowFn: func() Now { return Now{Unix: fixed.Unix()} }}).clock(); !got.Equal(fixed) || got.Location() != time.UTC {
		t.Errorf("clock = %v, want %v in UTC", got, fixed)
	}
	if got := (&Options{}).clock(); time.Since(got) > time.Minute || time.Since(got) < 0 {
		t.Errorf("default clock = %v, want the current time", got)
	}
}

func TestShipStop_ABaseAndAMainThatCannotBeResolvedRunNoProof(t *testing.T) {
	t.Parallel()
	l := newBaseRedLane(t)
	l.enforceStop(t)
	l.holdStop(t, redKey)
	runGit(t, l.repo, "branch", "-m", "main", "trunk")
	run := &scriptedBase{answers: [][]baseattr.Key{nil}}
	opts := l.options(run.run)
	opts.WorktreeBaseSHA = "0000000000000000000000000000000000000001"

	checked := stopCheckOf(t, opts)

	wantShipErr(t, checked.err, shiperr.CodeGitFleetRebaseNeeded, core.ShipClassTransient, "")
	if run.calls != 0 || len(checked.green) != 0 {
		t.Errorf("proof runs = %d, green = %v; a lane with no known base proves no fix", run.calls, checked.green)
	}
}

func TestShipStop_AParkOnANewerMainKeepsTheMainOfTheStop(t *testing.T) {
	t.Parallel()
	first := newBaseRedLane(t)
	first.enforceStop(t)
	held := first.holdStop(t, redKey)
	first.landOnMain(t, "main moves on", map[string]string{"docs/note.md": "a note\n"})
	l := first.laneOn(t, "cycle-2-pack")
	run := &scriptedBase{answers: [][]baseattr.Key{{redKey}}}

	err, _ := stopChecked(t, l.options(run.run))

	se := wantShipErr(t, err, shiperr.CodeMainRedPark, core.ShipClassPrecondition, "")
	if l.base == held.MainSHA || se.Debug["main"] != held.MainSHA || se.Debug["base"] != l.base || run.calls != 1 {
		t.Errorf("debug = %v, proof runs = %d, want the main %s of the stop and the lane base %s", se.Debug, run.calls, held.MainSHA, l.base)
	}
}

func TestAttributeContractRed_KeysThatAreRedOnMainKeepTheStop(t *testing.T) {
	t.Parallel()
	l := newBaseRedLane(t)
	l.holdStop(t, redKey)
	l.landOnMain(t, "main moves on", map[string]string{"docs/note.md": "a note\n"})
	run := &scriptedBase{answers: [][]baseattr.Key{{redKey}, {redKey}}}

	err, _ := attributed(t, l.options(run.run), redPack(redPkg+"."+redTest))

	wantShipErr(t, err, shiperr.CodeMainRedPark, core.ShipClassPrecondition, "")
	if !l.stop(t).Holds() || len(l.emitted(core.CodeMainRedStopReleased)) != 0 {
		t.Error("a key that is red on main released the stop")
	}
}

func TestReleaseMainRedStop_NoGreenKeyDoesNotOpenTheRecord(t *testing.T) {
	t.Parallel()
	l := newBaseRedLane(t)
	mustWrite(t, mainstop.Path(filepath.Join(l.repo, ".evolve")), "{not json")

	if err := (stopRelease{opts: l.options(nil), warn: os.Stderr}).release(nil, "main abc"); err != nil {
		t.Errorf("a release of no key read the record: %v", err)
	}
}

func TestShipStop_TheRepoContractGateRefusesBeforeTheStopCheck(t *testing.T) {
	l := newBaseRedLane(t)
	l.landable(t)
	l.enforceStop(t)
	l.holdStop(t, redKey)
	swapRepoContractTest(t, redPack(redPkg+"."+redTest))

	err := l.ship(t)

	if err == nil || !strings.Contains(err.Error(), "ship repo-contract gate") || strings.Contains(err.Error(), "ship main-red stop") {
		t.Errorf("err = %v, want the repo-contract gate as the refuser, because it runs before the stop check", err)
	}
}

func TestShipStop_AShortLaneBaseIsResolvedBeforeTheCompareWithMain(t *testing.T) {
	t.Parallel()
	l := newBaseRedLane(t)
	l.enforceStop(t)
	l.holdStop(t, redKey)
	run := &scriptedBase{answers: [][]baseattr.Key{nil}}
	opts := l.options(run.run)
	opts.WorktreeBaseSHA = l.base[:12]

	checked := stopCheckOf(t, opts)

	if checked.err != nil || run.calls != 1 || !slices.Equal(checked.green, []baseattr.Key{redKey}) {
		t.Errorf("err = %v, proof runs = %d, green = %v; want a short base that names the main tip to prove the fix", checked.err, run.calls, checked.green)
	}
}

func TestShipStop_AShadowRefusalOnANewerMainNamesTheMainOfTheStop(t *testing.T) {
	t.Parallel()
	first := newBaseRedLane(t)
	held := first.holdStop(t, redKey)
	first.landOnMain(t, "main moves on", map[string]string{"docs/note.md": "a note\n"})
	l := first.laneOn(t, "cycle-2-pack")
	run := &scriptedBase{answers: [][]baseattr.Key{{redKey}}}
	opts := l.options(run.run)
	opts.CycleID, opts.RunID = 7, "run-7"

	err, _ := stopChecked(t, opts)

	got := l.emitted(core.CodeMainRedStopWouldRefuse)
	if err != nil || l.base == held.MainSHA || len(got) != 1 || got[0].Cycle != 7 || got[0].RunID != "run-7" || got[0].Fields["main"] != held.MainSHA || got[0].Fields["base"] != l.base || got[0].Fields["admission"] != "not-a-fix" {
		t.Errorf("err = %v, signals = %+v, want one would-refuse of cycle 7 and run-7 with the main %s of the stop and the lane base %s", err, got, held.MainSHA, l.base)
	}
}

func TestReleaseLandedFix_TheReleaseNamesTheCycleOfTheShip(t *testing.T) {
	t.Parallel()
	l := newBaseRedLane(t)
	l.holdStop(t, redKey)
	opts := l.options(nil)
	opts.CycleID, opts.RunID = 7, "run-7"

	res := (stopRelease{opts: opts, warn: opts.Stderr}).landedFix([]baseattr.Key{redKey}, RunResult{CommitSHA: "abc1234"}, nil)

	got := l.emitted(core.CodeMainRedStopReleased)
	if len(res.Logs) != 0 || l.stop(t).ReleasedBy != "cycle 7 abc1234" || len(got) != 1 || got[0].Cycle != 7 || got[0].RunID != "run-7" {
		t.Errorf("logs = %v, stop = %+v, signals = %+v, want a release by cycle 7 of run-7", res.Logs, l.stop(t), got)
	}
	if _, err := os.Stat(filepath.Join(l.repo, ".evolve", "inbox", "processed", "cycle-7")); err != nil {
		t.Errorf("the fix item is not in processed/cycle-7: %v", err)
	}
}

func (l *baseRedLane) checkedAt(t *testing.T, after time.Duration) checkedStop {
	t.Helper()
	opts := l.options((&scriptedBase{answers: [][]baseattr.Key{{redKey}}}).run)
	at := time.Now().Add(after)
	opts.NowFn = func() Now { return Now{Unix: at.Unix()} }
	return stopCheckOf(t, opts)
}

func (l *baseRedLane) incidents(t *testing.T) []selfheal.Incident {
	t.Helper()
	incidents, err := selfheal.LoadIncidents(selfheal.Dir(filepath.Join(l.repo, ".evolve")))
	if err != nil {
		t.Fatalf("load the incidents: %v", err)
	}
	return incidents
}

func TestShipStop_AStopPastThePolicyBoundEscalatesOneTime(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name          string
		policy        string
		after         time.Duration
		wantEscalated bool
	}{
		{name: "past the default bound of 120 minutes", after: 3 * time.Hour, wantEscalated: true},
		{name: "inside the default bound", after: time.Hour},
		{name: "inside a longer bound of the policy", policy: `{"gates":{"main_red_stop_bound_minutes":600}}`, after: 3 * time.Hour},
		{name: "past a shorter bound of the policy", policy: `{"gates":{"main_red_stop_bound_minutes":30}}`, after: time.Hour, wantEscalated: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			l := newBaseRedLane(t)
			if tc.policy != "" {
				mustWrite(t, filepath.Join(l.repo, ".evolve", "policy.json"), tc.policy)
			}
			held := l.holdStop(t, redKey)

			first := l.checkedAt(t, tc.after)
			second := l.checkedAt(t, tc.after+time.Minute)

			if first.err != nil || second.err != nil || strings.Contains(first.log+second.log, "WARN main-red stop:") {
				t.Fatalf("errs %v, %v, log %q; the bound check holds no lane and warns of nothing", first.err, second.err, first.log+second.log)
			}
			want := 0
			if tc.wantEscalated {
				want = 1
			}
			got, incidents := l.emitted(core.CodeMainRedStopEscalated), l.incidents(t)
			if len(got) != want || len(incidents) != want || (l.stop(t).Escalated != nil) != tc.wantEscalated {
				t.Fatalf("%d signals, %d incidents, escalated_at %v; want %d escalation in two checks", len(got), len(incidents), l.stop(t).Escalated, want)
			}
			if !tc.wantEscalated {
				return
			}
			if !strings.Contains(incidents[0].Summary, held.Cause()) || !strings.Contains(incidents[0].Summary, held.ResumeCondition()) {
				t.Errorf("summary = %q, want the cause and the resume condition", incidents[0].Summary)
			}
			if got[0].Fields["incident"] != incidents[0].ID || got[0].Fields["main"] != held.MainSHA || got[0].Severity != signalcenter.SeverityIncident {
				t.Errorf("signal = %+v, want the incident %s of the stop on %s", got[0], incidents[0].ID, held.MainSHA)
			}
		})
	}
}

func TestShipStop_AnEscalationThatFailsIsAWarningAndTheCheckGoesOn(t *testing.T) {
	t.Parallel()
	l := newBaseRedLane(t)
	l.enforceStop(t)
	l.holdStop(t, redKey)
	mustWrite(t, selfheal.Dir(filepath.Join(l.repo, ".evolve")), "a file, not a directory")

	c := l.checkedAt(t, 3*time.Hour)

	if !strings.Contains(c.log, "[ship] WARN main-red stop: the escalation of the stop past its bound") {
		t.Errorf("log = %q, want the warning of the failed escalation", c.log)
	}
	wantShipErr(t, c.err, shiperr.CodeMainRedPark, core.ShipClassPrecondition, redPkg+"."+redTest)
	if l.stop(t).Escalated != nil || len(l.emitted(core.CodeMainRedStopEscalated)) != 0 {
		t.Error("a failed escalation marked the stop or emitted the signal")
	}
}
