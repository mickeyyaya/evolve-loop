package main

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciwatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

type laggingPR struct {
	oldHead, newHead string
	staleViews       int
	updated          bool
	viewsAfterUpdate int
}

type laggingGitHub struct {
	prs          map[string]*laggingPR
	behindBefore map[string]int
	behindAfter  map[string]int
	failAfter    string
	merged       map[string]string
	calls        []string
}

func (f *laggingGitHub) head(n string) string {
	pr := f.prs[n]
	if !pr.updated {
		return pr.oldHead
	}
	pr.viewsAfterUpdate++
	if pr.viewsAfterUpdate <= pr.staleViews {
		return pr.oldHead
	}
	return pr.newHead
}

func (f *laggingGitHub) behind(head string) int {
	if len(f.merged) > 0 {
		if b, ok := f.behindAfter[head]; ok {
			return b
		}
	}
	return f.behindBefore[head]
}

func (f *laggingGitHub) gh(_ context.Context, args ...string) ([]byte, error) {
	call := strings.Join(args, " ")
	f.calls = append(f.calls, call)
	switch {
	case strings.HasPrefix(call, "pr view") && strings.HasSuffix(call, "state,mergeCommit"):
		if sha, ok := f.merged[args[2]]; ok {
			return []byte(`{"state":"MERGED","mergeCommit":{"oid":"` + sha + `"}}`), nil
		}
		return []byte(`{"state":"OPEN","mergeCommit":null}`), nil
	case strings.HasPrefix(call, "pr view"):
		if f.failAfter != "" && f.prs[args[2]].updated && f.prs[args[2]].viewsAfterUpdate >= 1 {
			return nil, errors.New(f.failAfter)
		}
		return []byte(openPR(f.head(args[2]))), nil
	case strings.HasPrefix(call, "pr update-branch"):
		f.prs[args[2]].updated = true
		return nil, nil
	case strings.HasPrefix(call, "pr merge"):
		f.merged[args[2]] = strings.Repeat(args[2][len(args[2])-1:], 40)
		return nil, nil
	case strings.HasPrefix(call, "api "):
		return []byte(`{"behind_by":` + strconv.Itoa(f.behind(call[strings.LastIndex(call, "...")+3:])) + `}`), nil
	}
	return nil, nil
}

type ciLog struct{ heads []string }

func (c *ciLog) greenFetch(_ context.Context, sha string) (ciwatch.RunStatus, error) {
	c.heads = append(c.heads, sha)
	return ciwatch.RunStatus{Status: ciwatch.StatusCompleted, Conclusion: ciwatch.ConclusionSuccess}, nil
}

func (c *ciLog) saw(sha string) bool {
	for _, h := range c.heads {
		if h == sha {
			return true
		}
	}
	return false
}

func laggingMerger(gh *laggingGitHub, ci *ciLog) (prMerger, *strings.Builder, *strings.Builder, *time.Duration) {
	var stdout, stderr strings.Builder
	var slept time.Duration
	clock := time.Date(2026, 10, 17, 4, 30, 0, 0, time.UTC)
	return prMerger{
		method: "squash", poll: time.Second, wait: 10 * time.Second, updateBranch: true, gh: gh.gh, fetch: ci.greenFetch,
		liveRuns: func() ([]runlease.LiveRun, error) { return nil, nil },
		now:      func() time.Time { return clock },
		sleep:    func(d time.Duration) { clock = clock.Add(d); slept += d },
		stdout:   &stdout, stderr: &stderr,
	}, &stdout, &stderr, &slept
}

func callsWith(calls []string, prefix string) []string {
	var hits []string
	for _, c := range calls {
		if strings.HasPrefix(c, prefix) {
			hits = append(hits, c)
		}
	}
	return hits
}

func TestPRMerger_WaitsForNewHeadAfterUpdateBranch(t *testing.T) {
	t.Parallel()
	for _, staleViews := range []int{1, 3} {
		gh := &laggingGitHub{
			prs:          map[string]*laggingPR{"916": {oldHead: "old916", newHead: "new916", staleViews: staleViews}},
			behindBefore: map[string]int{"old916": 4, "new916": 0},
			merged:       map[string]string{},
		}
		ci := &ciLog{}
		m, stdout, stderr, _ := laggingMerger(gh, ci)
		code := m.mergeAll(context.Background(), []string{"916"})
		if code != 0 || !strings.HasPrefix(stdout.String(), "merged #916 ") {
			t.Fatalf("stale views %d: exit %d stdout %q stderr %q; want #916 merged on its first pass", staleViews, code, stdout.String(), stderr.String())
		}
		if merges := callsWith(gh.calls, "pr merge 916"); len(merges) != 1 || !strings.HasSuffix(merges[0], "--match-head-commit new916") {
			t.Errorf("stale views %d: want one merge bound to the new head, got %q", staleViews, merges)
		}
		if ci.saw("old916") || !ci.saw("new916") {
			t.Errorf("stale views %d: CI must be awaited on the new head only, fetched %q", staleViews, ci.heads)
		}
		if updates := callsWith(gh.calls, "pr update-branch 916"); len(updates) != 1 {
			t.Errorf("stale views %d: want exactly one update-branch, got %q", staleViews, updates)
		}
	}
}

