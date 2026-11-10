package ship

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/baseattr"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestShipGate_BaseRedWithGreenMainAsksForTheFleetRebase(t *testing.T) {
	l := newBaseRedLane(t)
	l.landOnMain(t, "main fixes the test", map[string]string{redTestFile: greenBody})
	swapRepoContractTest(t, redPack(redPkg+"."+redTest))

	err := l.ship(t)

	se := wantShipErr(t, err, shiperr.CodeGitFleetRebaseNeeded, core.ShipClassTransient, redPkg+"."+redTest)
	if se.Debug["keys"] != redPkg+"."+redTest {
		t.Errorf("debug %v: the rebase error carries the base-red keys", se.Debug)
	}
	if se.Debug["main"] == "" || se.Debug["base"] != l.base {
		t.Errorf("debug %v: the rebase error carries the base and main", se.Debug)
	}
	if got := l.emitted(CodeBaseRedRebase); len(got) != 1 || got[0].RunID != "test-run" || got[0].Cycle != 1 || got[0].Kind != signalcenter.KindGateCorrected || got[0].Severity != signalcenter.SeverityInfo || got[0].Fields["keys"] == "" || got[0].Fields["base"] != l.base || got[0].Fields["main"] == "" || got[0].Fields["took"] == "" {
		t.Errorf("one SHIP_BASE_RED_REBASE with keys, base, main and took; got %v", got)
	}
}

func TestShipGate_BaseRedWithRedMainParks(t *testing.T) {
	l := newBaseRedLane(t)
	l.landOnMain(t, "main moves on and stays red", map[string]string{"docs/note.md": "a note\n"})
	swapRepoContractTest(t, redPack(redPkg+"."+redTest))

	err := l.ship(t)

	se := wantShipErr(t, err, shiperr.CodeMainRedPark, core.ShipClassPrecondition, redPkg+"."+redTest)
	if se.Debug["keys"] != redPkg+"."+redTest {
		t.Errorf("debug %v: the park error carries the base-red keys", se.Debug)
	}
	if se.Debug["main"] == "" || se.Debug["base"] != l.base {
		t.Errorf("debug %v: the park error carries the base and main", se.Debug)
	}
	if got := l.emitted(CodeBaseRedMainRed); len(got) != 1 || got[0].RunID != "test-run" || got[0].Cycle != 1 || got[0].Kind != signalcenter.KindShipWarning || got[0].Severity != signalcenter.SeverityWarn || !strings.Contains(got[0].Fields["keys"], redTest) {
		t.Errorf("one SHIP_BASE_RED_MAIN_RED naming the keys; got %v", got)
	}
}

func attributed(t *testing.T, opts *Options, o packOutcome) (error, string) {
	t.Helper()
	var log strings.Builder
	return attributeContractRed(context.Background(), baseRedGate{opts: opts, base: opts.WorktreeBaseSHA}, &log, o), log.String()
}

func TestAttributeContractRed_ALaneOnlyKeyBlamesTheLane(t *testing.T) {
	t.Parallel()
	l := newBaseRedLane(t)
	base := &scriptedBase{answers: [][]baseattr.Key{nil}}

	err, log := attributed(t, l.options(base.run), redPack(redPkg+"."+redTest))

	if err != nil || base.calls != 1 || log != "" {
		t.Errorf("a key green on the base blames the lane with no error and no WARN; got err=%v calls=%d log=%q", err, base.calls, log)
	}
	if len(l.events) != 0 {
		t.Errorf("a blamed lane emits no base-red signal; got %v", l.events)
	}
}

func TestAttributeContractRed_ALaneOnlyKeyAmongBaseRedKeysBlamesTheLane(t *testing.T) {
	t.Parallel()
	l := newBaseRedLane(t)
	base := &scriptedBase{answers: [][]baseattr.Key{{redKey}}}

	err, _ := attributed(t, l.options(base.run), redPack(redPkg+"."+redTest, redPkg+".TestOther"))

	if err != nil || base.calls != 1 {
		t.Errorf("one lane-only key blames the whole red; got err=%v calls=%d", err, base.calls)
	}
}

func TestAttributeContractRed_AnUnnamedFailureBlamesTheLaneWithoutABaseRun(t *testing.T) {
	t.Parallel()
	l := newBaseRedLane(t)
	base := &scriptedBase{}

	err, _ := attributed(t, l.options(base.run), redPack(redPkg+" [build failed]"))

	if err != nil || base.calls != 0 {
		t.Errorf("a build failure has no key to rerun; got err=%v calls=%d", err, base.calls)
	}
}

func TestAttributeContractRed_AnAttributionErrorBlamesTheLaneAndWritesAWarn(t *testing.T) {
	t.Parallel()
	l := newBaseRedLane(t)
	base := &scriptedBase{err: errors.New("go test did not start")}

	err, log := attributed(t, l.options(base.run), redPack(redPkg+"."+redTest))

	if err != nil || !strings.Contains(log, "WARN") || !strings.Contains(log, "go test did not start") {
		t.Errorf("an attribution error never excuses the lane and leaves a WARN; got err=%v log=%q", err, log)
	}
}

