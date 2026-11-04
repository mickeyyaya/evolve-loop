package integration

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/acsrunner"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type batchSeams struct {
	f              composeFixture
	mu             sync.Mutex
	predRuns       []string
	testRuns       []string
	recorded       []string
	green          map[string]bool
	testFail       func(tip, pkg string, call int) error
	predRed        func(tip string, cycle, call int) bool
	affected       func(tip string, edited []string) ([]string, error)
	recordErr      error
	predErr        error
	noIDs          bool
	predErrAt      int
	predBoth       bool
	testErr        error
	testErrAt      int
	incomplete     bool
	bareIncomplete bool
	keys           []string
	calls          map[string]int
}

func newSeams(f composeFixture) *batchSeams {
	return &batchSeams{f: f, green: map[string]bool{}, calls: map[string]int{}}
}

func (s *batchSeams) Green(tip, pkg string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.keys = append(s.keys, "green "+s.where(tip)+" "+pkg)
	return s.green[pkg]
}

func (s *batchSeams) Record(tip, pkg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.keys = append(s.keys, "record "+s.where(tip)+" "+pkg)
	s.recorded = append(s.recorded, pkg)
	return s.recordErr
}

func (s *batchSeams) test(_ context.Context, tip, pkg string) (TestOutcome, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.testRuns = append(s.testRuns, s.where(tip)+" "+pkg)
	key := "t " + tip + " " + pkg
	s.calls[key]++
	if s.testErr != nil && s.calls[key] >= max(s.testErrAt, 1) {
		return TestOutcome{}, s.testErr
	}
	if s.testFail == nil {
		return TestOutcome{Green: true}, nil
	}
	if err := s.testFail(tip, pkg, s.calls[key]); err != nil {
		return TestOutcome{Output: err.Error()}, nil
	}
	return TestOutcome{Green: true}, nil
}

func (s *batchSeams) predicates(_ context.Context, tip string, cycle int) (acsrunner.Verdict, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.predRuns = append(s.predRuns, s.where(tip)+" "+strings.TrimSpace(CycleTrailer(cycle)))
	key := "p " + tip + " " + CycleTrailer(cycle)
	s.calls[key]++
	if s.predBoth {
		return acsrunner.Verdict{Cycle: cycle, RedCount: 1, RedIDs: []string{"egps/no-predicates"}}, errors.New("acsrunner go test: exit status 1")
	}
	if s.predErr != nil && s.calls[key] >= max(s.predErrAt, 1) {
		return acsrunner.Verdict{}, s.predErr
	}
	if s.predRed != nil && s.predRed(tip, cycle, s.calls[key]) {
		if s.noIDs {
			return acsrunner.Verdict{Cycle: cycle, RedCount: 1}, nil
		}
		if s.bareIncomplete {
			return acsrunner.Verdict{Cycle: cycle, IncompleteCount: 1}, nil
		}
		if s.incomplete {
			return acsrunner.Verdict{Cycle: cycle, IncompleteCount: 1, IncompleteIDs: []string{"acs/cycle-incomplete"}}, nil
		}
		return acsrunner.Verdict{Cycle: cycle, RedCount: 1, RedIDs: []string{"acs/cycle" + strings.TrimPrefix(CycleTrailer(cycle), "Evolve-Cycle: ")}}, nil
	}
	return acsrunner.Verdict{Cycle: cycle, GreenCount: 1}, nil
}

func (s *batchSeams) batchTest(f composeFixture) BatchTest {
	aff := s.affected
	if aff == nil {
		aff = func(_ string, edited []string) ([]string, error) { return edited, nil }
	}
	return BatchTest{
		Composer: Composer{Repo: f.repo, Stage: policy.FleetIntegrationEnforce, Gate: (&gateLog{}).gate},
		Affected: func(tip string, edited []string) ([]string, error) {
			s.mu.Lock()
			s.keys = append(s.keys, "affected "+s.where(tip))
			s.mu.Unlock()
			return aff(tip, edited)
		},
		Test:       s.test,
		Predicates: s.predicates,
		Verdicts:   s,
	}
}

func (s *batchSeams) where(tip string) string {
	if tip == "" {
		return "none"
	}
	var held []string
	for _, name := range []string{"a", "b", "c"} {
		if s.f.holdsFile(tip, goFile(name)) {
			held = append(held, strings.ToUpper(name))
		}
	}
	return strings.Join(held, "+")
}

