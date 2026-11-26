package ship

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/ciparity"
	"github.com/mickeyyaya/evolve-loop/go/internal/ciwatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/covchanged"
	"github.com/mickeyyaya/evolve-loop/go/internal/mainstop"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	gateStep       = "make cover-changed"
	gateDetail     = "step \"Cover changed\" failed; the message was not read; next: gh run view --job 7 --attempt 1 --log-failed | tail -n 200 (the end of the job log)"
	runURL         = "https://github.com/o/r/actions/runs/9"
	wantNoneParsed = "(none parsed)"
)

type scriptedCI struct {
	mu    sync.Mutex
	calls int
	shas  []string
	runs  []ciwatch.RunStatus
	err   error
}

func (c *scriptedCI) fetch(_ context.Context, sha string) (ciwatch.RunStatus, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	c.shas = append(c.shas, sha)
	if c.err != nil {
		return ciwatch.RunStatus{}, c.err
	}
	return c.runs[min(c.calls, len(c.runs))-1], nil
}

func redRun(failing ...ciwatch.FailingTest) ciwatch.RunStatus {
	return ciwatch.RunStatus{Status: ciwatch.StatusCompleted, Conclusion: "failure", RunURL: runURL, FailingTests: failing}
}

func ciOf(runs ...ciwatch.RunStatus) *scriptedCI { return &scriptedCI{runs: runs} }

func stepRow() ciwatch.FailingTest {
	return ciwatch.FailingTest{Step: gateStep, Jobs: []string{"cover"}, Detail: gateDetail, JobID: 7, Attempt: 1}
}

type provenAtMain struct {
	mu    sync.Mutex
	calls int
	asked []baseattr.Reds
	red   []baseattr.Key
	err   error
}

func (p *provenAtMain) run(_ context.Context, _ string, r baseattr.Reds) ([]baseattr.Key, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	p.asked = append(p.asked, r)
	return p.red, p.err
}

func (l *baseRedLane) mainCIOptions(ci *scriptedCI, at *provenAtMain) *Options {
	opts := l.options(at.run)
	opts.mainCI = ci.fetch
	return opts
}

func (l *baseRedLane) noStop(t *testing.T) {
	t.Helper()
	if _, found, _ := mainstop.Read(filepath.Join(l.repo, ".evolve")); found {
		t.Error("a stop was opened, want none")
	}
}

func (l *baseRedLane) wantOneUnprovable(t *testing.T, keys string) signalcenter.Event {
	t.Helper()
	got := l.emitted(CodeMainRedCIUnprovable)
	if len(got) != 1 {
		t.Fatalf("unprovable signals = %+v, want exactly one", got)
	}
	e := got[0]
	if e.Severity != signalcenter.SeverityWarn || e.Fields["keys"] != keys || e.Fields["main"] != l.base || e.Fields["run"] != runURL {
		t.Errorf("signal = %+v, want a warn that names keys %q, main %s and the run URL", e, keys, l.base)
	}
	if !strings.Contains(e.Reason, "the train refuses on the red main until it is green") {
		t.Errorf("reason = %q, want the next step for the operator", e.Reason)
	}
	return e
}

func TestProvable_OnlyAWellFormedTestKey(t *testing.T) {
	t.Parallel()
	for _, g := range ciparity.GateSteps {
		if provable(ciwatch.FailingTest{Step: g.Gate}) {
			t.Errorf("the gate row %q is provable, want no gate row provable", g.Gate)
		}
	}
	rows := map[string]struct {
		row  ciwatch.FailingTest
		want bool
	}{
		"package and test":        {ciwatch.FailingTest{Package: "p", Test: "TestX"}, true},
		"test without package":    {ciwatch.FailingTest{Test: "TestX"}, false},
		"package without test":    {ciwatch.FailingTest{Package: "p"}, false},
		"step only":               {ciwatch.FailingTest{Step: "s"}, false},
		"empty row":               {ciwatch.FailingTest{}, false},
		"job names only":          {ciwatch.FailingTest{Jobs: []string{"build"}}, false},
		"package test and a step": {ciwatch.FailingTest{Package: "p", Test: "TestX", Step: "s"}, true},
	}
	for name, c := range rows {
		if got := provable(c.row); got != c.want {
			t.Errorf("%s: provable = %v, want %v", name, got, c.want)
		}
	}
}

