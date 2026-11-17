package inboxcurator

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func filedItem(id, created string) inboxbatch.Item {
	it := item(id, "t "+id, []string{"go/x.go"})
	it.Path = id + ".json"
	it.CreatedAt = created
	return it
}

func TestClosedOnMain_ADeterministicConsumeNamesTheCommit(t *testing.T) {
	open := []inboxbatch.Item{filedItem("x", "2026-10-01T00:00:00Z"), filedItem("y", "2026-10-01T00:00:00Z")}
	history := []Change{{Commit: "abc1234", Closes: []string{"x"}, When: landedAt}}

	got := ClosedOnMain(history, open)

	if len(got) != 1 {
		t.Fatalf("ClosedOnMain = %+v, want one proposal", got)
	}
	p := got[0]
	if p.Kind != ProposeConsume || p.ID != "x" || p.Path != "x.json" || !p.Deterministic || p.Trigger != TriggerHistory {
		t.Errorf("proposal = %+v, want the deterministic history consume of x", p)
	}
	if p.Reason != "main holds Closes-Inbox for this item (abc1234)" || !slices.Contains(p.Evidence, "commit abc1234") {
		t.Errorf("reason = %q evidence = %v, want the commit named", p.Reason, p.Evidence)
	}
}

func TestClosedOnMain_ProposesNothingWhenNoTrailerCloses(t *testing.T) {
	filed := "2026-10-01T00:00:00Z"
	for name, tc := range map[string]struct {
		item   inboxbatch.Item
		change Change
	}{
		"a commit older than the item":           {filedItem("x", "2026-11-01T00:00:00Z"), Change{Commit: "c", Closes: []string{"x"}, When: landedAt}},
		"an unknown id":                          {filedItem("x", filed), Change{Commit: "c", Closes: []string{"other"}, When: landedAt}},
		"a zero filed time":                      {filedItem("x", ""), Change{Commit: "c", Closes: []string{"x"}, When: landedAt}},
		"a name and the surface with no trailer": {filedItem("x", filed), Change{Commit: "c", Text: "feat: x is done", Paths: []string{"go/x.go"}, Tests: []string{"TestX"}, When: landedAt}},
	} {
		if got := ClosedOnMain([]Change{tc.change}, []inboxbatch.Item{tc.item}); len(got) != 0 {
			t.Errorf("%s: ClosedOnMain = %+v, want no proposal", name, got)
		}
	}
}

func TestMatchLanding_ATrailerOlderThanTheItemDoesNotConsumeIt(t *testing.T) {
	it := filedItem("x", "2026-11-01T00:00:00Z")

	old := MatchLanding(Change{Commit: "c", Closes: []string{"x"}, When: landedAt}, []inboxbatch.Item{it}, testsIn(), 0.5)
	fresh := MatchLanding(Change{Commit: "c", Closes: []string{"x"}, When: landedAt.AddDate(0, 2, 0)}, []inboxbatch.Item{it}, testsIn(), 0.5)

	if len(old) != 0 || len(fresh) != 1 || !fresh[0].Deterministic {
		t.Errorf("old = %+v fresh = %+v, want only the later commit to consume", old, fresh)
	}
}

func TestHistoryFloor_IsTheOldestNonZeroFiledTime(t *testing.T) {
	open := []inboxbatch.Item{filedItem("a", "2026-11-01T00:00:00Z"), filedItem("b", ""), filedItem("c", "2026-10-01T00:00:00Z")}

	got, ok := HistoryFloor(open)

	if !ok || !got.Equal(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("HistoryFloor = %v, %v; want 2026-10-01", got, ok)
	}
	if _, ok := HistoryFloor([]inboxbatch.Item{filedItem("b", "")}); ok {
		t.Error("HistoryFloor of an item with no filed time = ok, want not ok")
	}
	if _, ok := HistoryFloor(nil); ok {
		t.Error("HistoryFloor of no items = ok, want not ok")
	}
}

func historyRepo(t *testing.T) *gittest.Repo {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Write("README.md", "x\n")
	repo.Git("add", ".")
	repo.Git("commit", "-q", "-m", "base")
	repo.Write("a.txt", "x\n")
	repo.Git("add", ".")
	repo.Git("commit", "-q", "-m", "work", "-m", "Closes-Inbox: x-item")
	repo.Write("b.txt", "x\n")
	repo.Git("add", ".")
	repo.Git("commit", "-q", "-m", "prose only", "-m", "Refs-Inbox: y-item")
	return repo
}