func scoped(rec DeliverableRecord, scope ...string) DeliverableRecord {
	rec.Scope = scope
	return rec
}

func goFile(name string) string {
	return map[string]string{"a": "go/internal/alpha/a.go", "b": "go/internal/beta/b.go", "c": "go/internal/gamma/c.go"}[name]
}

func (f composeFixture) holdsFile(tip, file string) bool {
	return slices.Contains(strings.Fields(f.g.Git("ls-tree", "-r", "--name-only", tip)), file)
}

func twoDeliverables(t *testing.T, f composeFixture) []DeliverableRecord {
	t.Helper()
	return []DeliverableRecord{
		scoped(f.lane(t, 701, map[string]string{goFile("a"): "package alpha\n"}), "wave-integration-i3b-alpha"),
		scoped(f.lane(t, 702, map[string]string{goFile("b"): "package beta\n"}), "wave-integration-i3b-beta"),
	}
}

func TestBatchTest_TwoDeliverablesThatPassAloneFailTogetherAndTheLaterOneIsEjected(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	recs := twoDeliverables(t, f)
	s := newSeams(f)
	s.predRed = func(tip string, cycle, _ int) bool {
		return cycle == 702 && f.holdsFile(tip, goFile("a")) && f.holdsFile(tip, goFile("b"))
	}
	var got BatchResult
	got, err := s.batchTest(f).Check(context.Background(), f.batch(t), recs)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !slices.Contains(s.predRuns, "A+B Evolve-Cycle: 701") || !slices.Contains(s.predRuns, "A+B Evolve-Cycle: 702") {
		t.Errorf("predicates ran %v, want both sets on the combined tree A+B", s.predRuns)
	}
	if !slices.Equal(got.Landed, []int{701}) || len(got.Ejected) != 1 {
		t.Fatalf("Check = landed %v ejected %v, want [701] and one ejection", got.Landed, got.Ejected)
	}
	ej := got.Ejected[0]
	if ej.Cycle != 702 || ej.Cause != CauseBatch || !slices.Contains(ej.Paths, "acs/cycle702") {
		t.Errorf("ejection = %+v, want cycle 702 on the batch with the red predicate", ej)
	}
	v := got.Verdicts[702]
	if v.Pass || !slices.Contains(v.Red, "acs/cycle702") {
		t.Errorf("verdict of 702 = %+v, want a red verdict that names the predicate", v)
	}
	if !got.Verdicts[701].Pass {
		t.Errorf("verdict of 701 = %+v, want green", got.Verdicts[701])
	}
}

func TestBatchTest_AnEarlierRedDeliverableIsTheCulpritAndTheRestLand(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	recs := twoDeliverables(t, f)
	s := newSeams(f)
	s.predRed = func(tip string, cycle, _ int) bool { return cycle == 701 }
	got, err := s.batchTest(f).Check(context.Background(), f.batch(t), recs)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !slices.Equal(got.Landed, []int{702}) || len(got.Ejected) != 1 || got.Ejected[0].Cycle != 701 {
		t.Errorf("Check = landed %v ejected %+v, want 702 landed and 701 ejected", got.Landed, got.Ejected)
	}
}

func TestBatchTest_AllGreenLandsEveryDeliverableAndRecordsTheTestsThatRan(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	recs := twoDeliverables(t, f)
	s := newSeams(f)
	got, err := s.batchTest(f).Check(context.Background(), f.batch(t), recs)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !slices.Equal(got.Landed, []int{701, 702}) || len(got.Ejected) != 0 {
		t.Fatalf("Check = landed %v ejected %v, want both landed", got.Landed, got.Ejected)
	}
	if want := []string{"./internal/alpha/...", "./internal/beta/..."}; !slices.Equal(got.Verdicts[702].Ran, want) {
		t.Errorf("verdict of 702 ran %v, want %v", got.Verdicts[702].Ran, want)
	}
	if !slices.Contains(s.recorded, "./internal/alpha/...") {
		t.Errorf("a green package is not recorded in the verdict store: %v", s.recorded)
	}
}