func TestCheckMainRedStop_UnrunnableRedOpensNoStopAndEmitsOnce(t *testing.T) {
	t.Parallel()
	pkgless := ciwatch.FailingTest{Test: "TestOrphan", Jobs: []string{"unit"}}
	cases := map[string]struct {
		run   ciwatch.RunStatus
		atErr error
		keys  string
		runs  int
	}{
		"a coverage gate row":         {redRun(stepRow()), nil, gateStep, 0},
		"a test with no package":      {redRun(pkgless), nil, "TestOrphan", 0},
		"a key that is green at main": {redRun(ciwatch.FailingTest{Package: redPkg, Test: redTest}), nil, redPkg + "." + redTest, 1},
		"no parsed row":               {redRun(), nil, wantNoneParsed, 0},
		"an empty row":                {redRun(ciwatch.FailingTest{}), nil, wantNoneParsed, 0},
		"a package only row":          {redRun(ciwatch.FailingTest{Package: redPkg}), nil, redPkg, 0},
		"a job only row":              {redRun(ciwatch.FailingTest{Jobs: []string{"build", "vet"}}), nil, "build, vet", 0},
		"a run at main that fails":    {redRun(ciwatch.FailingTest{Package: redPkg, Test: redTest}), errors.New("worktree add failed"), redPkg + "." + redTest, 1},
	}
	for name, c := range cases {
		for _, enforce := range []bool{false, true} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				l := newBaseRedLane(t)
				if enforce {
					l.enforceStop(t)
				}

				at := &provenAtMain{err: c.atErr}

				checked := stopCheckOf(t, l.mainCIOptions(ciOf(c.run), at))

				if checked.err != nil || len(checked.green) != 0 {
					t.Fatalf("err = %v, green = %v, want the lane free", checked.err, checked.green)
				}
				l.noStop(t)
				l.wantOneUnprovable(t, c.keys)
				if at.calls != c.runs {
					t.Errorf("the run at main happened %d times, want %d", at.calls, c.runs)
				}
				if got := l.emitted(CodeMainRedCIOpened); len(got) != 0 {
					t.Errorf("opened signals = %+v, want none", got)
				}
			})
		}
	}
}

func TestCheckMainRedStop_TheSignalCarriesTheHintAndTheCauseOfTheRow(t *testing.T) {
	t.Parallel()
	l := newBaseRedLane(t)
	stepAndTest := redRun(stepRow(), ciwatch.FailingTest{Package: redPkg, Test: redTest})
	at := &provenAtMain{err: errors.New("worktree add failed")}

	stopCheckOf(t, l.mainCIOptions(ciOf(stepAndTest), at))

	e := l.wantOneUnprovable(t, gateStep+", "+redPkg+"."+redTest)
	if !strings.Contains(e.Fields["detail"], "gh run view --job 7") || !strings.Contains(e.Fields["error"], "worktree add failed") {
		t.Errorf("fields = %v, want the hint of the gate row and the error of the run at main", e.Fields)
	}
}

