package integration

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/acsrunner"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type openObserver struct {
	*fakeCI
	seen func()
}

func (o openObserver) Open(ctx context.Context, b Batch) (PR, error) {
	o.seen()
	return o.fakeCI.Open(ctx, b)
}

func l2Rig(t *testing.T) (*trainRig, *batchSeams) {
	t.Helper()
	r := newTrainRig(t)
	s := newSeams(r.f)
	r.train.L2 = s.batchTest(r.f)
	return r, s
}

func (r *trainRig) landAlphaAndBeta(t *testing.T) (Outcome, error) {
	t.Helper()
	r.pr(t, 1, "feat: alpha", map[string]string{goFile("a"): "package alpha\n"})
	r.pr(t, 2, "feat: beta", map[string]string{goFile("b"): "package beta\n"})
	plan, err := r.train.Plan(context.Background(), []int{1, 2})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	return r.train.Land(context.Background(), plan)
}

func (s *batchSeams) testRunCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.testRuns)
}

func TestTrainL2_TheBatchTestRunsBeforeTheBatchPROpensAndTheTestedTipLands(t *testing.T) {
	t.Parallel()
	r, s := l2Rig(t)
	testsBeforeOpen := -1
	r.train.CI = openObserver{fakeCI: r.ci, seen: func() { testsBeforeOpen = s.testRunCount() }}
	out, err := r.landAlphaAndBeta(t)
	if err != nil {
		t.Fatalf("Land: %v", err)
	}
	if !slices.Equal(out.Landed, []int{1, 2}) || len(out.Ejected) != 0 {
		t.Fatalf("Outcome = landed %v ejected %v, want both landed", out.Landed, out.Ejected)
	}
	if testsBeforeOpen <= 0 {
		t.Fatalf("the batch test had run %d package tests when the batch PR opened, want the L2 run first", testsBeforeOpen)
	}
	if len(r.ci.opened) != 1 {
		t.Fatalf("opened %d batch PRs, want 1", len(r.ci.opened))
	}
	if !slices.ContainsFunc(s.testRuns, func(run string) bool { return strings.HasPrefix(run, "A+B ") }) {
		t.Errorf("package tests ran %v, want a run on the combined tree A+B", s.testRuns)
	}
	if got := r.originMain(); got != r.ci.opened[0].Tip {
		t.Errorf("origin main = %s, want the tip of the batch PR %s", got, r.ci.opened[0].Tip)
	}
}

func TestTrainL2_AShadowStageRunsNoSeamAndLandsAsBefore(t *testing.T) {
	t.Parallel()
	r, s := l2Rig(t)
	r.train.L2.Composer.Stage = policy.FleetIntegrationShadow
	out, err := r.landAlphaAndBeta(t)
	if err != nil {
		t.Fatalf("Land: %v", err)
	}
	if !slices.Equal(out.Landed, []int{1, 2}) || len(out.Ejected) != 0 {
		t.Fatalf("Outcome = landed %v ejected %v, want both landed", out.Landed, out.Ejected)
	}
	if n := s.testRunCount(); n != 0 || len(s.predRuns) != 0 {
		t.Errorf("a shadow stage ran %d package tests and %d predicate runs, want none", n, len(s.predRuns))
	}
}

func TestTrainL2_TwoPRsThatPassAloneAndFailTogetherEjectTheLaterOneWithTheCauseInItsBrief(t *testing.T) {
	t.Parallel()
	r, s := l2Rig(t)
	s.testFail = func(tip, pkg string, _ int) error {
		if strings.Contains(pkg, "beta") && r.f.holdsFile(tip, goFile("a")) && r.f.holdsFile(tip, goFile("b")) {
			return errors.New("--- FAIL: TestBetaMeetsAlpha")
		}
		return nil
	}
	out, err := r.landAlphaAndBeta(t)
	if err != nil {
		t.Fatalf("Land: %v", err)
	}
	if !slices.Equal(out.Landed, []int{1}) || !slices.Equal(out.Ejected, []int{2}) {
		t.Fatalf("Outcome = landed %v ejected %v, want PR 1 landed and PR 2 ejected", out.Landed, out.Ejected)
	}
	brief := strings.Join(r.host.comments[2], "\n")
	for _, want := range []string{"ejected PR #2 on a batch", "--- FAIL: TestBetaMeetsAlpha", "./internal/beta/..."} {
		if !strings.Contains(brief, want) {
			t.Errorf("the brief of PR 2 lacks %q:\n%s", want, brief)
		}
	}
	if len(r.host.comments[1]) != 0 {
		t.Errorf("PR 1 passed and got a comment: %v", r.host.comments[1])
	}
	if _, closed := r.host.closed[2]; closed {
		t.Error("an ejected PR stays open")
	}
	if len(r.ci.opened) != 1 {
		t.Fatalf("opened %d batch PRs, want 1", len(r.ci.opened))
	}
	opened := r.ci.opened[0]
	if !r.f.holdsFile(opened.Tip, goFile("a")) || r.f.holdsFile(opened.Tip, goFile("b")) {
		t.Errorf("the batch PR tip %s must hold PR 1 and not PR 2", opened.Tip)
	}
	if branch := strings.TrimSpace(r.f.g.Git("rev-parse", branchRef(opened.Branch))); branch != opened.Tip {
		t.Errorf("branch %s is at %s, want the tested tip %s", opened.Branch, branch, opened.Tip)
	}
	if got := r.originMain(); got != opened.Tip {
		t.Errorf("origin main = %s, want the tested tip %s", got, opened.Tip)
	}
	if _, ok := FindPRRecord(r.root, 2); !ok {
		t.Error("the record of the ejected PR 2 must stay")
	}
}