func TestBatchTest_AFlakeIsRecordedAndEjectsNothing(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	recs := twoDeliverables(t, f)
	s := newSeams(f)
	s.testFail = func(_, pkg string, call int) error {
		if strings.Contains(pkg, "alpha") && call == 1 {
			return errors.New("TestFlaky failed")
		}
		return nil
	}
	s.predRed = func(_ string, cycle, call int) bool { return cycle == 702 && call == 1 }
	got, err := s.batchTest(f).Check(context.Background(), f.batch(t), recs)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !slices.Equal(got.Landed, []int{701, 702}) || len(got.Ejected) != 0 {
		t.Fatalf("Check = landed %v ejected %v, want a flake to eject nothing", got.Landed, got.Ejected)
	}
	if !slices.Contains(got.Flaky, "./internal/alpha/...") || !slices.Contains(got.Flaky, "acs/cycle702") {
		t.Errorf("flaky = %v, want the flaky package and the flaky predicate", got.Flaky)
	}
	if slices.Contains(s.recorded, "./internal/alpha/...") {
		t.Errorf("a flaky package must not be stored as green: %v", s.recorded)
	}
}

func TestBatchTest_ARedPredicateWithoutANameOrAnIncompleteOneIsStillRed(t *testing.T) {
	t.Parallel()
	for name, arm := range map[string]func(*batchSeams){
		"unnamed":                   func(s *batchSeams) { s.noIDs = true },
		"incomplete":                func(s *batchSeams) { s.incomplete = true },
		"incomplete without a name": func(s *batchSeams) { s.bareIncomplete = true },
	} {
		t.Run(name, func(t *testing.T) {
			f := newComposeFixture(t)
			s := newSeams(f)
			arm(s)
			s.predRed = func(_ string, cycle, _ int) bool { return cycle == 702 }
			got, err := s.batchTest(f).Check(context.Background(), f.batch(t), twoDeliverables(t, f))
			if err != nil || !slices.Equal(got.Landed, []int{701}) || got.Verdicts[702].Pass {
				t.Errorf("Check = %+v, %v; want 702 red", got, err)
			}
		})
	}
}

func TestBatchTest_APackageThatFailsTwiceIsRedAndEjects(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	recs := twoDeliverables(t, f)
	s := newSeams(f)
	s.testFail = func(_, pkg string, _ int) error {
		if strings.Contains(pkg, "beta") {
			return errors.New("TestBeta failed")
		}
		return nil
	}
	got, err := s.batchTest(f).Check(context.Background(), f.batch(t), recs)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !slices.Equal(got.Landed, []int{701}) || len(got.Ejected) != 1 || got.Ejected[0].Cycle != 702 {
		t.Errorf("Check = landed %v ejected %+v, want 702 ejected", got.Landed, got.Ejected)
	}
	if !slices.Contains(got.Verdicts[702].Red, "./internal/beta/...") {
		t.Errorf("red = %v, want the package", got.Verdicts[702].Red)
	}
}

func TestBatchTest_APackageWithAGreenVerdictOnTheSameKeyIsSkippedAndRecorded(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	recs := twoDeliverables(t, f)
	s := newSeams(f)
	s.green["./internal/alpha/..."] = true
	got, err := s.batchTest(f).Check(context.Background(), f.batch(t), recs)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	for _, run := range s.testRuns {
		if strings.Contains(run, "alpha") {
			t.Errorf("the skipped package ran: %q", run)
		}
	}
	if !slices.Contains(got.Verdicts[701].Skipped, "./internal/alpha/...") || slices.Contains(got.Verdicts[701].Ran, "./internal/alpha/...") {
		t.Errorf("verdict of 701 = %+v, want alpha skipped and not ran", got.Verdicts[701])
	}
	if slices.Contains(s.recorded, "./internal/alpha/...") {
		t.Errorf("a skipped package is stored again: %v", s.recorded)
	}
}

func TestBatchTest_AtShadowRunsNothing(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	recs := twoDeliverables(t, f)
	s := newSeams(f)
	bt := s.batchTest(f)
	bt.Composer.Stage = policy.FleetIntegrationShadow
	b := f.batch(t)
	got, err := bt.Check(context.Background(), b, recs)
	if err != nil || len(got.Landed) != 0 || len(got.Verdicts) != 0 {
		t.Errorf("Check = %+v, %v; want an empty result at shadow", got, err)
	}
	if len(s.testRuns)+len(s.predRuns) != 0 || f.g.Git("rev-parse", "refs/heads/"+b.Branch) != f.base {
		t.Errorf("shadow ran tests %v predicates %v or moved the branch", s.testRuns, s.predRuns)
	}
}