func TestClosedHistory_ReadsOnlyTheCommitsThatCarryTheTrailer(t *testing.T) {
	repo := historyRepo(t)
	git := gitexec.Default(repo.Dir)

	got, err := ClosedHistory(context.Background(), git, time.Now().Add(-time.Hour))

	if err != nil || len(got) != 1 || !slices.Equal(got[0].Closes, []string{"x-item"}) || got[0].When.IsZero() || len(got[0].Paths) != 0 {
		t.Fatalf("ClosedHistory = %+v, %v; want the one closing commit with no paths", got, err)
	}
}

func TestClosedHistory_ACommitBeforeTheFloorOrNoTrailerIsNoRead(t *testing.T) {
	repo := historyRepo(t)
	git := gitexec.Default(repo.Dir)

	got, err := ClosedHistory(context.Background(), git, time.Now().Add(time.Hour))

	if err != nil || got != nil {
		t.Errorf("ClosedHistory = %+v, %v; want nil", got, err)
	}
}

func TestClosedHistory_AMergedBranchTrailerIsFound(t *testing.T) {
	repo, _, _ := mergeFixture(t, map[string][]string{"feature": {"Closes-Inbox: branch-item"}}, "feature")
	repo.Git("merge", "--no-ff", "-q", "-m", "Merge feature", "feature")

	got, err := ClosedHistory(context.Background(), gitexec.Default(repo.Dir), time.Now().Add(-time.Hour))

	if err != nil || len(got) != 1 || !slices.Equal(got[0].Closes, []string{"branch-item"}) {
		t.Errorf("ClosedHistory = %+v, %v; want the branch commit", got, err)
	}
}

func TestClosedHistory_AFaultOnTheLogIsAnError(t *testing.T) {
	repo := historyRepo(t)
	git := gitexec.Default(repo.Dir)
	gittest.FaultOn(t, "log")

	if _, err := ClosedHistory(context.Background(), git, time.Now().Add(-time.Hour)); err == nil || !strings.Contains(err.Error(), "find the commits that close") {
		t.Errorf("ClosedHistory = %v, want the log fault named", err)
	}
}

func TestCurate_AnItemThatMainClosedHasItsConsumeBeforeItsReroute(t *testing.T) {
	gate := func(p string) bool { return strings.HasPrefix(p, "go/internal/guards/") }
	closed := filedItem("x", "2026-10-01T00:00:00Z")
	closed.Kind, closed.Route, closed.Files = "feature", inboxbatch.RouteLaneValue, []string{"go/internal/guards/ship.go"}
	open := filedItem("y", "2026-10-01T00:00:00Z")
	open.Kind, open.Route, open.Files = "feature", inboxbatch.RouteLaneValue, []string{"go/internal/guards/role.go"}
	history := []Change{{Commit: "abc1234", Closes: []string{"x"}, When: landedAt}}

	got := Curate(Inputs{Open: []inboxbatch.Item{closed, open}, History: history, Backstop: true, IsProtected: gate, MinShare: 0.5, HasTest: testsIn()})

	var ofX, ofY []Proposal
	for _, p := range got {
		switch p.ID {
		case "x":
			ofX = append(ofX, p)
		case "y":
			ofY = append(ofY, p)
		}
	}
	if len(ofX) != 2 || ofX[0].Kind != ProposeConsume || ofX[0].Trigger != TriggerHistory || ofX[1].Kind != ProposeReroute {
		t.Errorf("proposals for x = %+v, want the history consume first, so the apply skips the later reroute", ofX)
	}
	if len(ofY) != 1 || ofY[0].Kind != ProposeReroute || ofY[0].Target != RouteConsoleTarget {
		t.Errorf("proposals for y = %+v, want the reroute to the console", ofY)
	}
}

func TestCurate_HistoryFeedsOnlyTheBackstop(t *testing.T) {
	open := []inboxbatch.Item{filedItem("x", "2026-10-01T00:00:00Z")}
	history := []Change{{Commit: "abc1234", Closes: []string{"x"}, When: landedAt}}

	if got := Curate(Inputs{Open: open, History: history, MinShare: 0.5, HasTest: testsIn()}); len(got) != 0 {
		t.Errorf("Curate without the backstop = %+v, want none", got)
	}
	got := Curate(Inputs{Open: open, History: history, Backstop: true, MinShare: 0.5, HasTest: testsIn()})
	if len(got) != 1 || got[0].ID != "x" || !got[0].Deterministic || got[0].Trigger != TriggerHistory {
		t.Errorf("Curate at the backstop = %+v, want the history consume of x", got)
	}
}