func TestTrainL2_AHostKillOfAPackageTestEjectsNoMemberAndOpensNoBatchPR(t *testing.T) {
	t.Parallel()
	r, s := l2Rig(t)
	s.testErr = acsrunner.HostFailure(errors.New("signal: killed"))
	before := r.originMain()
	out, err := r.landAlphaAndBeta(t)
	if err == nil || !acsrunner.IsHostFailure(err) {
		t.Fatalf("Land = %v, want the host failure", err)
	}
	if len(out.Ejected) != 0 || len(out.Landed) != 0 {
		t.Errorf("Outcome = landed %v ejected %v, want none", out.Landed, out.Ejected)
	}
	if len(r.host.comments) != 0 || len(r.host.closed) != 0 {
		t.Errorf("a host kill touched PRs: comments %v closed %v", r.host.comments, r.host.closed)
	}
	if len(r.ci.opened) != 0 || r.originMain() != before {
		t.Errorf("a host kill opened %d batch PRs and moved main to %s", len(r.ci.opened), r.originMain())
	}
	for _, pr := range []int{1, 2} {
		if _, ok := FindPRRecord(r.root, pr); !ok {
			t.Errorf("the record of PR %d must stay after a host kill", pr)
		}
	}
}

func syntheticRedWith(err error) PredicateRun {
	return func(context.Context, string, int) (acsrunner.Verdict, error) {
		return acsrunner.Verdict{RedCount: 1, RedIDs: []string{"egps/no-predicates"}}, err
	}
}

func TestBatchTest_AHostKillOfThePredicateRunEjectsNobodyEvenWithItsSyntheticRed(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	bt := newSeams(f).batchTest(f)
	bt.Predicates = syntheticRedWith(acsrunner.HostFailure(errors.New("signal: killed")))
	got, err := bt.Check(context.Background(), f.batch(t), twoDeliverables(t, f))
	if err == nil || !acsrunner.IsHostFailure(err) {
		t.Fatalf("Check = %+v, %v, want the host failure", got, err)
	}
	if len(got.Ejected) != 0 {
		t.Errorf("a host kill ejected %v", got.Ejected)
	}
}

func TestBatchTest_ABuildFailureOfThePredicatePackageStillEjects(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	bt := newSeams(f).batchTest(f)
	bt.Predicates = syntheticRedWith(errors.New("acsrunner go test: exit status 1"))
	got, err := bt.Check(context.Background(), f.batch(t), twoDeliverables(t, f))
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(got.Ejected) == 0 || got.Ejected[0].Cause != CauseBatch {
		t.Errorf("Check ejected %v, want the deliverable whose predicate package did not build", got.Ejected)
	}
}

var l2PackNames = []string{"repo-contract", "cover-changed", "cover-strict", "build", "vet", "gofmt"}

func packNames(steps []PackStep) []string {
	names := make([]string, len(steps))
	for i, s := range steps {
		names[i] = s.Name
	}
	return names
}

func gitCheckout(repo Repo) Checkout {
	return func(ctx context.Context, tip string) (string, func(), error) {
		dir, err := os.MkdirTemp("", "l2-checkout-")
		if err != nil {
			return "", nil, err
		}
		if _, err := repo.Git(ctx, repo.Dir, "", "worktree", "add", "--detach", dir, tip); err != nil {
			return "", nil, err
		}
		release := func() {
			_, _ = repo.Git(context.Background(), repo.Dir, "", "worktree", "remove", "--force", dir)
			_ = os.RemoveAll(dir)
		}
		return dir, release, nil
	}
}

type packRun struct {
	stubbed int
	cannot  error
}

func (p *packRun) run(ctx context.Context, dir string, argv ...string) (string, int, error) {
	if p.cannot != nil {
		return "", 0, p.cannot
	}
	isGo := argv[0] == "go" && len(argv) > 1 && (argv[1] == "build" || argv[1] == "vet")
	if argv[0] != "gofmt" && !isGo {
		p.stubbed++
		return "", 0, nil
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return string(out), exit.ExitCode(), nil
	}
	return string(out), 0, err
}

const (
	fixtureModule = "module fixture\n\ngo 1.23\n"
	cleanBeta     = "package beta\n\nfunc Beta() int {\n\treturn 1\n}\n"
	uglyBeta      = "package beta\n\nfunc  Beta( ) int {\nreturn 1\n}\n"
)

