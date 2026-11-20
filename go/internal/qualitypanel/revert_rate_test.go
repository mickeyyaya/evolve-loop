package qualitypanel

import (
	"math"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func revertInput(repo *gittest.Repo) RevertRateInput {
	return RevertRateInput{Repo: repo.Dir, Now: fixtureNow, Window: "days:30", Source: "git", Cycle: 7}
}

func TestRevertRate_ASubjectAndABodyBothCountAndLookalikesDoNot(t *testing.T) {
	repo := gittest.Fixture(t)
	commitAt(t, repo, fixtureNow.Add(-45*day), map[string]string{prodFile: "x\n"}, "Revert \"ancient\"")
	commitAt(t, repo, fixtureNow.Add(-10*day), map[string]string{prodFile: "a\n"}, "feat: add a")
	target := commitAt(t, repo, fixtureNow.Add(-9*day), map[string]string{prodFile: "b\n"}, "feat: add b")
	commitAt(t, repo, fixtureNow.Add(-8*day), map[string]string{prodFile: "a\n"}, "fix: undo b", "This reverts commit "+target+".")
	commitAt(t, repo, fixtureNow.Add(-7*day), map[string]string{prodFile: "c\n"}, "Revert \"feat: add a\"")
	commitAt(t, repo, fixtureNow.Add(-6*day), map[string]string{prodFile: "d\n"}, "Reverse the list order", "this reverts nothing at all")
	res, err := MeasureRevertRate(revertInput(repo))
	if err != nil || res.Reverts != 2 || res.Commits != 5 {
		t.Fatalf("result = %+v, %v; want 2 reverts of 5 commits", res, err)
	}
	want := Wilson(2, 5)
	if r := res.OQ3; r.Interval != want || r.Index != "OQ3" || r.Method != MethodWilson95 || r.N != 5 || r.Window != "days:30" || r.Cycle != 7 {
		t.Errorf("reading = %+v, want Wilson(2, 5) = %+v", r, want)
	}
}

func TestRevertRate_ALogWithNoRevertIsZeroWithTheWilsonUpperBound(t *testing.T) {
	repo := gittest.Fixture(t)
	for i := 0; i < 10; i++ {
		commitAt(t, repo, fixtureNow.Add(-time.Duration(20-i)*day), nil, "chore: step")
	}
	res, err := MeasureRevertRate(revertInput(repo))
	iv := res.OQ3.Interval
	if err != nil || res.Reverts != 0 || res.Commits != 10 || iv.Value != 0 || iv.Lo != 0 || math.Abs(iv.Hi-0.2775) > 0.001 {
		t.Errorf("result = %+v, %v; want 0 of 10 with upper bound 0.2775", res, err)
	}
}

func TestRevertRate_ACommitAfterNowIsNotInTheWindow(t *testing.T) {
	repo := gittest.Fixture(t)
	commitAt(t, repo, fixtureNow.Add(2*day), nil, "feat: from the future")
	res, err := MeasureRevertRate(revertInput(repo))
	if err != nil || res.Commits != 0 || res.OQ3.N != 0 {
		t.Errorf("result = %+v, %v; want no commit", res, err)
	}
}

func TestRevertRate_ADirectoryThatIsNotARepositoryIsAnError(t *testing.T) {
	if res, err := MeasureRevertRate(RevertRateInput{Repo: t.TempDir(), Now: fixtureNow}); err == nil {
		t.Errorf("result = %+v, nil error, want an error", res)
	}
}
