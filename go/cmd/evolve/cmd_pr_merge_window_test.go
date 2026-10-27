package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciwatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

var liveLoop = []runlease.LiveRun{{Dir: "/r/cycle-9", Lease: runlease.Lease{RunID: "cycle-9"}}}

func windowMerger(t *testing.T, policyJSON string, gh *fakePRGitHub, live func() ([]runlease.LiveRun, error)) (prMerger, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	m, err := newPRMerger(windowPlane(t, policyJSON), prMergeArgs{}, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	clock := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	m.gh, m.liveRuns = gh.gh, live
	m.fetch = func(context.Context, string) (ciwatch.RunStatus, error) {
		return ciwatch.RunStatus{Status: ciwatch.StatusCompleted, Conclusion: ciwatch.ConclusionSuccess}, nil
	}
	m.now, m.sleep = func() time.Time { return clock }, func(d time.Duration) { clock = clock.Add(d) }
	return m, &stdout, &stderr
}

func windowGH() *fakePRGitHub {
	return &fakePRGitHub{
		views:  map[string]string{"12": openPR("h12"), "13": openPR("h13")},
		behind: map[string]string{"h12": `{"behind_by":0}`, "h13": `{"behind_by":0}`},
		merged: map[string]string{"12": `{"state":"MERGED","mergeCommit":{"oid":"` + mergeSHA + `"}}`},
		files:  map[string]string{"12": "\"docs/plans/a.md\"\n\"README.md\"\n", "13": "\"docs/plans/a.md\"\n\"go/x.go\"\n"},
	}
}

func alwaysLive() ([]runlease.LiveRun, error) { return liveLoop, nil }

func TestPRMerge_WithALiveLoopAndTheWindowOnADocsPRMergesAndAGoPRIsRefused(t *testing.T) {
	t.Parallel()
	gh := windowGH()
	m, stdout, stderr := windowMerger(t, windowOn, gh, alwaysLive)

	code := m.mergeAll(context.Background(), []string{"12", "13"})

	if code != cliExitRefused || stdout.String() != "merged #12 "+mergeSHA+"\n" {
		t.Fatalf("exit %d stdout %q stderr %q; want #12 merged and #13 refused", code, stdout.String(), stderr.String())
	}
	want := "evolve pr merge: #13 refused: a loop is running: run cycle-9 (/r/cycle-9) is live; #13 waits for a boundary: 2 paths need a boundary: docs/plans/a.md, go/x.go\n"
	if stderr.String() != want {
		t.Errorf("stderr %q, want %q", stderr.String(), want)
	}
	calls := strings.Join(gh.calls, "\n")
	if !strings.Contains(calls, "pr merge 12 --merge --match-head-commit h12") || strings.Contains(calls, "pr merge 13") || strings.Contains(calls, "pr view 13") {
		t.Errorf("want a merge of #12 only, and #13 refused before any view:\n%s", calls)
	}
}

func TestPRMerge_BelowTheOpenWindowALiveLoopRefusesEveryPRAsToday(t *testing.T) {
	t.Parallel()
	for name, policyJSON := range map[string]string{
		"stage off at integration enforce": `{"pr":{"merge_window":{"stage":"off"}},"fleet":{"integration":{"stage":"enforce"}}}`,
		"stage on below integration":       `{"pr":{"merge_window":{"stage":"on"}}}`,
		"no policy":                        "",
	} {
		t.Run(name, func(t *testing.T) {
			gh := windowGH()
			m, stdout, stderr := windowMerger(t, policyJSON, gh, alwaysLive)
			code := m.mergeAll(context.Background(), []string{"12", "13"})
			want := "evolve pr merge: refused: a loop is running: run cycle-9 (/r/cycle-9) is live\nevolve pr merge: not attempted: #12 #13\n"
			if code != cliExitRefused || stdout.Len() != 0 || stderr.String() != want || len(gh.calls) != 0 {
				t.Errorf("exit %d stdout %q stderr %q calls %v; want today's refusal of every PR and no gh call", code, stdout.String(), stderr.String(), gh.calls)
			}
		})
	}
}

func TestPRMerge_AnOpenWindowFailsLoudOnAnUnreadableWindowOrPlane(t *testing.T) {
	t.Parallel()
	gh := windowGH()
	gh.failCall = "api --paginate"
	m, _, stderr := windowMerger(t, windowOn, gh, alwaysLive)
	if code := m.mergeAll(context.Background(), []string{"12"}); code != exitIO || !strings.Contains(stderr.String(), "gh: HTTP 502") {
		t.Errorf("an unreadable file list: exit %d stderr %q, want exitIO", code, stderr.String())
	}
	m, _, stderr = windowMerger(t, windowOn, windowGH(), func() ([]runlease.LiveRun, error) { return nil, errors.New("git worktree list: exit 128") })
	if code := m.mergeAll(context.Background(), []string{"12"}); code != exitIO || !strings.Contains(stderr.String(), "git worktree list") {
		t.Errorf("an unprovable plane: exit %d stderr %q, want exitIO", code, stderr.String())
	}
}

func TestPRMerge_AnOpenWindowWithAnIdlePlaneMergesWithoutTheWindowRule(t *testing.T) {
	t.Parallel()
	gh := windowGH()
	m, stdout, _ := windowMerger(t, windowOn, gh, func() ([]runlease.LiveRun, error) { return nil, nil })
	if code := m.mergeAll(context.Background(), []string{"12"}); code != 0 || stdout.String() != "merged #12 "+mergeSHA+"\n" {
		t.Fatalf("exit %d stdout %q", code, stdout.String())
	}
	if strings.Contains(strings.Join(gh.calls, "\n"), "pulls/12/files") {
		t.Errorf("an idle plane needs no window rule: %v", gh.calls)
	}
}

func TestPRMerge_ALoopThatStartsDuringTheWaitRefusesABoundaryPRAtTheMerge(t *testing.T) {
	t.Parallel()
	calls := 0
	live := func() ([]runlease.LiveRun, error) {
		calls++
		if calls == 1 {
			return nil, nil
		}
		return liveLoop, nil
	}
	gh := windowGH()
	m, _, stderr := windowMerger(t, windowOn, gh, live)
	if code := m.mergeAll(context.Background(), []string{"13"}); code != cliExitRefused || !strings.Contains(stderr.String(), "#13 waits for a boundary") {
		t.Errorf("exit %d stderr %q, want the boundary refusal at the merge", code, stderr.String())
	}
	if strings.Contains(strings.Join(gh.calls, "\n"), "pr merge 13") {
		t.Errorf("a boundary PR must not merge into a live wave: %v", gh.calls)
	}
}

func TestNewPRMerger_AnUnreadableWindowStageFailsAndTheOpenGateIsRead(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if _, err := newPRMerger(windowPlane(t, `{"pr":{"merge_window":{"stage":"maybe"}}}`), prMergeArgs{}, &stdout, &stderr); err == nil || !strings.Contains(err.Error(), "pr.merge_window.stage") {
		t.Errorf("err = %v, want the stage error", err)
	}
	for policyJSON, open := range map[string]bool{windowOn: true, `{"pr":{"merge_window":{"stage":"on"}}}`: false, "": false} {
		m, err := newPRMerger(windowPlane(t, policyJSON), prMergeArgs{}, &stdout, &stderr)
		if err != nil || m.windowOpen != open || m.root == "" {
			t.Errorf("policy %q: windowOpen = %v, err %v; want %v and the root", policyJSON, m.windowOpen, err, open)
		}
	}
}
