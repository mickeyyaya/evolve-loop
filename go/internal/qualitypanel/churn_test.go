package qualitypanel

import (
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

const (
	prodFile = "go/internal/widget/add.go"
	testFile = "go/internal/widget/add_test.go"
	docFile  = "docs/notes.md"
)

func churnInput(repo *gittest.Repo) ChurnInput {
	return ChurnInput{Repo: repo.Dir, Now: fixtureNow, Window: "commits:14-30d", Source: "git", Cycle: 7}
}

func churnRepo(t *testing.T, rewriteAfter time.Duration) *gittest.Repo {
	t.Helper()
	repo := gittest.Fixture(t)
	added := fixtureNow.Add(-20 * day)
	commitAt(t, repo, added, map[string]string{prodFile: numberedLines("line", 10), testFile: numberedLines("check", 3), docFile: "note\n"}, "feat: add widget")
	if rewriteAfter > 0 {
		commitAt(t, repo, added.Add(rewriteAfter), map[string]string{prodFile: "line1\nline2\nline3\nline4 rewritten\nline5\nline6\nline7\nline8\nline9\nline10\n"}, "fix: rewrite line four")
	}
	return repo
}

func TestChurn_OneOfTenLinesRewrittenWithinFourteenDaysIsTenPercent(t *testing.T) {
	res, err := MeasureChurn(churnInput(churnRepo(t, 10*day)))
	if err != nil {
		t.Fatal(err)
	}
	if res.Added != 10 || res.Survived != 9 || res.OQ2.Interval.Value != 0.1 || res.OQ2.N != 1 {
		t.Errorf("result = %+v, want added 10 survived 9 value 0.1 n 1", res)
	}
	r := res.OQ2
	if r.Index != "OQ2" || r.Method != MethodBootstrap95 || r.Window != "commits:14-30d" || r.Source != "git" || r.Cycle != 7 {
		t.Errorf("reading = %+v, want the labels of the input", r)
	}
}

func TestChurn_ARewriteAfterFourteenDaysDoesNotCount(t *testing.T) {
	res, err := MeasureChurn(churnInput(churnRepo(t, 15*day)))
	if err != nil || res.Added != 10 || res.Survived != 10 || res.OQ2.Interval.Value != 0 {
		t.Errorf("result = %+v, %v; want 10 of 10 survived", res, err)
	}
}

func TestChurn_ADeletedFileHasNoSurvivingLine(t *testing.T) {
	repo := churnRepo(t, 0)
	commitAt(t, repo, fixtureNow.Add(-18*day), map[string]string{prodFile: ""}, "refactor: drop widget")
	res, err := MeasureChurn(churnInput(repo))
	if err != nil || res.Added != 10 || res.Survived != 0 || res.OQ2.Interval.Value != 1 {
		t.Errorf("result = %+v, %v; want 10 added, none surviving, churn 1", res, err)
	}
}

func TestChurn_OnlyTestAndDocumentLinesIsNoData(t *testing.T) {
	repo := gittest.Fixture(t)
	commitAt(t, repo, fixtureNow.Add(-20*day), map[string]string{testFile: numberedLines("check", 3), docFile: "note\n"}, "test: add checks")
	res, err := MeasureChurn(churnInput(repo))
	if err != nil || res.Added != 0 || res.OQ2.N != 0 {
		t.Errorf("result = %+v, %v; want no data", res, err)
	}
	if got, _ := Lookup("OQ2"); Judge(got, res.OQ2) != StatusNoData {
		t.Errorf("status = %s, want no_data", Judge(got, res.OQ2))
	}
}

func TestChurn_ADirectoryThatIsNotARepositoryIsAnError(t *testing.T) {
	if res, err := MeasureChurn(ChurnInput{Repo: t.TempDir(), Now: fixtureNow}); err == nil {
		t.Errorf("result = %+v, nil error, want an error", res)
	}
}

func TestChurn_EveryGitFailureIsAnError(t *testing.T) {
	repo := churnRepo(t, 10*day)
	var total int
	if _, err := measureChurn(churnInput(repo), readerFailingAt(repo.Dir, &total, 0)); err != nil || total < 5 {
		t.Fatalf("clean run: err %v after %d git calls, want nil after at least 5", err, total)
	}
	for failAt := 1; failAt <= total; failAt++ {
		var calls int
		if res, err := measureChurn(churnInput(repo), readerFailingAt(repo.Dir, &calls, failAt)); err == nil {
			t.Errorf("git call %d of %d failed, result = %+v, nil error, want the error", failAt, total, res)
		}
	}
}