func TestCheckMainRedStop_AWellFormedKeyThatIsRedAtMainOpensTheStopOnThatKeyOnly(t *testing.T) {
	t.Parallel()
	l := newBaseRedLane(t)
	ci := ciOf(redRun(ciwatch.FailingTest{Package: redPkg, Test: redTest}))
	at := &provenAtMain{red: []baseattr.Key{redKey}}

	stopCheckOf(t, l.mainCIOptions(ci, at))

	stop := l.stop(t)
	if !stop.Holds() || !slices.Equal(stop.Keys, []baseattr.Key{redKey}) || stop.MainSHA != l.base {
		t.Errorf("stop = %+v, want a stop on %v at the main tip %s", stop, redKey, l.base)
	}
	if len(at.asked) == 0 || !slices.Equal(at.asked[0].Keys, []baseattr.Key{redKey}) || !slices.Equal(at.asked[0].Flags, covchanged.TagFlags(covchanged.DefaultTags)) {
		t.Errorf("the run at main asked %+v in %d runs, want the proof reds of the key", at.asked, at.calls)
	}
	if got := l.emitted(CodeMainRedCIUnprovable); len(got) != 0 {
		t.Errorf("unprovable signals = %+v, want none when every key is held", got)
	}
	if got := l.emitted(CodeMainRedCIOpened); len(got) != 1 || got[0].Fields["keys"] != redPkg+"."+redTest || got[0].Fields["run"] != runURL || got[0].Fields["main"] != l.base {
		t.Errorf("opened signals = %+v, want one that names the key, the main tip and the run", got)
	}
	if !l.fixItemPending(t, stop.FixItem) {
		t.Errorf("the fix item %s is not in the inbox", stop.FixItem)
	}
}

func TestCheckMainRedStop_AMixedRunHoldsTheReproducedKeyAndSignalsTheRest(t *testing.T) {
	t.Parallel()
	l := newBaseRedLane(t)
	ci := ciOf(redRun(stepRow(), ciwatch.FailingTest{Package: redPkg, Test: redTest}, ciwatch.FailingTest{Package: redPkg, Test: "TestOther"}))
	at := &provenAtMain{red: []baseattr.Key{redKey}}

	stopCheckOf(t, l.mainCIOptions(ci, at))

	if got := l.stop(t).Keys; !slices.Equal(got, []baseattr.Key{redKey}) {
		t.Errorf("keys = %v, want only the key that is red at main", got)
	}
	l.wantOneUnprovable(t, gateStep+", "+redPkg+".TestOther")
	if !slices.Equal(at.asked[0].Keys, []baseattr.Key{redKey, otherKey}) {
		t.Errorf("the run at main asked %v, want both well formed keys and no gate row", at.asked[0].Keys)
	}
}

func TestCheckMainRedStop_ASecondLaneOnTheSameShaJudgesNothingAgain(t *testing.T) {
	t.Parallel()
	l := newBaseRedLane(t)
	ci := ciOf(redRun(stepRow()))
	at := &provenAtMain{}
	opts := l.mainCIOptions(ci, at)

	stopCheckOf(t, opts)
	stopCheckOf(t, l.mainCIOptions(ci, at))

	l.wantOneUnprovable(t, gateStep)
	if ci.calls != 1 {
		t.Errorf("the CI source was read %d times, want once: a judged sha skips the fetch", ci.calls)
	}
}

func TestCheckMainRedStop_LanesThatShipTogetherJudgeTheShaOnceWithoutADeadlock(t *testing.T) {
	t.Parallel()
	l := newBaseRedLane(t)
	ci := ciOf(redRun(ciwatch.FailingTest{Package: redPkg, Test: redTest}))
	at := &provenAtMain{red: []baseattr.Key{redKey}}
	var mu sync.Mutex
	opened := 0
	signals := signalcenter.New()
	signals.Subscribe(func(e signalcenter.Event) {
		if e.Code == CodeMainRedCIOpened {
			mu.Lock()
			opened++
			mu.Unlock()
		}
	})
	var wg sync.WaitGroup
	for range 4 {
		opts := l.mainCIOptions(ci, at)
		opts.Signals = signals
		wg.Add(1)
		go func() {
			defer wg.Done()
			checkMainRedStop(context.Background(), opts, &strings.Builder{})
		}()
	}
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if opened != 1 || !l.stop(t).Holds() {
		t.Errorf("opened signals = %d, stop holds = %v, want one stop opened by one lane", opened, l.stop(t).Holds())
	}
}

