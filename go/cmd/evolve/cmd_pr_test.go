package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ciwatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

func TestParsePRMergeArgs_InterleavesAndRejectsUsage(t *testing.T) {
	a, err := parsePRMergeArgs([]string{"12", "--wait", "5m", "13", "--update-branch", "--project-root=/p"})
	if err != nil || strings.Join(a.prs, ",") != "12,13" || a.wait != 5*time.Minute || !a.updateBranch || a.projectRoot != "/p" {
		t.Fatalf("parsePRMergeArgs = %+v, %v", a, err)
	}
	for _, bad := range [][]string{{}, {"0"}, {"abc"}, {"-3"}, {"12", "12"}, {"12", "--wait", "-1s"}, {"12", "--wait", "soon"}, {"12", "--wait"}, {"12", "--bogus"}} {
		if _, err := parsePRMergeArgs(bad); err == nil {
			t.Errorf("parsePRMergeArgs(%q) must be a usage error", bad)
		}
	}
}

func TestRunPR_UsageExitsTen(t *testing.T) {
	for _, args := range [][]string{nil, {"bogus"}, {"merge"}, {"merge", "0"}} {
		var stdout, stderr bytes.Buffer
		if code := runPR(args, nil, &stdout, &stderr); code != exitUsage || stdout.Len() != 0 || !strings.Contains(stderr.String(), "usage: evolve pr merge") {
			t.Errorf("runPR(%q) = %d, stdout %q, stderr %q", args, code, stdout.String(), stderr.String())
		}
	}
}

type fakePRGitHub struct {
	views    map[string]string
	behind   map[string]string
	merged   map[string]string
	calls    []string
	failCall string
}

func (f *fakePRGitHub) gh(_ context.Context, args ...string) ([]byte, error) {
	call := strings.Join(args, " ")
	f.calls = append(f.calls, call)
	if f.failCall != "" && strings.HasPrefix(call, f.failCall) {
		return nil, errors.New("gh: HTTP 502")
	}
	switch {
	case strings.HasPrefix(call, "pr view") && strings.HasSuffix(call, "state,mergeCommit"):
		return []byte(f.merged[args[2]]), nil
	case strings.HasPrefix(call, "pr view"):
		return []byte(f.views[args[2]]), nil
	case strings.HasPrefix(call, "api "):
		return []byte(f.behind[call[strings.LastIndex(call, "...")+3:]]), nil
	}
	return nil, nil
}

func greenMerger(gh *fakePRGitHub, fetch ciwatch.Fetcher, live []runlease.LiveRun) (prMerger, *bytes.Buffer, *bytes.Buffer) {
	var stdout, stderr bytes.Buffer
	clock := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	return prMerger{
		method: "squash", poll: time.Second, wait: 3 * time.Second, gh: gh.gh, fetch: fetch,
		liveRuns: func() ([]runlease.LiveRun, error) { return live, nil },
		now:      func() time.Time { return clock },
		sleep:    func(d time.Duration) { clock = clock.Add(d) },
		stdout:   &stdout, stderr: &stderr,
	}, &stdout, &stderr
}

const mergeSHA = "0123456789abcdef0123456789abcdef01234567"

func openPR(head string) string {
	return `{"state":"OPEN","isDraft":false,"mergeable":"MERGEABLE","headRefOid":"` + head + `","baseRefName":"main"}`
}

