package ciwatch

import (
	"context"
	"errors"
	"os"
	"slices"
	"testing"
	"time"
)

func writeFileF1(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestClassifyBackfillF1_FallsBackToUnknownWhenNoRuleHolds(t *testing.T) {
	orig := classifyRules
	t.Cleanup(func() { classifyRules = orig })
	classifyRules = []classifyRule{{"never", LabelReal, func(Evidence) bool { return false }}}
	label, rule := classify(Evidence{Touched: FactYes, BaseRed: FactYes})
	if label != LabelUnknown || rule != "default" {
		t.Fatalf("classify with no matching rule = %s/%s, want unknown/default", label, rule)
	}
}

type failingSourceF1 struct {
	*fakeSource
	latestErr, mainErr, rerunErr error
}

func (f failingSourceF1) LatestRunOn(ctx context.Context, sha string) (RunStatus, error) {
	if f.latestErr != nil {
		return RunStatus{}, f.latestErr
	}
	return f.fakeSource.LatestRunOn(ctx, sha)
}

func (f failingSourceF1) MainRunsBefore(ctx context.Context, before time.Time, limit int) ([]RunStatus, error) {
	if f.mainErr != nil {
		return nil, f.mainErr
	}
	return f.fakeSource.MainRunsBefore(ctx, before, limit)
}

func (f failingSourceF1) RerunFailed(ctx context.Context, id int64, attempt int, timeout, poll time.Duration) (RunStatus, error) {
	if f.rerunErr != nil {
		return RunStatus{}, f.rerunErr
	}
	return f.fakeSource.RerunFailed(ctx, id, attempt, timeout, poll)
}

func TestClassifyTargetBackfillF1_EachEvidenceSourceFailureAbortsWithItsOwnError(t *testing.T) {
	at := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	run := failed(7, "h", at, FailingTest{Package: "m/a", Test: "TestA", Jobs: []string{"j"}})
	run.CreatedAt = at
	change := Change{HeadSHA: "h", BaseSHA: "b", Run: run}
	opts := ClassifyOptions{Rerun: true, MainWindow: 5, ModulePath: "m", ModuleDir: "go"}
	cases := []struct {
		name string
		src  failingSourceF1
	}{
		{"base run lookup", failingSourceF1{latestErr: errors.New("base 502")}},
		{"main window lookup", failingSourceF1{mainErr: errors.New("main 502")}},
		{"rerun", failingSourceF1{rerunErr: errors.New("rerun 502")}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var wantMsg string
			switch {
			case tc.src.latestErr != nil:
				wantMsg = tc.src.latestErr.Error()
			case tc.src.mainErr != nil:
				wantMsg = tc.src.mainErr.Error()
			default:
				wantMsg = tc.src.rerunErr.Error()
			}
			tc.src.fakeSource = &fakeSource{change: change}
			rep, err := ClassifyTarget(context.Background(), tc.src, Target{TargetRun, "7"}, opts)
			if err == nil || err.Error() != wantMsg {
				t.Fatalf("err = %v, want %q", err, wantMsg)
			}
			if rep.RunID != 7 || rep.Failures == nil || len(rep.Failures) != 0 || rep.RetrySafe {
				t.Fatalf("the partial report keeps the run identity and no verdicts: %+v", rep)
			}
		})
	}
}

func TestGatherFactsBackfillF1_SkipsEverySourceItHasNoGroundsToAsk(t *testing.T) {
	src := failingSourceF1{fakeSource: &fakeSource{}, latestErr: errors.New("must not be asked"), mainErr: errors.New("must not be asked"), rerunErr: errors.New("must not be asked")}
	facts, err := gatherFacts(context.Background(), src, Change{Run: RunStatus{RunID: 1}}, ClassifyOptions{MainWindow: 5})
	if err != nil {
		t.Fatalf("no base SHA, no run time and no rerun request means no source call: %v", err)
	}
	for name, f := range map[string]func(string) Fact{"base": facts.base, "main": facts.main, "rerun": facts.rerun} {
		if got := f("any"); got != FactUnknown {
			t.Errorf("%s fact = %s, want unknown", name, got)
		}
	}
}

func TestRecurredOnBackfillF1(t *testing.T) {
	a := FailingTest{Package: "m/a", Test: "TestA"}
	b := FailingTest{Package: "m/b", Test: "TestB"}
	runA := RunStatus{Status: StatusCompleted, Conclusion: "failure", FailingTests: []FailingTest{a}}
	runB := RunStatus{Status: StatusCompleted, Conclusion: "failure", FailingTests: []FailingTest{b}}
	green := RunStatus{Status: StatusCompleted, Conclusion: ConclusionSuccess, FailingTests: []FailingTest{a}}

	if got := recurredOn(nil)(failureKey(a)); got != FactUnknown {
		t.Errorf("no eligible runs = %s, want unknown", got)
	}
	both := recurredOn([]RunStatus{runA, runB})
	if both(failureKey(a)) != FactYes || both(failureKey(b)) != FactYes {
		t.Error("a failure seen on any eligible main run recurred")
	}
	if got := both(failureKey(FailingTest{Package: "m/c", Test: "TestC"})); got != FactNo {
		t.Errorf("a failure no main run showed = %s, want no", got)
	}
	if got := recurredOn([]RunStatus{green})(failureKey(a)); got != FactNo {
		t.Errorf("a green main run carries no failures even if its list is populated: got %s", got)
	}
}

