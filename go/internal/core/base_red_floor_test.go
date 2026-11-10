package core

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
	"github.com/mickeyyaya/evolve-loop/go/internal/covchanged"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func repoContractReview(t *testing.T, f baseRedFixture, reds ...string) ReviewResult {
	t.Helper()
	f.in.Worktree = worktreeWithModule(t)
	pack := &fakeRepoContractPack{reds: reds, diagnostic: "--- FAIL: pack\n", err: errors.New("exit status 1")}
	floor := BuildHandoffFloor{{Name: "repo-contract", Run: RepoContractFloorChecks(pack.run)}}
	return floor.Review(context.Background(), f.in)
}

func TestRepoContractFloor_ABaseRedPackRedDoesNotRejectTheLane(t *testing.T) {
	t.Parallel()
	f := newBaseRedFixture(t, "TestA")
	res := repoContractReview(t, f, packRedNames("TestA")...)
	if !res.Approve || res.Retry {
		t.Fatalf("a red that the base shows too does not send the builder back; got Approve=%v Retry=%v %q", res.Approve, res.Retry, res.Reason)
	}
	if !f.saw(CodeBuildBaseRedAttributed) || !strings.Contains(f.stderr.String(), "NOTE") || !strings.Contains(f.stderr.String(), "TestA") {
		t.Fatalf("the pass writes a NOTE and emits the attribution signal; codes=%v stderr=%q", f.codes(), f.stderr.String())
	}
}

func TestRepoContractFloor_ALaneOnlyRedStillRejectsAndListsTheBaseRedKeys(t *testing.T) {
	t.Parallel()
	f := newBaseRedFixture(t, "TestOld")
	res := repoContractReview(t, f, packRedNames("TestOld", "TestNew")...)
	if res.Approve || !res.Retry {
		t.Fatalf("a lane-only red rejects; got Approve=%v Retry=%v", res.Approve, res.Retry)
	}
	if !strings.Contains(res.Reason, baseRedPkg+".TestNew") {
		t.Fatalf("the rejection names the lane-only key; got %q", res.Reason)
	}
	if !strings.Contains(res.Reason, "red on the base too, not refused: "+baseRedPkg+".TestOld") {
		t.Fatalf("the rejection lists the base-red key as information; got %q", res.Reason)
	}
	if strings.Contains(res.Reason, "RED: "+baseRedPkg+".TestOld") {
		t.Fatalf("the base-red key is not named as a lane red; got %q", res.Reason)
	}
	_, info, _ := strings.Cut(res.Reason, "red on the base too, not refused: ")
	if strings.Contains(info, "TestNew") {
		t.Fatalf("the lane-only key is not listed as a base red; got %q", info)
	}
}

func TestFloor_AnUnjudgedAttributionBlamesTheLaneAndSignals(t *testing.T) {
	t.Parallel()
	f := newBaseRedFixture(t, "TestA")
	f.runs.err = errors.New("worktree add failed")
	res := repoContractReview(t, f, packRedNames("TestA")...)
	if res.Approve || !res.Retry {
		t.Fatalf("an attribution that cannot run blames the lane; got Approve=%v Retry=%v", res.Approve, res.Retry)
	}
	if !f.saw(CodeBuildBaseRedUnjudged) || f.saw(CodeBuildBaseRedAttributed) {
		t.Fatalf("the unjudged signal is emitted alone; codes=%v", f.codes())
	}
	if !strings.Contains(f.stderr.String(), "WARN") || !strings.Contains(res.Reason, "worktree add failed") {
		t.Fatalf("the WARN and the rejection carry the cause; stderr=%q reason=%q", f.stderr.String(), res.Reason)
	}
}

func TestFloor_ANonAttributableRedRejectsWithNoSignalAndNoBaseRun(t *testing.T) {
	t.Parallel()
	f := newBaseRedFixture(t)
	res := repoContractReview(t, f, "not-a-test-name")
	if res.Approve || len(f.codes()) != 0 || f.runs.calls != 0 {
		t.Fatalf("a red with no key is blamed as today; approve=%v codes=%v runs=%d", res.Approve, f.codes(), f.runs.calls)
	}
	if !strings.Contains(res.Reason, "RED: not-a-test-name") {
		t.Fatalf("the rejection names the red that has no key; got %q", res.Reason)
	}
}