func TestAttributeContractRed_MainAtTheBaseParksAfterOneBaseRun(t *testing.T) {
	t.Parallel()
	l := newBaseRedLane(t)
	base := &scriptedBase{answers: [][]baseattr.Key{{redKey}}}

	err, _ := attributed(t, l.options(base.run), redPack(redPkg+"."+redTest))

	wantShipErr(t, err, shiperr.CodeMainRedPark, core.ShipClassPrecondition, redPkg+"."+redTest)
	if base.calls != 1 {
		t.Errorf("main is the base, so one run judges both; got %d", base.calls)
	}
	if scratch := filepath.Join(l.repo, baseattr.DefaultScratch); len(base.dirs) != 1 || !strings.HasPrefix(base.dirs[0], scratch) {
		t.Errorf("the base checkout is under the project scratch %s, never in the lane tree; got %v", scratch, base.dirs)
	}
}

func TestAttributeContractRed_AMovedMainDecidesBetweenTheRebaseAndThePark(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		main []baseattr.Key
		want shiperr.ShipErrorCode
	}{
		{"main is green", nil, shiperr.CodeGitFleetRebaseNeeded},
		{"main is red", []baseattr.Key{redKey}, shiperr.CodeMainRedPark},
	} {
		l := newBaseRedLane(t)
		l.landOnMain(t, "main moves", map[string]string{"docs/note.md": "a note\n"})
		base := &scriptedBase{answers: [][]baseattr.Key{{redKey}, tc.main}}

		err, _ := attributed(t, l.options(base.run), redPack(redPkg+"."+redTest))

		se, ok := core.AsShipError(err)
		if !ok || se.Code != tc.want || base.calls != 2 {
			t.Errorf("%s: got %v after %d runs, want %s after 2", tc.name, err, base.calls, tc.want)
		}
	}
}

func TestContractVerdict_OnlyTheScannerPackWithAWiredGateIsAttributed(t *testing.T) {
	t.Parallel()
	l := newBaseRedLane(t)
	base := &scriptedBase{answers: [][]baseattr.Key{{redKey}}}
	red := redPack(redPkg + "." + redTest)
	wired := withBaseRed(context.Background(), l.options(base.run), l.base)

	for name, got := range map[string]error{
		"no gate in the context":  contractVerdict(context.Background(), io.Discard, scannerPackName, red),
		"the importer backstop":   contractVerdict(wired, io.Discard, "importer backstop", red),
		"the added-test backstop": contractVerdict(wired, io.Discard, "added-test backstop", red),
	} {
		wantShipErr(t, got, shiperr.CodeRepoContractGate, core.ShipClassPrecondition, redPkg+"."+redTest)
		if base.calls != 0 {
			t.Errorf("%s: ran the base %d time(s)", name, base.calls)
		}
	}
	wantShipErr(t, contractVerdict(wired, io.Discard, scannerPackName, red), shiperr.CodeMainRedPark, core.ShipClassPrecondition, redTest)
}

func TestRunClassifiedPack_ARedSecondRunAfterAnAmbiguousFirstIsAttributedToo(t *testing.T) {
	t.Parallel()
	l := newBaseRedLane(t)
	base := &scriptedBase{answers: [][]baseattr.Key{{redKey}}}
	runs := []packOutcome{ambiguousPack(), redPack(redPkg + "." + redTest)}
	next := func() packOutcome {
		o := runs[0]
		runs = runs[1:]
		return o
	}

	err := runClassifiedPackRetrying(withBaseRed(context.Background(), l.options(base.run), l.base), io.Discard, "", scannerPackName, true, next, nil)

	wantShipErr(t, err, shiperr.CodeMainRedPark, core.ShipClassPrecondition, redTest)
}