func TestBatchTest_ATextualConflictEjectsWithItsOwnCause(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	recs := []DeliverableRecord{
		f.lane(t, 711, map[string]string{sharedFile: "ONE\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n"}),
		f.lane(t, 712, map[string]string{sharedFile: "uno\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n"}),
	}
	s := newSeams(f)
	got, err := s.batchTest(f).Check(context.Background(), f.batch(t), recs)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !slices.Equal(got.Landed, []int{711}) || len(got.Ejected) != 1 || got.Ejected[0].Cause != CauseConflict {
		t.Errorf("Check = landed %v ejected %+v, want 712 ejected on a conflict", got.Landed, got.Ejected)
	}
}

func TestBatchTest_ARunnerErrorStopsTheCheck(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	cases := map[string]func(*batchSeams){
		"predicates": func(s *batchSeams) { s.predErr = boom },
		"predicates on the re-run": func(s *batchSeams) {
			s.predErr, s.predErrAt = boom, 2
			s.predRed = func(string, int, int) bool { return true }
		},
		"affected": func(s *batchSeams) {
			s.affected = func(string, []string) ([]string, error) { return nil, boom }
		},
		"record": func(s *batchSeams) { s.recordErr = boom },
	}
	for name, arm := range cases {
		t.Run(name, func(t *testing.T) {
			f := newComposeFixture(t)
			recs := twoDeliverables(t, f)
			s := newSeams(f)
			arm(s)
			if _, err := s.batchTest(f).Check(context.Background(), f.batch(t), recs); !errors.Is(err, boom) {
				t.Errorf("Check error = %v, want %v", err, boom)
			}
		})
	}
}

func TestBatchTest_RefusesAnUnreadyComposerAndAnEmptyBatch(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	s := newSeams(f)
	bt := s.batchTest(f)
	b := f.batch(t)
	if got, err := bt.Check(context.Background(), b, nil); err != nil || len(got.Landed) != 0 {
		t.Errorf("empty batch = %+v, %v; want an empty result", got, err)
	}
	bt.Composer.Gate = nil
	if _, err := bt.Check(context.Background(), b, twoDeliverables(t, f)); err == nil {
		t.Error("an enforcing composer without a gate must be refused")
	}
}

func TestEditedPackages_NamesTheGoPackageOfEachChangedPath(t *testing.T) {
	t.Parallel()
	got := editedPackages([]string{
		"go/internal/a/x.go", "go/internal/a/y.go", "go/internal/b/embed.tmpl", "go/go.mod", "go/go.sum",
		"go/cmd/evolve/main.go", "go/internal/c/testdata/golden.json", "go/testdata/x.json", "docs/readme.md", "go/x.go",
		"go/internal/d/testdata/fixture.go", "go/vendor/example.org/dep/d.go",
	})
	want := []string{"./...", "./cmd/evolve/...", "./internal/a/...", "./internal/b/...", "./internal/c/...", "./internal/d/..."}
	if !slices.Equal(got, want) {
		t.Errorf("editedPackages = %v, want %v", got, want)
	}
	for file, want := range map[string]string{
		"go/go.mod": "./...", "go/go.sum": "./...", "go/vendor/modules.txt": "./...",
		"go/internal/d/testdata/fixture.go": "./internal/d/...", "go/internal/d/testdata/x/y.json": "./internal/d/...",
	} {
		if got := editedPackages([]string{file}); !slices.Equal(got, []string{want}) {
			t.Errorf("editedPackages(%q) = %v, want [%s]", file, got, want)
		}
	}
	if got := editedPackages([]string{"docs/readme.md"}); len(got) != 0 {
		t.Errorf("editedPackages of a path outside go/ = %v, want none", got)
	}
}

func TestBatchTest_ATaskIDScopeStillTestsThePackagesTheDiffEdits(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	s := newSeams(f)
	rec := scoped(f.lane(t, 701, map[string]string{goFile("a"): "package alpha\n"}), "some-inbox-task-id")
	got, err := s.batchTest(f).Check(context.Background(), f.batch(t), []DeliverableRecord{rec})
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if want := []string{"./internal/alpha/..."}; !slices.Equal(got.Verdicts[701].Ran, want) {
		t.Errorf("ran %v, want %v although the scope holds a task id", got.Verdicts[701].Ran, want)
	}
}