func TestUnitAndCoverFloors_SubtractOnlyKeysRedOnTheBase(t *testing.T) {
	f := newBaseRedFixture(t, "TestOld")
	paths := []string{"go/q/q.go"}
	prev, prevProfiler := buildSelfCheckRunner, coverChangedProfiler
	t.Cleanup(func() { buildSelfCheckRunner, coverChangedProfiler = prev, prevProfiler })
	failing := goTestRed("TestOld", "TestNew")
	buildSelfCheckRunner = func(context.Context, string, string) (string, bool) { return failing, false }
	coverChangedProfiler = func(context.Context, ReviewInput, string, []string, []string) (string, string, int) {
		return "", failing, coverStatusTestsFailed
	}
	writeFile(t, filepath.Join(f.in.Worktree, "go", "q", "q.go"), "package q\n")
	var flags []string
	f.in.baseRun = func(ctx context.Context, dir string, r baseattr.Reds) ([]baseattr.Key, error) {
		flags = r.Flags
		return f.runs.run(ctx, dir, r)
	}
	writeFile(t, filepath.Join(f.in.Worktree, ".evolve", "policy.json"), `{"cover_changed":{"stage":"enforce"}}`)

	unit := changedPackageFloorChecks(context.Background(), f.in, paths)
	cover := coverChangedFailures(context.Background(), f.in, paths)
	if want := "-tags " + strings.Join(covchanged.DefaultTags, ","); strings.Join(flags, " ") != want {
		t.Errorf("the cover floor reran the base with the flags %v, want %s", flags, want)
	}
	for name, got := range map[string][]string{"unit": unit, "cover": cover} {
		joined := strings.Join(got, "\n")
		if !strings.Contains(joined, "TestNew") {
			t.Errorf("%s floor must refuse the lane-only red; got %q", name, joined)
		}
		if !strings.Contains(joined, "red on the base too, not refused: "+baseRedPkg+".TestOld") {
			t.Errorf("%s floor must list the base red key as information; got %q", name, joined)
		}
	}

	f.runs.red = append(f.runs.red, baseattr.Key{Package: baseRedPkg, Test: "TestNew"})
	if got, err := UnitTestFloorJudge(context.Background(), f.in); err != nil || len(got) != 0 {
		t.Errorf("unit judge with every red base-red: got %v, %v", got, err)
	}
	if got, err := CoverChangedFloorJudge(context.Background(), f.in); err != nil || len(got) != 0 {
		t.Errorf("cover judge with every red base-red: got %v, %v", got, err)
	}
}

func TestFloor_AMainRedBaseRedOpensTheStopThroughTheHook(t *testing.T) {
	t.Parallel()
	f := newBaseRedFixture(t, "TestA")
	f.in.Cycle = 7
	var got baseattr.Verdict
	var cycle int
	f.in.MainRedStop = func(_ string, v baseattr.Verdict, c int) (bool, error) {
		got, cycle = v, c
		return false, errors.New("record locked")
	}

	res := repoContractReview(t, f, packRedNames("TestA")...)

	if !res.Approve || len(got.MainRed) != 1 || cycle != 7 {
		t.Fatalf("a main-red base-red reaches the hook; approve=%v verdict=%+v cycle=%d", res.Approve, got, cycle)
	}
	if !strings.Contains(f.stderr.String(), "record locked") {
		t.Fatalf("a hook error is a WARN, never a rejection; stderr=%q", f.stderr.String())
	}
	called := false
	f.in.MainRedStop = func(string, baseattr.Verdict, int) (bool, error) { called = true; return false, nil }
	f.in.openMainRedStop(baseattr.Verdict{BaseRed: got.BaseRed, Main: "abc"})
	if called {
		t.Fatalf("a green main opens no stop")
	}
	if res := repoContractReview(t, newBaseRedFixture(t, "TestA"), packRedNames("TestA")...); !res.Approve {
		t.Fatalf("a nil hook is not an error; got %+v", res)
	}
}