func TestGreenOnRerunBackfillF1(t *testing.T) {
	a := FailingTest{Package: "m/a", Test: "TestA"}
	other := FailingTest{Package: "m/z", Test: "TestZ"}
	cases := []struct {
		name    string
		newer   RunStatus
		attempt int
		key     FailingTest
		want    Fact
	}{
		{"same attempt is not a rerun", RunStatus{Attempt: 1, Status: StatusCompleted, Conclusion: ConclusionSuccess}, 1, a, FactUnknown},
		{"older attempt is not a rerun", RunStatus{Attempt: 1, Status: StatusCompleted, Conclusion: ConclusionSuccess}, 2, a, FactUnknown},
		{"unfinished rerun proves nothing", RunStatus{Attempt: 2, Status: "in_progress"}, 1, a, FactUnknown},
		{"green rerun clears every key", RunStatus{Attempt: 2, Status: StatusCompleted, Conclusion: ConclusionSuccess}, 1, a, FactYes},
		{"red rerun still failing the key", RunStatus{Attempt: 2, Status: StatusCompleted, Conclusion: "failure", FailingTests: []FailingTest{a}}, 1, a, FactNo},
		{"red rerun failing something else says nothing about this key", RunStatus{Attempt: 2, Status: StatusCompleted, Conclusion: "failure", FailingTests: []FailingTest{a}}, 1, other, FactUnknown},
	}
	for _, tc := range cases {
		if got := greenOnRerun(tc.newer, tc.attempt)(failureKey(tc.key)); got != tc.want {
			t.Errorf("%s: %s, want %s", tc.name, got, tc.want)
		}
	}
}

func TestFailuresOfBackfillF1(t *testing.T) {
	listed := []FailingTest{{Package: "m/a", Test: "TestA", Jobs: []string{"j"}}}
	if got := failuresOf(RunStatus{FailingTests: listed, FailingJobs: []string{"ignored"}}); !slices.EqualFunc(got, listed, func(x, y FailingTest) bool { return x.Test == y.Test }) {
		t.Errorf("listed tests win over jobs: %+v", got)
	}
	jobs := []string{"build", "lint"}
	got := failuresOf(RunStatus{FailingJobs: jobs})
	if len(got) != 1 || !slices.Equal(got[0].Jobs, jobs) {
		t.Fatalf("a run with only failing jobs reports one job-level failure: %+v", got)
	}
	got[0].Jobs[0] = "mutated"
	if jobs[0] != "build" {
		t.Error("the job list must be copied, not aliased to the run")
	}
	empty := failuresOf(RunStatus{})
	if len(empty) != 1 || empty[0].Jobs == nil || len(empty[0].Jobs) != 0 {
		t.Fatalf("a red run naming nothing keeps one failure with an empty, non-nil job list: %#v", empty)
	}
}

func TestRetrySafeBackfillF1(t *testing.T) {
	mk := func(labels ...Label) []ClassifiedFailure {
		out := make([]ClassifiedFailure, len(labels))
		for i, l := range labels {
			out[i] = ClassifiedFailure{Label: l}
		}
		return out
	}
	cases := []struct {
		name string
		in   []ClassifiedFailure
		want bool
	}{
		{"no failures means nothing proves a retry is safe", nil, false},
		{"empty slice", mk(), false},
		{"all pre-existing", mk(LabelPreExisting), true},
		{"pre-existing and flake", mk(LabelPreExisting, LabelFlakeEvidence), true},
		{"one real failure blocks the retry", mk(LabelFlakeEvidence, LabelReal), false},
		{"one unknown failure blocks the retry", mk(LabelPreExisting, LabelUnknown), false},
	}
	for _, tc := range cases {
		if got := retrySafe(tc.in); got != tc.want {
			t.Errorf("%s: retrySafe = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestPackageDirBackfillF1(t *testing.T) {
	opts := ClassifyOptions{ModulePath: "github.com/o/r/go", ModuleDir: "go"}
	cases := []struct {
		name      string
		pkg       string
		opts      ClassifyOptions
		want      string
		wantKnown bool
	}{
		{"the module root package", "github.com/o/r/go", opts, "go", true},
		{"a nested package", "github.com/o/r/go/internal/a", opts, "go/internal/a", true},
		{"a package of another module", "github.com/o/r/other/x", opts, "", false},
		{"a sibling sharing the module prefix", "github.com/o/r/gopher", opts, "", false},
		{"empty package", "", opts, "", false},
		{"no module path configured", "github.com/o/r/go/x", ClassifyOptions{}, "", false},
	}
	for _, tc := range cases {
		got, known := packageDir(tc.pkg, tc.opts)
		if got != tc.want || known != tc.wantKnown {
			t.Errorf("%s: packageDir = (%q, %v), want (%q, %v)", tc.name, got, known, tc.want, tc.wantKnown)
		}
	}
}

func TestClassifyTargetBackfillF1_AModuleRootPackageIsTouchedByAnyFileUnderTheModuleDir(t *testing.T) {
	at := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	run := failed(9, "h", at, FailingTest{Package: "m", Test: "TestRoot", Jobs: []string{"j"}})
	src := &fakeSource{change: Change{HeadSHA: "h", Files: []string{"go/main.go"}, Run: run}}
	rep, err := ClassifyTarget(context.Background(), src, Target{TargetRun, "9"}, ClassifyOptions{ModulePath: "m", ModuleDir: "go"})
	if err != nil || len(rep.Failures) != 1 || rep.Failures[0].Evidence.Touched != FactYes || rep.Failures[0].Label != LabelReal {
		t.Fatalf("report = %+v, err = %v; want the root-package failure labelled real", rep, err)
	}
}