func TestBatchTest_APackageRunnerFailureStopsTheCheckAndEjectsNobody(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	s := newSeams(f)
	s.testErr = context.Canceled
	got, err := s.batchTest(f).Check(context.Background(), f.batch(t), twoDeliverables(t, f))
	if !errors.Is(err, context.Canceled) || len(got.Ejected) != 0 || len(got.Landed) != 0 {
		t.Errorf("Check = %+v, %v; want the runner error and no ejection", got, err)
	}
}

func TestBatchTest_ARedPackageKeepsItsOutputInTheEjection(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	s := newSeams(f)
	s.testFail = func(_, pkg string, _ int) error {
		if strings.Contains(pkg, "beta") {
			return errors.New("--- FAIL: TestBeta")
		}
		return nil
	}
	got, err := s.batchTest(f).Check(context.Background(), f.batch(t), twoDeliverables(t, f))
	if err != nil || len(got.Ejected) != 1 {
		t.Fatalf("Check = %+v, %v", got, err)
	}
	if !strings.Contains(got.Ejected[0].Err.Error(), "--- FAIL: TestBeta") || !strings.Contains(got.Verdicts[702].Output, "TestBeta") {
		t.Errorf("ejection error %q and verdict output %q must hold the test output", got.Ejected[0].Err, got.Verdicts[702].Output)
	}
}

func TestBatchTest_ABatchEjectionHoldsTheDiffTheDeliverableMet(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	s := newSeams(f)
	s.predRed = func(_ string, cycle, _ int) bool { return cycle == 702 }
	got, err := s.batchTest(f).Check(context.Background(), f.batch(t), twoDeliverables(t, f))
	if err != nil || len(got.Ejected) != 1 {
		t.Fatalf("Check = %+v, %v", got, err)
	}
	if d := got.Ejected[0].Diff; !strings.Contains(d, goFile("a")) || strings.Contains(d, goFile("b")) {
		t.Errorf("diff = %q, want the change of 701 that 702 met and not the change of 702", d)
	}
}

func TestBatchTest_APredicateThatFailsWithAVerdictIsRedAndNotFatal(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	s := newSeams(f)
	s.predBoth = true
	got, err := s.batchTest(f).Check(context.Background(), f.batch(t), twoDeliverables(t, f))
	if err != nil {
		t.Fatalf("Check: %v, want the red verdict to eject", err)
	}
	if len(got.Landed) != 0 || len(got.Ejected) != 2 || !slices.Contains(got.Ejected[0].Paths, "egps/no-predicates") {
		t.Errorf("Check = landed %v ejected %+v, want both ejected on the red predicate", got.Landed, got.Ejected)
	}
}

func TestBatchTest_ACancelledContextWithAPredicateVerdictIsFatal(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	s := newSeams(f)
	s.predBoth = true
	ctx, cancel := context.WithCancel(context.Background())
	bt := s.batchTest(f)
	predicates := bt.Predicates
	bt.Predicates = func(c context.Context, tip string, cycle int) (acsrunner.Verdict, error) {
		cancel()
		return predicates(c, tip, cycle)
	}
	if _, err := bt.Check(ctx, f.batch(t), twoDeliverables(t, f)); err == nil {
		t.Error("a cancelled context must stop the check")
	}
}

func TestBatchTest_AnUnknownStageIsRefusedAndANilSeamIsRefused(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	s := newSeams(f)
	bt := s.batchTest(f)
	bt.Composer.Stage = "enforced-typo"
	b := f.batch(t)
	recs := twoDeliverables(t, f)
	if _, err := bt.Check(context.Background(), b, recs); err == nil {
		t.Error("an unknown stage must be refused")
	}
	for name, arm := range map[string]func(*BatchTest){
		"affected":   func(b *BatchTest) { b.Affected = nil },
		"test":       func(b *BatchTest) { b.Test = nil },
		"predicates": func(b *BatchTest) { b.Predicates = nil },
		"verdicts":   func(b *BatchTest) { b.Verdicts = nil },
	} {
		bad := s.batchTest(f)
		arm(&bad)
		if _, err := bad.Check(context.Background(), b, recs); err == nil {
			t.Errorf("a nil %s seam must be refused", name)
		}
	}
}

func TestBatchTest_L2NeverRunsTheGate(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	s := newSeams(f)
	gates := &gateLog{fail: func(string) error { return errors.New("gate red") }}
	bt := s.batchTest(f)
	bt.Composer.Gate = gates.gate
	got, err := bt.Check(context.Background(), f.batch(t), twoDeliverables(t, f))
	if err != nil || !slices.Equal(got.Landed, []int{701, 702}) || gates.calls() != 0 {
		t.Errorf("Check = %+v, %v with %d gate calls; want both landed and no gate", got, err, gates.calls())
	}
}

