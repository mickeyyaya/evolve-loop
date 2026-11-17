package integration

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
)

type mainCIStub struct {
	verdict Verdict
	err     error
	asked   []string
}

func (s *mainCIStub) Watch(_ context.Context, pr PR) (Verdict, error) {
	s.asked = append(s.asked, pr.Tip)
	return s.verdict, s.err
}

func TestGhCITypesARunThatIsMissingOrEndedWithoutAVerdict(t *testing.T) {
	t.Parallel()
	s := &ghScript{reply: func([]string) (string, int, error) { return recordedRunListEmpty, 0, nil }}
	if _, err := newGhCI(t, s).Watch(context.Background(), PR{Tip: sha('a')}); !errors.Is(err, ErrNoRun) {
		t.Errorf("Watch with no run = %v, want ErrNoRun", err)
	}
	s = &ghScript{reply: replyWith("9", "cancelled")}
	_, err := newGhCI(t, s).Watch(context.Background(), PR{Tip: sha('a')})
	var ended *RunConclusionError
	if !errors.As(err, &ended) || ended.Conclusion != "cancelled" || ended.Run != "9" {
		t.Errorf("Watch of a cancelled run = %v, want a RunConclusionError for run 9 and cancelled", err)
	}
}

func TestPeekCIReadsTheRunOnceAndNeverWatchesIt(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name    string
		view    string
		want    Verdict
		pending bool
	}{
		{"complete and green", recordedRunViewOK, Green, false},
		{"still running", recordedRunViewActive, "", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := &ghScript{}
			s.reply = func(args []string) (string, int, error) {
				switch {
				case has(args, "run", "list"):
					return runListJSON("9"), 0, nil
				case has(args, "run", "view"):
					return c.view, 0, nil
				}
				return "", 0, nil
			}
			ci := newGhCI(t, s)
			ci.Settle = func(context.Context) error { t.Error("a peek must not wait"); return nil }
			v, err := PeekCI{CI: ci}.Watch(context.Background(), PR{Tip: sha('a')})
			if v != c.want || errors.Is(err, ErrRunPending) != c.pending || (err != nil) != c.pending {
				t.Errorf("Peek = %q, %v; want %q pending=%v", v, err, c.want, c.pending)
			}
			if slices.ContainsFunc(s.calls, func(a []string) bool { return has(a, "run", "watch") }) {
				t.Errorf("a peek ran gh run watch: %v", s.calls)
			}
		})
	}
}

func TestPeekCIStopsOnAMissingRunWithoutWaiting(t *testing.T) {
	t.Parallel()
	s := &ghScript{reply: func([]string) (string, int, error) { return recordedRunListEmpty, 0, nil }}
	ci := newGhCI(t, s)
	ci.Settle = func(context.Context) error { t.Error("a peek must not wait"); return nil }
	if _, err := (PeekCI{CI: ci}).Watch(context.Background(), PR{Tip: sha('a')}); !errors.Is(err, ErrNoRun) {
		t.Errorf("Peek with no run = %v, want ErrNoRun", err)
	}
}

func TestTrainPlanChecksTheCIOfTheBaseItPlansOn(t *testing.T) {
	t.Parallel()
	ghDown := errors.New("gh: HTTP 502")
	cases := []struct {
		name      string
		stub      mainCIStub
		fix       bool
		wantState MainCIState
		refused   string
		otherErr  error
	}{
		{name: "green", stub: mainCIStub{verdict: Green}, wantState: MainGreen},
		{name: "pending in a peek", stub: mainCIStub{err: fmt.Errorf("%w: run 9", ErrRunPending)}, wantState: MainPending},
		{name: "red", stub: mainCIStub{verdict: Red}, refused: "red"},
		{name: "no run", stub: mainCIStub{err: fmt.Errorf("%w of required.yml", ErrNoRun)}, refused: "no CI run"},
		{name: "cancelled", stub: mainCIStub{err: &RunConclusionError{Run: "9", Conclusion: "cancelled"}}, refused: "cancelled"},
		{name: "gh is down", stub: mainCIStub{err: ghDown}, otherErr: ghDown},
		{name: "red with the fix flag", stub: mainCIStub{verdict: Red}, fix: true, wantState: MainRedOverridden},
		{name: "cancelled with the fix flag", stub: mainCIStub{err: &RunConclusionError{Run: "9", Conclusion: "cancelled"}}, fix: true, wantState: MainRedOverridden},
		{name: "gh is down with the fix flag", stub: mainCIStub{err: ghDown}, fix: true, otherErr: ghDown},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			r := newTrainRig(t)
			r.pr(t, 1, "feat: a", map[string]string{"a.md": "a\n"})
			stub := c.stub
			r.train.MainCI, r.train.MainRedFix = &stub, c.fix
			plan, err := r.train.Plan(context.Background(), []int{1})
			if !slices.Equal(stub.asked, []string{r.originMain()}) {
				t.Errorf("the check asked %v, want the plan base %s once", stub.asked, r.originMain())
			}
			var refusal *MainRefusal
			switch {
			case c.otherErr != nil:
				if !errors.Is(err, c.otherErr) || errors.As(err, &refusal) {
					t.Errorf("Plan = %v, want the gh error and no main refusal", err)
				}
			case c.refused != "":
				if !errors.As(err, &refusal) || refusal.SHA != r.originMain() || refusal.Reason != c.refused || len(plan.Kept) != 0 {
					t.Errorf("Plan = %+v, %v; want a main refusal of %s for %q and no plan", plan, err, r.originMain(), c.refused)
				}
			default:
				if err != nil || plan.Main != (MainState{SHA: r.originMain(), State: c.wantState}) || len(plan.Kept) != 1 {
					t.Errorf("Plan = %+v, %v; want main %s %s and one kept PR", plan.Main, err, r.originMain(), c.wantState)
				}
			}
		})
	}
}