func TestReplay_20261109(t *testing.T) {
	l := newBaseRedLane(t)
	runGit(t, l.wt, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "lane 2240 work")
	l.landOnMain(t, "F: main fixes the red test", map[string]string{redTestFile: greenBody})
	swapRepoContractTest(t, redPack(redPkg+"."+redTest))

	wantShipErr(t, l.ship(t), shiperr.CodeGitFleetRebaseNeeded, core.ShipClassTransient, redPkg+"."+redTest)
	runGit(t, l.wt, "rebase", "main")
	l.base = strings.TrimSpace(runGitOut(t, l.repo, "rev-parse", "main"))
	swapRepoContractTest(t, greenPack())
	if err := l.gateOnly(t); err != nil {
		t.Fatalf("step 2: after the rebase onto F the gate is green; got %v", err)
	}

	l.landOnMain(t, "M2: main is red again", map[string]string{redTestFile: redBody})
	lane2242 := l.laneOn(t, "cycle-2242")
	var opened []baseattr.Verdict
	floor := core.BuildHandoffFloor{{Name: "repo-contract", Run: core.RepoContractFloorChecks(redNamedPack(redPkg + "." + redTest))}}
	res := floor.Review(context.Background(), core.ReviewInput{
		Phase: string(core.PhaseBuild), Cycle: 2242, Worktree: lane2242.wt, ProjectRoot: l.repo, WorktreeBaseSHA: lane2242.base,
		Stderr: io.Discard, Signals: lane2242.signals,
		MainRedStop: func(_ string, v baseattr.Verdict, _ int) (bool, error) { opened = append(opened, v); return true, nil },
	})
	if !res.Approve || res.Retry {
		t.Fatalf("step 3: the build floor approves lane 2242 on a red base; got %+v", res)
	}
	if len(opened) != 1 || len(opened[0].MainRed) != 1 || opened[0].MainRed[0] != redKey {
		t.Fatalf("step 3: the floor hands the red main to the stop sink once; got %+v", opened)
	}

	swapRepoContractTest(t, redPack(redPkg+"."+redTest))
	lane2249 := l.laneOn(t, "cycle-2249")
	wantShipErr(t, lane2249.ship(t), shiperr.CodeMainRedPark, core.ShipClassPrecondition, redPkg+"."+redTest)
}

func redNamedPack(names ...string) core.RepoContractPackFn {
	return func(context.Context, string, string) ([]string, string, error) {
		return names, "--- FAIL: " + redTest + "\n", errors.New("exit status 1")
	}
}

func TestAttributeContractRed_AMalformedPolicyIsReportedInThePackLog(t *testing.T) {
	t.Parallel()
	l := newBaseRedLane(t)
	policyPath := filepath.Join(l.repo, ".evolve", "policy.json")
	if err := os.MkdirAll(filepath.Dir(policyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policyPath, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	base := &scriptedBase{answers: [][]baseattr.Key{nil}}

	err, log := attributed(t, l.options(base.run), redPack(redPkg+"."+redTest))

	if err != nil || !strings.Contains(log, "policy") {
		t.Errorf("a lane-only red stays blamed (err %v) and the pack log names the unreadable policy; log %q", err, log)
	}
}

func TestBaseRedError_AnUnjudgedMainParksWithAnEmptyMain(t *testing.T) {
	t.Parallel()
	l := newBaseRedLane(t)

	err := baseRedError(l.options(nil), baseattr.Verdict{BaseRed: []baseattr.Key{redKey}, Base: l.base})

	se := wantShipErr(t, err, shiperr.CodeMainRedPark, core.ShipClassPrecondition, redTest)
	if se.Debug["main"] != "" || len(l.emitted(CodeBaseRedMainRed)) != 1 {
		t.Errorf("an unjudged main parks with an empty main and one signal; debug %v", se.Debug)
	}
}

func TestAttributeContractRed_ABuildFailureBesideABaseRedKeyBlamesTheLane(t *testing.T) {
	t.Parallel()
	l := newBaseRedLane(t)
	base := &scriptedBase{answers: [][]baseattr.Key{{redKey}}}

	err, _ := attributed(t, l.options(base.run), redPack(redPkg+"."+redTest, redPkg+"/other [build failed]"))

	if err != nil || base.calls != 0 {
		t.Errorf("a build failure beside a base-red key is never excused; got err=%v calls=%d", err, base.calls)
	}
}

func TestBaseRedAttributor_TakesTheRerunLimitFromThePolicy(t *testing.T) {
	t.Parallel()
	l := newBaseRedLane(t)
	policyPath := filepath.Join(l.repo, ".evolve", "policy.json")
	if err := os.MkdirAll(filepath.Dir(policyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policyPath, []byte(`{"gates":{"base_red_rerun_seconds":77}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := baseRedAttributor(l.options(nil), io.Discard).Timeout; got != 77*time.Second {
		t.Errorf("the rerun limit is the policy value 77s; got %s", got)
	}
}

func TestBaseRedError_TheParkCarriesOnlyTheKeysThatAreRedOnMain(t *testing.T) {
	t.Parallel()
	l := newBaseRedLane(t)
	greenOnMain := baseattr.Key{Package: "example.com/m/other", Test: "TestGreenOnMain"}
	sent := baseattr.Verdict{BaseRed: []baseattr.Key{greenOnMain, redKey}, MainRed: []baseattr.Key{redKey}, Base: l.base, Main: "9a2bbe0e1c0ffee"}

	err := baseRedError(l.options(nil), sent)

	se := wantShipErr(t, err, shiperr.CodeMainRedPark, core.ShipClassPrecondition, redTest)
	got := baseattr.VerdictOfDebug(se.Debug)
	if len(got.MainRed) != 1 || got.MainRed[0] != redKey || len(got.BaseRed) != 2 || got.Main != sent.Main || got.Base != sent.Base {
		t.Errorf("the park error carries %+v, want the one main-red key apart from the two base-red keys", got)
	}
}