func TestBatchTest_L2NeverMovesTheBatchBranch(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	s := newSeams(f)
	b := f.batch(t)
	if _, err := s.batchTest(f).Check(context.Background(), b, twoDeliverables(t, f)); err != nil {
		t.Fatalf("Check: %v", err)
	}
	if got := f.g.Git("rev-parse", "refs/heads/"+b.Branch); got != f.base {
		t.Errorf("branch = %s, want the base %s", got, f.base)
	}
}

func TestBatchTest_AnIncompletePredicateNamesItselfInTheRedList(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	s := newSeams(f)
	s.incomplete = true
	s.predRed = func(_ string, cycle, _ int) bool { return cycle == 702 }
	got, _ := s.batchTest(f).Check(context.Background(), f.batch(t), twoDeliverables(t, f))
	if !slices.Contains(got.Verdicts[702].Red, "acs/cycle-incomplete") {
		t.Errorf("red = %v, want the incomplete predicate", got.Verdicts[702].Red)
	}
}

func TestBatchTest_AFlakeOnTwoPrefixesIsListedOnce(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	s := newSeams(f)
	s.testFail = func(_, pkg string, call int) error {
		if strings.Contains(pkg, "alpha") && call == 1 {
			return errors.New("flaky")
		}
		return nil
	}
	got, err := s.batchTest(f).Check(context.Background(), f.batch(t), twoDeliverables(t, f))
	if err != nil || !slices.Equal(got.Flaky, []string{"./internal/alpha/..."}) {
		t.Errorf("Check = flaky %v, %v; want one entry", got.Flaky, err)
	}
}

func TestBatchTest_TheRedListComesFromTheRerun(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	s := newSeams(f)
	bt := s.batchTest(f)
	bt.Predicates = func(_ context.Context, _ string, cycle int) (acsrunner.Verdict, error) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.calls["r"]++
		if cycle != 702 {
			return acsrunner.Verdict{Cycle: cycle, GreenCount: 1}, nil
		}
		if s.calls["r"]%2 == 1 {
			return acsrunner.Verdict{Cycle: cycle, RedCount: 2, RedIDs: []string{"first-only", "both"}}, nil
		}
		return acsrunner.Verdict{Cycle: cycle, RedCount: 1, RedIDs: []string{"both"}}, nil
	}
	got, _ := bt.Check(context.Background(), f.batch(t), twoDeliverables(t, f))
	if want := []string{"both"}; !slices.Equal(got.Verdicts[702].Red, want) {
		t.Errorf("red = %v, want %v", got.Verdicts[702].Red, want)
	}
	if want := []string{"first-only"}; !slices.Equal(got.Verdicts[702].Flaky, want) || !slices.Equal(got.Flaky, want) {
		t.Errorf("flaky = %v and %v, want %v: a predicate red on the first run only is a flake", got.Verdicts[702].Flaky, got.Flaky, want)
	}
}