func TestReviewInputAttributor_TakesItsRerunLimitFromThePolicy(t *testing.T) {
	t.Parallel()
	cases := map[string]time.Duration{`{"gates":{"base_red_rerun_seconds":90}}`: 90 * time.Second, `{}`: policy.Policy{}.GatesConfig().BaseRedRerun}
	for body, want := range cases {
		root := floorRoot(t, body)
		a := ReviewInput{ProjectRoot: root, Stderr: &injectedStderr{}}.attributor()
		if a.Timeout != want || a.MainRef != "main" || a.Scratch != filepath.Join(".evolve", "ephemeral", "base-red") || a.Git.Dir != root {
			t.Errorf("policy %s: attributor = %+v, want Timeout %v", body, a, want)
		}
	}
	if a := (ReviewInput{Worktree: "/w"}).attributor(); a.Git.Dir != "/w" || a.Serialize == nil || a.Run == nil {
		t.Errorf("without a project root the worktree is the git seam: %+v", a)
	}
}

func TestReviewInputAttributor_SerializesOnTheIntegrationTierLock(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	release := ReviewInput{ProjectRoot: root, Stderr: &injectedStderr{}}.attributor().Serialize(context.Background())
	if _, err := os.Stat(filepath.Join(root, ".evolve")); err != nil {
		t.Errorf("the serialize hook takes the tier lock under the project root: %v", err)
	}
	release()
}

func TestGoTestFailureLines_APanicGivesNoLaneFreePass(t *testing.T) {
	t.Parallel()
	out := "--- FAIL: TestA (0.00s)\npanic: boom\nFAIL\t" + baseRedPkg + "\t0.1s\n"
	lines, tests := goTestFailureLines(out)
	if len(lines) != 2 || len(tests) != 1 || tests[0] != "TestA" {
		t.Fatalf("display lines and test names stay; got %v %v", lines, tests)
	}
	f := newBaseRedFixture(t, "TestA")
	buildFail := []selfCheckFailure{{Pkg: "./q", Output: out}}
	if got := f.in.laneFailures(context.Background(), buildFail); len(got) != 1 || f.runs.calls != 0 {
		t.Fatalf("a panic output is never excused by the base; kept %v after %d base runs", got, f.runs.calls)
	}
}

func TestBaseRedSignalCodes_RegisterUnderTheBuildModule(t *testing.T) {
	t.Parallel()
	for _, c := range []signalcenter.Code{CodeBuildBaseRedAttributed, CodeBuildBaseRedUnjudged} {
		if !c.BelongsTo(signalcenter.ModuleBuild) {
			t.Errorf("%s does not belong to the build module", c)
		}
	}
}

func TestFailOpen_AJudgeErrorWarnsAndPasses(t *testing.T) {
	t.Parallel()
	var sink injectedStderr
	check := FailOpen("probe", func(context.Context, ReviewInput) ([]string, error) { return []string{"x"}, errors.New("no toolchain") })
	if got := check(context.Background(), ReviewInput{Stderr: &sink}); got != nil || !strings.Contains(sink.String(), "WARN probe no toolchain") {
		t.Fatalf("got %v stderr %q", got, sink.String())
	}
}

func TestLaneFailures_AnOutputWithNoParsedRedStaysALaneFailure(t *testing.T) {
	t.Parallel()
	f := newBaseRedFixture(t, "TestA")
	writeFile(t, filepath.Join(f.in.ProjectRoot, ".evolve", "policy.json"), "{not json")
	fails := []selfCheckFailure{{Pkg: "./q", Output: "go: exec: no toolchain"}}
	if got := f.in.laneFailures(context.Background(), fails); len(got) != 1 || f.runs.calls != 0 {
		t.Fatalf("a red with no parsed test is never excused; kept %v after %d base runs", got, f.runs.calls)
	}
	if f.stderr.String() != "" {
		t.Fatalf("with no attributable red the floor reads no policy; stderr=%q", f.stderr.String())
	}
}

func TestLaneFailures_AttributesEachFailingPackageInOneBaseRun(t *testing.T) {
	t.Parallel()
	f := newBaseRedFixture(t, "TestOld")
	other := "--- FAIL: TestNew (0.00s)\nFAIL\texample.com/m/r\t0.1s\nFAIL\n"
	fails := []selfCheckFailure{{Pkg: "./q", Output: goTestRed("TestOld")}, {Pkg: "./r", Output: other}, {Pkg: "./s", Output: "panic: boom"}}
	got := f.in.laneFailures(context.Background(), fails)
	if len(got) != 2 || got[0].Pkg != "./r" || got[1].Pkg != "./s" {
		t.Fatalf("only the lane-only package and the unparsed one stay; got %+v", got)
	}
	if !strings.Contains(got[0].Output, "red on the base too, not refused: "+baseRedPkg+".TestOld") {
		t.Fatalf("the kept failure lists the base red of the excused package; got %q", got[0].Output)
	}
	if f.runs.calls != 1 {
		t.Fatalf("%d base runs for two attributable packages, want 1", f.runs.calls)
	}
}