func TestCheckMainRedStop_ARunThatIsNotRedAndCompletedIsNotJudgedYet(t *testing.T) {
	t.Parallel()
	l := newBaseRedLane(t)
	pending := ciwatch.RunStatus{Status: "in_progress", FailingTests: []ciwatch.FailingTest{stepRow()}}
	green := ciwatch.RunStatus{Status: ciwatch.StatusCompleted, Conclusion: ciwatch.ConclusionSuccess}
	ci := ciOf(pending, green, redRun(stepRow()))

	for range 2 {
		stopCheckOf(t, l.mainCIOptions(ci, &provenAtMain{}))
	}
	if got := l.emitted(CodeMainRedCIUnprovable); len(got) != 0 {
		t.Fatalf("signals = %+v, want none before the run is completed and red", got)
	}

	stopCheckOf(t, l.mainCIOptions(ci, &provenAtMain{}))

	l.wantOneUnprovable(t, gateStep)
}

func TestCheckMainRedStop_AnUnreadableCISourceFailsOpenWithASignalAndJudgesNothing(t *testing.T) {
	t.Parallel()
	l := newBaseRedLane(t)
	l.enforceStop(t)
	ci := &scriptedCI{err: errors.New("gh is not logged in")}

	checked := stopCheckOf(t, l.mainCIOptions(ci, &provenAtMain{}))

	if checked.err != nil || len(checked.green) != 0 {
		t.Fatalf("err = %v, green = %v, want the lane free", checked.err, checked.green)
	}
	got := l.emitted(CodeMainRedCIUnread)
	if len(got) != 1 || got[0].Severity != signalcenter.SeverityWarn || !strings.Contains(got[0].Fields["error"], "gh is not logged in") || got[0].Fields["main"] != l.base {
		t.Errorf("signals = %+v, want one warn signal with the cause", got)
	}
	l.noStop(t)
	stopCheckOf(t, l.mainCIOptions(ci, &provenAtMain{}))
	if ci.calls != 2 {
		t.Errorf("the CI source was read %d times, want a retry on the next ship", ci.calls)
	}
}

func TestCheckMainRedStop_AMainThatCannotBeResolvedSkipsTheCISource(t *testing.T) {
	t.Parallel()
	l := newBaseRedLane(t)
	l.enforceStop(t)
	runGit(t, l.repo, "branch", "-m", "main", "trunk")
	ci := ciOf(redRun(stepRow()))

	checked := stopCheckOf(t, l.mainCIOptions(ci, &provenAtMain{}))

	if checked.err != nil || ci.calls != 0 || !strings.Contains(checked.log, "the CI source was not read") {
		t.Errorf("err = %v, CI reads = %d, log = %q, want the lane free, no CI read and a warning", checked.err, ci.calls, checked.log)
	}
}

func TestCheckMainRedStop_AHeldStopIsNotJudgedAgainAndItsShaCountsAsJudged(t *testing.T) {
	t.Parallel()
	l := newBaseRedLane(t)
	l.enforceStop(t)
	l.holdStop(t, redKey)
	ci := ciOf(redRun(ciwatch.FailingTest{Package: redPkg, Test: redTest}))
	at := &provenAtMain{red: []baseattr.Key{redKey}}

	checked := stopCheckOf(t, l.mainCIOptions(ci, at))
	if ci.calls != 0 || checked.err == nil {
		t.Fatalf("CI reads = %d, err = %v, want no read while a stop holds and a park for the lane", ci.calls, checked.err)
	}
	if err := (stopRelease{opts: l.options(nil), warn: &strings.Builder{}}).release([]baseattr.Key{redKey}, "cycle 9 abc"); err != nil {
		t.Fatal(err)
	}

	stopCheckOf(t, l.mainCIOptions(ci, at))

	if l.stop(t).Holds() || ci.calls != 0 || len(l.emitted(CodeMainRedCIOpened)) != 0 {
		t.Errorf("the released stop reopened on the same sha (CI reads = %d)", ci.calls)
	}
}