func TestPRMerger_TrainWithUpdateBranch(t *testing.T) {
	t.Parallel()
	gh := &laggingGitHub{
		prs: map[string]*laggingPR{
			"916": {oldHead: "a916", newHead: "a916x", staleViews: 1},
			"917": {oldHead: "b917", newHead: "b917x", staleViews: 1},
		},
		behindBefore: map[string]int{"a916": 0, "b917": 0, "b917x": 0},
		behindAfter:  map[string]int{"b917": 1},
		merged:       map[string]string{},
	}
	ci := &ciLog{}
	m, stdout, stderr, _ := laggingMerger(gh, ci)
	code := m.mergeAll(context.Background(), []string{"916", "917"})
	if code != 0 || strings.Count(stdout.String(), "merged #") != 2 || !strings.Contains(stdout.String(), "merged #917 ") {
		t.Fatalf("exit %d stdout %q stderr %q; want both PRs of the train merged in one run", code, stdout.String(), stderr.String())
	}
	if merges := callsWith(gh.calls, "pr merge 917"); len(merges) != 1 || !strings.HasSuffix(merges[0], "--match-head-commit b917x") {
		t.Errorf("want #917 merged at its updated head, got %q", merges)
	}
	if ci.saw("b917") {
		t.Errorf("CI must not be awaited on #917's pre-update head, fetched %q", ci.heads)
	}
}

func TestPRMerger_UpdateBranchTimeout(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, newHead, failAfter string
		code                     int
	}{
		{"head never moves", "old12", "", cliExitRefused},
		{"empty head is no new head", "", "", cliExitRefused},
		{"view fails while waiting", "new12", "gh: HTTP 502", exitIO},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gh := &laggingGitHub{
				prs:          map[string]*laggingPR{"12": {oldHead: "old12", newHead: c.newHead, staleViews: 1 << 20}},
				behindBefore: map[string]int{"old12": 4},
				failAfter:    c.failAfter,
				merged:       map[string]string{},
			}
			if c.newHead == "" {
				gh.prs["12"].staleViews = 1
			}
			ci := &ciLog{}
			m, stdout, stderr, slept := laggingMerger(gh, ci)
			code := m.mergeAll(context.Background(), []string{"12", "13"})
			if code != c.code || stdout.Len() != 0 || !strings.HasSuffix(stderr.String(), "not attempted: #13\n") {
				t.Fatalf("exit %d stdout %q stderr %q; want exit %d and #13 not attempted", code, stdout.String(), stderr.String(), c.code)
			}
			if len(callsWith(gh.calls, "pr merge")) != 0 || len(callsWith(gh.calls, "pr view 13")) != 0 {
				t.Errorf("a refusal must merge nothing and touch no later PR: %q", gh.calls)
			}
			if len(ci.heads) != 0 {
				t.Errorf("no new head was seen, so CI must not be awaited on any head; fetched %q", ci.heads)
			}
			if *slept > m.headWaitBound()+m.poll {
				t.Errorf("the head wait must stay inside its bound %s, slept %s", m.headWaitBound(), *slept)
			}
			if c.code == cliExitRefused && *slept == 0 {
				t.Errorf("a lagging head must be polled for before the refusal, slept %s", *slept)
			}
		})
	}
}

func pendingFetch(_ context.Context, _ string) (ciwatch.RunStatus, error) {
	return ciwatch.RunStatus{Status: "in_progress"}, nil
}

func movedHeadGitHub() *laggingGitHub {
	return &laggingGitHub{
		prs:          map[string]*laggingPR{"916": {oldHead: "old916", newHead: "new916", staleViews: 1}},
		behindBefore: map[string]int{"old916": 4},
		merged:       map[string]string{},
	}
}

func TestPRMerger_UpdateBranchWithoutWaitMergesAGreenNewHead(t *testing.T) {
	t.Parallel()
	gh := movedHeadGitHub()
	m, stdout, stderr, _ := laggingMerger(gh, &ciLog{})
	m.wait = 0
	code := m.mergeAll(context.Background(), []string{"916"})
	if code != cliExitOK || !strings.HasPrefix(stdout.String(), "merged #916 ") {
		t.Fatalf("exit %d stdout %q stderr %q; want #916 merged on the new head", code, stdout.String(), stderr.String())
	}
	if merges := callsWith(gh.calls, "pr merge 916"); len(merges) != 1 || !strings.HasSuffix(merges[0], "--match-head-commit new916") {
		t.Errorf("want one merge bound to the new head, got %q", merges)
	}
}