func TestMainRefusalNamesTheCommitAndTheReason(t *testing.T) {
	t.Parallel()
	msg := (&MainRefusal{SHA: sha('c'), Reason: "cancelled"}).Error()
	for _, want := range []string{ShortSHA(sha('c')), "(cancelled)", "red base"} {
		if !strings.Contains(msg, want) {
			t.Errorf("refusal %q lacks %q", msg, want)
		}
	}
}

func TestMergerEmitsEachStepOfARedSplitAndTheLandings(t *testing.T) {
	t.Parallel()
	r := newLandRig(sha('0'))
	r.ci.red = func(b Batch, _ int) bool { return b.Index == 1 }
	var steps []Step
	r.merger.Progress = func(s Step) { steps = append(steps, s) }
	if _, err := r.merger.Land(context.Background(), recs(1, 2)); err != nil {
		t.Fatal(err)
	}
	want := []Step{
		{Kind: StepTest, PRs: []int{1, 2}, Batch: 1},
		{Kind: StepVerdict, PRs: []int{1, 2}, Batch: 1, Verdict: Red},
		{Kind: StepRerun, PRs: []int{1, 2}, Batch: 1},
		{Kind: StepVerdict, PRs: []int{1, 2}, Batch: 1, Verdict: Red},
		{Kind: StepSplit, PRs: []int{1, 2}},
		{Kind: StepTest, PRs: []int{1}, Batch: 2},
		{Kind: StepVerdict, PRs: []int{1}, Batch: 2, Verdict: Green},
		{Kind: StepLand, PRs: []int{1}, Batch: 2},
		{Kind: StepTest, PRs: []int{2}, Batch: 3},
		{Kind: StepVerdict, PRs: []int{2}, Batch: 3, Verdict: Green},
		{Kind: StepLand, PRs: []int{2}, Batch: 3},
	}
	if !slices.EqualFunc(steps, want, func(a, b Step) bool {
		return a.Kind == b.Kind && slices.Equal(a.PRs, b.PRs) && a.Batch == b.Batch && a.Verdict == b.Verdict
	}) {
		t.Errorf("steps =\n%+v\nwant\n%+v", steps, want)
	}
}

func TestMergerWithoutProgressStillLands(t *testing.T) {
	t.Parallel()
	out, err := newLandRig(sha('0')).merger.Land(context.Background(), recs(1))
	if err != nil || !slices.Equal(out.Landed, []int{1}) {
		t.Fatalf("Land without progress = %+v, %v", out, err)
	}
}

func TestRunConclusionErrorNamesTheRunAndItsConclusion(t *testing.T) {
	t.Parallel()
	msg := (&RunConclusionError{Run: "9", Conclusion: "cancelled"}).Error()
	if !strings.Contains(msg, "run 9") || !strings.Contains(msg, `"cancelled"`) {
		t.Errorf("message %q must name run 9 and the conclusion", msg)
	}
}

func TestStepKindsAreDistinctWords(t *testing.T) {
	t.Parallel()
	kinds := []StepKind{StepTest, StepVerdict, StepRerun, StepSplit, StepLand}
	seen := map[StepKind]bool{}
	for _, k := range kinds {
		if k == "" || seen[k] {
			t.Errorf("step kind %q is empty or repeated", k)
		}
		seen[k] = true
	}
}

func TestMergerEmitsNoVerdictStepWhenTheWatchFails(t *testing.T) {
	t.Parallel()
	r := newLandRig(sha('0'))
	r.ci.watchErr = errors.New("gh down")
	var steps []Step
	r.merger.Progress = func(s Step) { steps = append(steps, s) }
	if _, err := r.merger.Land(context.Background(), recs(1)); err == nil {
		t.Fatal("want the watch error")
	}
	for _, s := range steps {
		if s.Kind == StepVerdict {
			t.Errorf("a failed watch emitted %+v", s)
		}
	}
}