func TestCheckMainRedStop_AStopThatCannotBeOpenedLeavesTheShaUnjudgedForTheNextLane(t *testing.T) {
	t.Parallel()
	l := newBaseRedLane(t)
	mustWrite(t, filepath.Join(l.repo, ".evolve", "inbox"), "not a directory")
	ci := ciOf(redRun(ciwatch.FailingTest{Package: redPkg, Test: redTest}))
	at := &provenAtMain{red: []baseattr.Key{redKey}}

	stopCheckOf(t, l.mainCIOptions(ci, at))

	if mainstop.Judged(filepath.Join(l.repo, ".evolve"), l.base) {
		t.Error("the sha stays judged after the stop failed to open, want it released so the next lane judges it")
	}
	if got := l.emitted(CodeMainRedCIOpened); len(got) != 1 || got[0].Fields["open_error"] == "" {
		t.Errorf("opened signals = %+v, want one that carries the open error", got)
	}
}

func TestCheckMainRedStop_AJudgedRecordThatCannotBeReleasedWarnsTheLane(t *testing.T) {
	t.Parallel()
	l := newBaseRedLane(t)
	evolveDir := filepath.Join(l.repo, ".evolve")
	ci := ciOf(redRun(ciwatch.FailingTest{Package: redPkg, Test: redTest}))
	at := &provenAtMain{red: []baseattr.Key{redKey}}
	lockOut := func(ctx context.Context, sha string, r baseattr.Reds) ([]baseattr.Key, error) {
		if err := os.Chmod(evolveDir, 0o500); err != nil {
			t.Fatal(err)
		}
		return at.run(ctx, sha, r)
	}
	opts := l.options(lockOut)
	opts.mainCI = ci.fetch
	t.Cleanup(func() { _ = os.Chmod(evolveDir, 0o755) })

	checked := stopCheckOf(t, opts)

	if !strings.Contains(checked.log, "could not be released") {
		t.Errorf("log = %q, want a warning that the judged record could not be released", checked.log)
	}
}

func TestCheckMainRedStop_ARecordThatCannotBeWrittenFailsClosedWithASignal(t *testing.T) {
	t.Parallel()
	l := newBaseRedLane(t)
	mustWrite(t, filepath.Join(l.repo, ".evolve", "main-red-judged.json", "keep"), "x")
	ci := ciOf(redRun(stepRow()))

	checked := stopCheckOf(t, l.mainCIOptions(ci, &provenAtMain{}))

	if checked.err != nil || !strings.Contains(checked.log, "judged") {
		t.Errorf("err = %v, log = %q, want the lane free and a warning about the record", checked.err, checked.log)
	}
	l.noStop(t)
}

func TestShipPhase_AStepRedOnMainParksNoLane(t *testing.T) {
	l := newBaseRedLane(t)
	l.landable(t)
	l.enforceStop(t)
	swapRepoContractTest(t, greenPack())
	ci := ciOf(redRun(stepRow()))
	p := New(Config{Runner: execRunner, RepoContractGate: "enforce", Signals: l.signals, MainCI: ci.fetch})

	_, err := p.Run(context.Background(), core.PhaseRequest{
		Cycle: 1, RunID: "test-run", AuditRound: 1, ProjectRoot: l.repo, Workspace: filepath.Join(l.repo, ".evolve", "runs", "cycle-1"),
		Worktree: l.wt, WorktreeBaseSHA: l.base, Context: map[string]string{"commit_message": "evolve-cycle 1: goal=test"},
	})

	if err != nil {
		t.Fatalf("a step red on main parked the lane: %v", err)
	}
	if l.mainTip(t) == l.base {
		t.Error("the lane did not land on main")
	}
	l.noStop(t)
	l.wantOneUnprovable(t, gateStep)
}