func TestPRMerger_UpdateBranchWithoutWaitRefusesAPendingNewHead(t *testing.T) {
	t.Parallel()
	gh := movedHeadGitHub()
	m, stdout, stderr, _ := laggingMerger(gh, &ciLog{})
	m.wait = 0
	m.fetch = pendingFetch
	code := m.mergeAll(context.Background(), []string{"916", "917"})
	if code != cliExitRefused || stdout.Len() != 0 || !strings.Contains(stderr.String(), "#916 refused: required CI in_progress on new916; pending") {
		t.Fatalf("exit %d stdout %q stderr %q; want exit 3 naming the new head", code, stdout.String(), stderr.String())
	}
	if len(callsWith(gh.calls, "pr merge")) != 0 || !strings.HasSuffix(stderr.String(), "not attempted: #917\n") {
		t.Errorf("a pending head must merge nothing and leave #917 not attempted: %q %q", gh.calls, stderr.String())
	}
}

func TestPRMerger_HeadWaitOutlastsTheMoveBoundWhenWaitIsLonger(t *testing.T) {
	t.Parallel()
	gh := movedHeadGitHub()
	gh.prs["916"].staleViews = 90
	m, stdout, stderr, slept := laggingMerger(gh, &ciLog{})
	m.wait = 5 * time.Minute
	code := m.mergeAll(context.Background(), []string{"916"})
	if code != cliExitOK || !strings.HasPrefix(stdout.String(), "merged #916 ") {
		t.Fatalf("exit %d stdout %q stderr %q; want the head awaited past %s", code, stdout.String(), stderr.String(), prHeadMoveBound)
	}
	if *slept <= prHeadMoveBound {
		t.Errorf("slept %s, want more than %s", *slept, prHeadMoveBound)
	}
}

func TestPRMergeJourney_UpdateBranchTrainWithAPendingNewHead(t *testing.T) {
	t.Parallel()
	a, err := parsePRMergeArgs([]string{"916", "917", "--update-branch", "--json"})
	if err != nil {
		t.Fatal(err)
	}
	m, _, _, _ := laggingMerger(movedHeadGitHub(), &ciLog{})
	m.wait = a.wait
	m.updateBranch = a.updateBranch
	m.fetch = pendingFetch
	var out, errOut strings.Builder
	code := mergePRs(m, a, &out, &errOut)
	var env struct {
		Merged       []prMerged `json:"merged"`
		Refused      string     `json:"refused"`
		NotAttempted []string   `json:"not_attempted"`
	}
	if err := json.Unmarshal([]byte(out.String()), &env); err != nil {
		t.Fatalf("stdout %q is not JSON: %v", out.String(), err)
	}
	if code != cliExitRefused || len(env.Merged) != 0 || !strings.Contains(env.Refused, "required CI in_progress on new916") || strings.Join(env.NotAttempted, ",") != "917" {
		t.Errorf("exit %d envelope %+v stderr %q; want a refusal on the new head with #917 not attempted", code, env, errOut.String())
	}
	if strings.Contains(out.String(), `"updated"`) {
		t.Errorf("the result has no updated key any more: %q", out.String())
	}
}

func TestPRMerger_HeadWaitBoundIsAtLeastTheMoveBound(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		wait, want time.Duration
	}{
		{0, prHeadMoveBound},
		{10 * time.Second, prHeadMoveBound},
		{5 * time.Minute, 5 * time.Minute},
	} {
		if got := (prMerger{wait: c.wait}).headWaitBound(); got != c.want {
			t.Errorf("wait %s: head wait bound %s, want %s", c.wait, got, c.want)
		}
	}
}

func TestPRMerger_UpdateBranchWithoutWaitRefusesAtTheBound(t *testing.T) {
	t.Parallel()
	gh := &laggingGitHub{
		prs:          map[string]*laggingPR{"12": {oldHead: "old12", newHead: "new12", staleViews: 1 << 20}},
		behindBefore: map[string]int{"old12": 4},
		merged:       map[string]string{},
	}
	ci := &ciLog{}
	m, stdout, stderr, slept := laggingMerger(gh, ci)
	m.wait = 0
	code := m.mergeAll(context.Background(), []string{"12"})
	want := "#12 refused: head is still old12 after update-branch; waited 1m0s\n"
	if code != cliExitRefused || stdout.Len() != 0 || !strings.HasSuffix(stderr.String(), want) {
		t.Fatalf("exit %d stdout %q stderr %q; want a refusal naming the bound", code, stdout.String(), stderr.String())
	}
	if *slept < prHeadMoveBound-m.poll || *slept > prHeadMoveBound {
		t.Errorf("slept %s, want the full head bound %s", *slept, prHeadMoveBound)
	}
}
