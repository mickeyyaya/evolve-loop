package gc

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func TestPlanDevWorktrees_ALandedTreeSkipsTheIdleLimitOnlyWhenEveryRuleHolds(t *testing.T) {
	cases := []struct {
		name    string
		prs     OpenPRLookup
		arrange func(e *devEnv, dir string)
		want    DevAction
		reason  string
	}{
		{"landed, clean, no pull request", noOpenPRs, nil, DevActionArchive, "landed early: branch br-tree is in main"},
		{"no pull request lookup", nil, nil, DevActionKeep, "active within the 14-day limit"},
		{"gh unavailable", func(context.Context, string) (map[string][]time.Time, error) {
			return nil, ErrPRLookupUnavailable
		}, nil, DevActionKeep, "active within the 14-day limit"},
		{"a failing lookup", func(context.Context, string) (map[string][]time.Time, error) {
			return nil, context.DeadlineExceeded
		}, nil, DevActionKeep, "landed check: " + context.DeadlineExceeded.Error()},
		{"untracked file", noOpenPRs, func(e *devEnv, dir string) { writeRepoFile(e.t, dir, "new.txt", "x\n") }, DevActionKeep, "active within the 14-day limit"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newDevEnv(t)
			dir := e.addDev("tree")
			e.commitIn(dir, "work.txt", "work\n")
			e.store.Git("update-ref", "refs/remotes/origin/main", e.store.Git("-C", dir, "rev-parse", "HEAD"))
			if tc.arrange != nil {
				tc.arrange(e, dir)
			}
			o := e.options(0)
			o.OpenPRs = tc.prs
			m, err := PlanDevWorktrees(context.Background(), o)
			if err != nil {
				t.Fatal(err)
			}
			it := m.Items[0]
			if it.Action != tc.want || !strings.Contains(it.Reason, tc.reason) {
				t.Errorf("item = %+v, want action %s with a reason that holds %q", it, tc.want, tc.reason)
			}
		})
	}
}

func TestHasOwnWork_ReadsTheOldestReflogEntryOfTheTree(t *testing.T) {
	e := newDevEnv(t)
	fresh := e.addDev("fresh")
	worked := e.addDev("worked")
	e.commitIn(worked, "w.txt", "w\n")
	for dir, want := range map[string]bool{fresh: false, worked: true} {
		head := e.store.Git("-C", dir, "rev-parse", "HEAD")
		got, err := hasOwnWork(context.Background(), gitexec.Git{Dir: dir, Exec: sysexec.DefaultRunner}, head)
		if err != nil || got != want {
			t.Errorf("hasOwnWork(%s) = %v, %v; want %v", dir, got, err, want)
		}
	}
}