func writeModule(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		path := filepath.Join(root, "go", name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestAffectedSet_IncludesEveryPackageThatImportsTheEditedOne(t *testing.T) {
	t.Parallel()
	root := writeModule(t, map[string]string{
		"go.mod":      "module example.com/m\n\ngo 1.21\n",
		"a/a.go":      "package a\n\nfunc A() int { return 1 }\n",
		"a/a_test.go": "package a\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) {}\n",
		"b/b.go":      "package b\n\nimport \"example.com/m/a\"\n\nfunc B() int { return a.A() }\n",
		"b/b_test.go": "package b\n\nimport \"testing\"\n\nfunc TestB(t *testing.T) {}\n",
		"c/c.go":      "package c\n\nimport \"example.com/m/b\"\n\nfunc C() int { return b.B() }\n",
		"c/c_test.go": "package c\n\nimport \"testing\"\n\nfunc TestC(t *testing.T) {}\n",
		"d/d.go":      "package d\n\nfunc D() int { return 4 }\n",
		"d/d_test.go": "package d\n\nimport \"testing\"\n\nfunc TestD(t *testing.T) {}\n",
	})
	got, err := AffectedSet(root, []string{"./a/..."})
	if err != nil {
		t.Fatalf("AffectedSet: %v", err)
	}
	want := []string{"./a/...", "./b/...", "./c/..."}
	if !slices.Equal(got, want) {
		t.Errorf("AffectedSet = %v, want %v with the direct and the indirect importer and not d", got, want)
	}
}

func TestAffectedSet_LeavesOutAPackageThatNoLongerExists(t *testing.T) {
	t.Parallel()
	root := writeModule(t, map[string]string{
		"go.mod":      "module example.com/m\n\ngo 1.21\n",
		"a/a.go":      "package a\n\nfunc A() int { return 1 }\n",
		"a/a_test.go": "package a\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) {}\n",
	})
	got, err := AffectedSet(root, []string{"./a/...", "./gone/..."})
	if err != nil || !slices.Equal(got, []string{"./a/..."}) {
		t.Errorf("AffectedSet = %v, %v; want only ./a/... because go test on a removed package exits 1", got, err)
	}
}

func TestAffectedSet_FailsLoudWhenTheModuleCannotBeListed(t *testing.T) {
	t.Parallel()
	if _, err := AffectedSet(t.TempDir(), []string{"./a/..."}); err == nil {
		t.Error("AffectedSet must fail when it cannot list the module")
	}
	if got, err := AffectedSet(t.TempDir(), nil); err != nil || len(got) != 0 {
		t.Errorf("AffectedSet of no edit = %v, %v; want empty", got, err)
	}
}

func TestBatchTest_AGitFailureOfTheComposeStopsTheCheck(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	recs := twoDeliverables(t, f)
	s := newSeams(f)
	bt := s.batchTest(f)
	bt.Composer.Repo = Repo{Dir: f.g.Dir, Git: failOn("merge-tree")}
	_, err := bt.Check(context.Background(), f.batch(t), recs)
	var ej *Ejection
	if err == nil || errors.As(err, &ej) {
		t.Errorf("Check = %v, want a plain error", err)
	}
	if len(s.testRuns)+len(s.predRuns) != 0 {
		t.Errorf("tests ran after a git failure: %v %v", s.testRuns, s.predRuns)
	}
}

func TestBatchTest_AGitFailureAfterTheComposeStopsTheCheck(t *testing.T) {
	t.Parallel()
	sys := SysGit()
	cases := map[string]func(calls *int) GitFunc{
		"the edited paths": func(calls *int) GitFunc {
			return func(ctx context.Context, dir, stdin string, args ...string) (string, error) {
				if args[0] == "diff-tree" {
					*calls++
					if *calls > 2 {
						return "", &GitError{Args: args, Code: 128, Err: errors.New("diff-tree broke")}
					}
				}
				return sys(ctx, dir, stdin, args...)
			}
		},
		"the ejection diff": func(*int) GitFunc { return failOn("diff") },
	}
	for name, mk := range cases {
		t.Run(name, func(t *testing.T) {
			f := newComposeFixture(t)
			s := newSeams(f)
			s.predRed = func(string, int, int) bool { return true }
			bt := s.batchTest(f)
			calls := 0
			bt.Composer.Repo = Repo{Dir: f.g.Dir, Git: mk(&calls)}
			recs := []DeliverableRecord{f.lane(t, 701, map[string]string{goFile("a"): "package alpha\n"})}
			_, err := bt.Check(context.Background(), f.batch(t), recs)
			var ej *Ejection
			if err == nil || errors.As(err, &ej) {
				t.Errorf("Check = %v, want a plain error", err)
			}
		})
	}
}

func TestBatchTest_ARunnerFailureOnTheRerunStopsTheCheck(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	s := newSeams(f)
	s.testErr, s.testErrAt = errors.New("checkout broke"), 2
	s.testFail = func(string, string, int) error { return errors.New("red") }
	if _, err := s.batchTest(f).Check(context.Background(), f.batch(t), twoDeliverables(t, f)); err == nil {
		t.Error("a runner error on the re-run must stop the check")
	}
}

func TestBatchTest_TheSeamsKeyTheirVerdictsOnTheTipUnderTest(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	s := newSeams(f)
	if _, err := s.batchTest(f).Check(context.Background(), f.batch(t), twoDeliverables(t, f)); err != nil {
		t.Fatalf("Check: %v", err)
	}
	for _, want := range []string{
		"affected A", "affected A+B", "green A ./internal/alpha/...", "green A+B ./internal/beta/...",
		"record A ./internal/alpha/...", "record A+B ./internal/beta/...",
	} {
		if !slices.Contains(s.keys, want) {
			t.Errorf("seam calls %v lack %q: a verdict must be keyed on the tip that it tested", s.keys, want)
		}
	}
	if slices.Contains(s.keys, "green none ./internal/alpha/...") || slices.Contains(s.keys, "record none ./internal/alpha/...") {
		t.Errorf("a seam got an empty tip: %v", s.keys)
	}
}

func TestBatchTest_ARedPackageKeepsTheOutputOfTheRerunOnly(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	s := newSeams(f)
	s.testFail = func(_, pkg string, call int) error {
		if strings.Contains(pkg, "beta") {
			return errors.New("run " + string(rune('0'+call)))
		}
		return nil
	}
	got, err := s.batchTest(f).Check(context.Background(), f.batch(t), twoDeliverables(t, f))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if out := got.Verdicts[702].Output; !strings.Contains(out, "run 2") || strings.Contains(out, "run 1") {
		t.Errorf("output = %q, want the re-run output only", out)
	}
}

func TestBatchTest_TheLandedTipIsTheTipThatWasTested(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	s := newSeams(f)
	s.predRed = func(_ string, cycle, _ int) bool { return cycle == 702 }
	b := f.batch(t)
	recs := twoDeliverables(t, f)
	got, err := s.batchTest(f).Check(context.Background(), b, recs)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if got.Tip == b.Base || !f.holdsFile(got.Tip, goFile("a")) || f.holdsFile(got.Tip, goFile("b")) {
		t.Errorf("tip %s must hold the landed A and not the ejected B", got.Tip)
	}
	s2 := newSeams(f)
	s2.predRed = func(string, int, int) bool { return true }
	none, err := s2.batchTest(f).Check(context.Background(), b, recs)
	if err != nil || none.Tip != b.Base {
		t.Errorf("with nothing landed the tip is %q, %v; want the base %s", none.Tip, err, b.Base)
	}
}

func TestBatchTest_AfterTheL1PrefixCheckTheSearchStillStartsFromTheBase(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	recs := twoDeliverables(t, f)
	s := newSeams(f)
	s.predRed = func(tip string, cycle, _ int) bool {
		return cycle == 701 && f.holdsFile(tip, goFile("b"))
	}
	bt := s.batchTest(f)
	l1, err := bt.Composer.PrefixCheck(context.Background(), f.batch(t), recs)
	if err != nil || !slices.Equal(l1.Landed, []int{701, 702}) || l1.Batch.Tip == l1.Batch.Base {
		t.Fatalf("PrefixCheck = %+v, %v; want both landed on a moved tip", l1, err)
	}
	got, err := bt.Check(context.Background(), l1.Batch, recs)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !slices.Equal(got.Landed, []int{701}) || len(got.Ejected) != 1 || got.Ejected[0].Cycle != 702 {
		t.Fatalf("Check = landed %v ejected %+v, want 701 landed and the real culprit 702 ejected", got.Landed, got.Ejected)
	}
	if d := got.Ejected[0].Diff; !strings.Contains(d, goFile("a")) || strings.Contains(d, goFile("b")) {
		t.Errorf("diff = %q, want only the change of 701", d)
	}
	if want := []string{"./internal/alpha/..."}; !slices.Equal(got.Verdicts[701].Ran, want) {
		t.Errorf("701 ran %v, want %v: its prefix must not hold 702", got.Verdicts[701].Ran, want)
	}
}

func TestBatchTest_ADeliverableThatEditsOnlyAGoFixtureOrAVendoredFileStillTestsAPackage(t *testing.T) {
	t.Parallel()
	for name, file := range map[string]string{
		"testdata fixture": "go/internal/alpha/testdata/fixture.go",
		"vendored source":  "go/vendor/example.org/dep/d.go",
	} {
		t.Run(name, func(t *testing.T) {
			f := newComposeFixture(t)
			s := newSeams(f)
			rec := f.lane(t, 701, map[string]string{file: "package x\n"})
			got, err := s.batchTest(f).Check(context.Background(), f.batch(t), []DeliverableRecord{rec})
			if err != nil || len(got.Verdicts[701].Ran) == 0 {
				t.Errorf("Check = %+v, %v; want a package test for an edit of %s", got.Verdicts[701], err, file)
			}
		})
	}
}