func TestCoverRedLines_RerunsTheBaseWithTheFlagsOfTheCoverRun(t *testing.T) {
	t.Parallel()
	f := newBaseRedFixture(t, "TestA")
	var flags []string
	f.in.baseRun = func(ctx context.Context, dir string, r baseattr.Reds) ([]baseattr.Key, error) {
		flags = r.Flags
		return f.runs.run(ctx, dir, r)
	}
	if got := f.in.coverRedLines(context.Background(), goTestRed("TestA"), []string{"-tags", "t"}); got != nil {
		t.Fatalf("a base-red cover run is not refused; got %v", got)
	}
	if strings.Join(flags, " ") != "-tags t" {
		t.Fatalf("the base run got the flags %v, want -tags t", flags)
	}
}

func TestCoverRedLines_AnOutputWithNoParsedRedStaysRefused(t *testing.T) {
	t.Parallel()
	f := newBaseRedFixture(t, "TestA")
	if got := f.in.coverRedLines(context.Background(), "go: exec: no toolchain", nil); len(got) == 0 || f.runs.calls != 0 {
		t.Fatalf("a cover red with no parsed test is never excused; got %v after %d base runs", got, f.runs.calls)
	}
}

func TestLaneFailures_AnUnjudgedCauseReachesEachKeptFailure(t *testing.T) {
	t.Parallel()
	f := newBaseRedFixture(t, "TestOld")
	f.runs.err = errors.New("worktree add failed")
	other := "--- FAIL: TestNew (0.00s)\nFAIL\texample.com/m/r\t0.1s\nFAIL\n"
	got := f.in.laneFailures(context.Background(), []selfCheckFailure{{Pkg: "./q", Output: goTestRed("TestOld")}, {Pkg: "./r", Output: other}})
	if len(got) != 2 {
		t.Fatalf("an unjudged run blames each package; got %+v", got)
	}
	for _, kept := range got {
		if !strings.Contains(kept.Output, "worktree add failed") {
			t.Errorf("%s does not say why the base run did not excuse it: %q", kept.Pkg, kept.Output)
		}
	}
}

func TestReportAttributed_TheSignalCarriesTheMainRedKeysAndTheDuration(t *testing.T) {
	t.Parallel()
	f := newBaseRedFixture(t)
	a, b := baseattr.Key{Package: baseRedPkg, Test: "TestA"}, baseattr.Key{Package: baseRedPkg, Test: "TestB"}
	f.in.reportAttributed(baseattr.Verdict{BaseRed: []baseattr.Key{a, b}, MainRed: []baseattr.Key{a}, Took: 2 * time.Second})
	fields := f.signals.Recent()[0].Fields
	want := map[string]string{"keys": baseRedPkg + ".TestA, " + baseRedPkg + ".TestB", "main_red": baseRedPkg + ".TestA", "took": "2s"}
	for name, value := range want {
		if fields[name] != value {
			t.Errorf("field %s = %q, want %q", name, fields[name], value)
		}
	}
}

func TestTagFlags_OneHomeForTheCoverTagFlags(t *testing.T) {
	t.Parallel()
	if got := tagFlags(nil); got != nil {
		t.Errorf("no tags gives no flags; got %v", got)
	}
	if got := tagFlags([]string{"integration", "acs"}); strings.Join(got, " ") != "-tags integration,acs" {
		t.Errorf("tagFlags = %v", got)
	}
	if got := coverTestArgs("/p", []string{"./a"}, []string{"integration"}); !slices.Contains(got, "integration") {
		t.Errorf("coverTestArgs does not use the shared flags: %v", got)
	}
}

func TestReviewInputAttributor_UsesTheBaseattrDefaults(t *testing.T) {
	t.Parallel()
	a := ReviewInput{ProjectRoot: t.TempDir(), Stderr: &injectedStderr{}}.attributor()
	if a.MainRef != baseattr.DefaultMainRef || a.Scratch != baseattr.DefaultScratch {
		t.Errorf("core builds its own defaults: %q %q", a.MainRef, a.Scratch)
	}
}