func packRig(t *testing.T, beta string) (*trainRig, *packRun) {
	t.Helper()
	r, _ := l2Rig(t)
	run := &packRun{}
	r.train.L2.Pack = L2Pack(gitCheckout(r.f.repo), run.run)
	r.pr(t, 1, "feat: module and alpha", map[string]string{"go/go.mod": fixtureModule, goFile("a"): "package alpha\n"})
	r.pr(t, 2, "feat: beta", map[string]string{goFile("b"): beta})
	return r, run
}

func TestL2Pack_NamesTheSixChecksOfTheL2RowInOrder(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	if got := packNames(L2Pack(gitCheckout(f.repo), (&packRun{}).run)); !slices.Equal(got, l2PackNames) {
		t.Fatalf("L2Pack = %v, want %v", got, l2PackNames)
	}
}

func TestTrainL2_ATreeThatFailsOnlyGofmtOnTheCombinedTreeEjectsTheMemberBeforeTheBatchPR(t *testing.T) {
	t.Parallel()
	r, run := packRig(t, uglyBeta)
	plan, err := r.train.Plan(context.Background(), []int{1, 2})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	out, err := r.train.Land(context.Background(), plan)
	if err != nil {
		t.Fatalf("Land: %v", err)
	}
	if !slices.Equal(out.Landed, []int{1}) || !slices.Equal(out.Ejected, []int{2}) {
		t.Fatalf("Outcome = landed %v ejected %v, want PR 1 landed and the unformatted PR 2 ejected", out.Landed, out.Ejected)
	}
	brief := strings.Join(r.host.comments[2], "\n")
	for _, want := range []string{"ejected PR #2 on a batch", "red on [gofmt]", "b.go"} {
		if !strings.Contains(brief, want) {
			t.Errorf("the brief of PR 2 lacks %q:\n%s", want, brief)
		}
	}
	if run.stubbed < 3 {
		t.Errorf("the repo-contract pack, cover-changed and cover-strict ran %d times, want at least once each", run.stubbed)
	}
	if len(r.ci.opened) != 1 || r.f.holdsFile(r.ci.opened[0].Tip, goFile("b")) {
		t.Errorf("batch PRs opened = %d, want one whose tip lacks the unformatted file", len(r.ci.opened))
	}
}

func TestTrainL2_AFormattedMemberPassesTheWholePackAndLands(t *testing.T) {
	t.Parallel()
	r, _ := packRig(t, cleanBeta)
	plan, err := r.train.Plan(context.Background(), []int{1, 2})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	out, err := r.train.Land(context.Background(), plan)
	if err != nil {
		t.Fatalf("Land: %v", err)
	}
	if !slices.Equal(out.Landed, []int{1, 2}) || len(out.Ejected) != 0 {
		t.Errorf("Outcome = landed %v ejected %v, want both landed", out.Landed, out.Ejected)
	}
}

func TestBatchTest_APackStepThatCannotRunIsAHostFailureAndEjectsNobody(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	bt := newSeams(f).batchTest(f)
	run := &packRun{cannot: errors.New("fork/exec: resource temporarily unavailable")}
	bt.Pack = L2Pack(gitCheckout(f.repo), run.run)
	got, err := bt.Check(context.Background(), f.batch(t), twoDeliverables(t, f))
	if err == nil || len(got.Ejected) != 0 {
		t.Fatalf("Check = %+v, %v, want an error and no ejection", got, err)
	}
}

func TestBatchTest_AnEmptyPackKeepsTheLayerAsItWas(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	bt := newSeams(f).batchTest(f)
	got, err := bt.Check(context.Background(), f.batch(t), twoDeliverables(t, f))
	if err != nil || !slices.Equal(got.Landed, []int{701, 702}) {
		t.Fatalf("Check = %+v, %v, want both landed with no pack", got, err)
	}
}

func TestL2Pack_AFailedStepIsRedByItsNameAndKeepsItsOutput(t *testing.T) {
	t.Parallel()
	f := newComposeFixture(t)
	bt := newSeams(f).batchTest(f)
	failing := func(_ context.Context, _ string, argv ...string) (string, int, error) {
		if argv[0] == "gofmt" {
			return "go/internal/beta/b.go\n", 0, nil
		}
		return "", 0, nil
	}
	bt.Pack = L2Pack(func(context.Context, string) (string, func(), error) {
		return filepath.Join(t.TempDir(), "tree"), func() {}, nil
	}, failing)
	got, err := bt.Check(context.Background(), f.batch(t), twoDeliverables(t, f))
	if err != nil || len(got.Ejected) == 0 {
		t.Fatalf("Check = %+v, %v, want the gofmt red to eject", got, err)
	}
	if !slices.Equal(got.Ejected[0].Paths, []string{"gofmt"}) || !strings.Contains(got.Ejected[0].Err.Error(), "go/internal/beta/b.go") {
		t.Errorf("ejection = %+v, want the step name as its path and the gofmt listing in its error", got.Ejected[0])
	}
}