func TestPRMerger_WaitsForGreenThenMergesAtTheVerifiedHead(t *testing.T) {
	gh := &fakePRGitHub{
		views:  map[string]string{"12": openPR("h12")},
		behind: map[string]string{"h12": `{"behind_by":0}`},
		merged: map[string]string{"12": `{"state":"MERGED","mergeCommit":{"oid":"` + mergeSHA + `"}}`},
	}
	polls := 0
	fetch := func(_ context.Context, sha string) (ciwatch.RunStatus, error) {
		polls++
		if polls < 3 {
			return ciwatch.RunStatus{Status: "in_progress"}, nil
		}
		return ciwatch.RunStatus{Status: ciwatch.StatusCompleted, Conclusion: ciwatch.ConclusionSuccess}, nil
	}
	m, stdout, stderr := greenMerger(gh, fetch, nil)
	if code := m.mergeAll(context.Background(), []string{"12"}); code != 0 || stdout.String() != "merged #12 "+mergeSHA+"\n" {
		t.Fatalf("exit %d stdout %q stderr %q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(strings.Join(gh.calls, "\n"), "pr merge 12 --squash --match-head-commit h12") || polls != 3 {
		t.Errorf("want one head-bound squash merge after 3 polls, polls=%d calls:\n%s", polls, strings.Join(gh.calls, "\n"))
	}
}

func TestPRMerger_RefusesAndStops(t *testing.T) {
	green := func(context.Context, string) (ciwatch.RunStatus, error) {
		return ciwatch.RunStatus{Status: ciwatch.StatusCompleted, Conclusion: ciwatch.ConclusionSuccess}, nil
	}
	red := func(context.Context, string) (ciwatch.RunStatus, error) {
		return ciwatch.RunStatus{Status: ciwatch.StatusCompleted, Conclusion: "failure", FailingJobs: []string{"test (ubuntu)"}}, nil
	}
	pending := func(context.Context, string) (ciwatch.RunStatus, error) {
		return ciwatch.RunStatus{Status: "queued"}, nil
	}
	cases := []struct {
		name, view, behind, want string
		fetch                    ciwatch.Fetcher
		live                     []runlease.LiveRun
		code                     int
	}{
		{"live lease", openPR("h12"), `{"behind_by":0}`, "run cycle-9", green, []runlease.LiveRun{{Dir: "/r/cycle-9", Lease: runlease.Lease{RunID: "cycle-9"}}}, exitRefused},
		{"draft", `{"state":"OPEN","isDraft":true,"headRefOid":"h12"}`, `{"behind_by":0}`, "draft", green, nil, exitRefused},
		{"closed", `{"state":"CLOSED","headRefOid":"h12"}`, `{"behind_by":0}`, "CLOSED", green, nil, exitRefused},
		{"conflicting", `{"state":"OPEN","mergeable":"CONFLICTING","headRefOid":"h12","baseRefName":"main"}`, `{"behind_by":0}`, "conflicts", green, nil, exitRefused},
		{"behind without --update-branch", openPR("h12"), `{"behind_by":2}`, "--update-branch", green, nil, exitRefused},
		{"red names the job", openPR("h12"), `{"behind_by":0}`, "test (ubuntu)", red, nil, exitRefused},
		{"pending past the deadline", openPR("h12"), `{"behind_by":0}`, "pending", pending, nil, exitRefused},
		{"unparsable gh JSON", `not json`, `{"behind_by":0}`, "decode", green, nil, exitIO},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gh := &fakePRGitHub{views: map[string]string{"12": c.view}, behind: map[string]string{"h12": c.behind}}
			m, stdout, stderr := greenMerger(gh, c.fetch, c.live)
			code := m.mergeAll(context.Background(), []string{"12", "13"})
			if code != c.code || stdout.Len() != 0 || !strings.Contains(stderr.String(), c.want) || !strings.Contains(stderr.String(), "not attempted:") || !strings.HasSuffix(stderr.String(), " #13\n") {
				t.Errorf("exit %d stdout %q stderr %q; want exit %d naming %q", code, stdout.String(), stderr.String(), c.code, c.want)
			}
			for _, call := range gh.calls {
				if strings.HasPrefix(call, "pr merge") || strings.HasPrefix(call, "pr view 13") {
					t.Errorf("a refusal must merge nothing and touch no later PR: %s", call)
				}
			}
		})
	}
}

func TestPRMerger_GHFailuresExitTwo(t *testing.T) {
	green := func(context.Context, string) (ciwatch.RunStatus, error) {
		return ciwatch.RunStatus{Status: ciwatch.StatusCompleted, Conclusion: ciwatch.ConclusionSuccess}, nil
	}
	for _, c := range []struct{ name, failCall, merged string }{
		{"merge rejected", "pr merge", ""},
		{"merge never recorded", "", `{"state":"OPEN","mergeCommit":null}`},
	} {
		t.Run(c.name, func(t *testing.T) {
			gh := &fakePRGitHub{views: map[string]string{"12": openPR("h12")}, behind: map[string]string{"h12": `{"behind_by":0}`},
				merged: map[string]string{"12": c.merged}, failCall: c.failCall}
			m, stdout, _ := greenMerger(gh, green, nil)
			if code := m.mergeAll(context.Background(), []string{"12"}); code != exitIO || stdout.Len() != 0 {
				t.Errorf("exit %d stdout %q, want exit 2 and no merged line", code, stdout.String())
			}
		})
	}
	m, _, stderr := greenMerger(&fakePRGitHub{}, green, nil)
	m.liveRuns = func() ([]runlease.LiveRun, error) { return nil, errors.New("git worktree list: exit 128") }
	if code := m.mergeAll(context.Background(), []string{"12"}); code != exitIO || !strings.Contains(stderr.String(), "git worktree list") {
		t.Errorf("an unprovable idle plane must exit 2, got %d: %s", code, stderr.String())
	}
}

func TestPlaneLiveRuns_SeesSiblingWorktreesOnly(t *testing.T) {
	repo := gittest.Fixture(t)
	if err := os.WriteFile(filepath.Join(repo.Dir, "README"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	sibling := filepath.Join(t.TempDir(), "sibling")
	repo.Git("worktree", "add", "-q", "-b", "sibling", sibling)
	now := time.Now()
	for _, dir := range []string{filepath.Join(sibling, ".evolve", "runs", "cycle-7"), filepath.Join(t.TempDir(), ".evolve", "runs", "cycle-8")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := runlease.Write(dir, runlease.Lease{RunID: filepath.Base(dir), OwnerPID: os.Getpid()}, now); err != nil {
			t.Fatal(err)
		}
	}
	live, err := planeLiveRuns(repo.Dir, now)
	if err != nil || len(live) != 1 || live[0].Lease.RunID != "cycle-7" {
		t.Fatalf("planeLiveRuns = %+v, %v; want only the sibling worktree's cycle-7", live, err)
	}
	if _, err := planeLiveRuns(t.TempDir(), now); err == nil {
		t.Error("a root that is no git repository cannot prove the plane idle")
	}
}
