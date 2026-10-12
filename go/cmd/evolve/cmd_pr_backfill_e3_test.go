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
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

type e3GHReply struct {
	out string
	err error
}

type e3ScriptedGH struct {
	script map[string][]e3GHReply
	calls  []string
}

func (s *e3ScriptedGH) gh(_ context.Context, args ...string) ([]byte, error) {
	call := strings.Join(args, " ")
	s.calls = append(s.calls, call)
	for prefix, replies := range s.script {
		if !strings.HasPrefix(call, prefix) || len(replies) == 0 {
			continue
		}
		r := replies[0]
		s.script[prefix] = replies[1:]
		if r.err != nil {
			return nil, r.err
		}
		return []byte(r.out), nil
	}
	return nil, errors.New("unscripted gh call: " + call)
}

func e3Green(context.Context, string) (ciwatch.RunStatus, error) {
	return ciwatch.RunStatus{Status: ciwatch.StatusCompleted, Conclusion: ciwatch.ConclusionSuccess}, nil
}

func e3Merger(gh *e3ScriptedGH, fetch ciwatch.Fetcher, live func() ([]runlease.LiveRun, error)) (prMerger, *bytes.Buffer, *bytes.Buffer) {
	m, stdout, stderr := greenMerger(&fakePRGitHub{}, fetch, nil)
	m.gh = gh.gh
	m.liveRuns = live
	return m, stdout, stderr
}

func e3Idle() ([]runlease.LiveRun, error) { return nil, nil }

const (
	e3Head   = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	e3Head2  = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	e3Merged = `{"state":"MERGED","mergeCommit":{"oid":"` + mergeSHA + `"}}`
)

func e3Behind(n string) e3GHReply { return e3GHReply{out: `{"behind_by":` + n + `}`} }

func TestPRMerger_MergeOneFailurePaths(t *testing.T) {
	boom := errors.New("gh: HTTP 502")
	cases := []struct {
		name      string
		script    map[string][]e3GHReply
		fetch     ciwatch.Fetcher
		live      func() ([]runlease.LiveRun, error)
		wantCode  int
		wantErr   string
		wantMerge bool
	}{
		{
			name:     "view fails",
			script:   map[string][]e3GHReply{"pr view": {{err: boom}}},
			fetch:    e3Green,
			live:     e3Idle,
			wantCode: exitIO, wantErr: "#12 gh: HTTP 502",
		},
		{
			name:     "first behind check fails",
			script:   map[string][]e3GHReply{"pr view": {{out: openPR(e3Head)}}, "api ": {{err: boom}}},
			fetch:    e3Green,
			live:     e3Idle,
			wantCode: exitIO, wantErr: "#12 gh: HTTP 502",
		},
		{
			name:   "CI fetch fails",
			script: map[string][]e3GHReply{"pr view": {{out: openPR(e3Head)}}, "api ": {e3Behind("0")}},
			fetch: func(context.Context, string) (ciwatch.RunStatus, error) {
				return ciwatch.RunStatus{}, errors.New("gh run list: rate limited")
			},
			live:     e3Idle,
			wantCode: exitIO, wantErr: "#12 gh run list: rate limited",
		},
		{
			name:     "second behind check fails after green",
			script:   map[string][]e3GHReply{"pr view": {{out: openPR(e3Head)}}, "api ": {e3Behind("0"), {err: boom}}},
			fetch:    e3Green,
			live:     e3Idle,
			wantCode: exitIO, wantErr: "#12 gh: HTTP 502",
		},
		{
			name:     "base moved during the wait",
			script:   map[string][]e3GHReply{"pr view": {{out: openPR(e3Head)}}, "api ": {e3Behind("0"), e3Behind("3")}},
			fetch:    e3Green,
			live:     e3Idle,
			wantCode: exitRefused, wantErr: "#12 refused: base moved during the wait: aaaaaaaaaaaa is behind main by 3",
		},
		{
			name:   "a loop started during the wait",
			script: map[string][]e3GHReply{"pr view": {{out: openPR(e3Head)}}, "api ": {e3Behind("0"), e3Behind("0")}},
			fetch:  e3Green,
			live: func() func() ([]runlease.LiveRun, error) {
				calls := 0
				return func() ([]runlease.LiveRun, error) {
					calls++
					if calls == 1 {
						return nil, nil
					}
					return []runlease.LiveRun{{Dir: "/r/cycle-4", Lease: runlease.Lease{RunID: "cycle-4"}}}, nil
				}
			}(),
			wantCode: exitRefused, wantErr: "#12 refused: a loop is running: run cycle-4 (/r/cycle-4) is live",
		},
		{
			name: "merged-commit lookup fails",
			script: map[string][]e3GHReply{
				"pr view 12 --json state,mergeCommit": {{err: boom}},
				"pr view 12 --json number":            {{out: openPR(e3Head)}},
				"api ":                                {e3Behind("0"), e3Behind("0")},
				"pr merge":                            {{out: ""}},
			},
			fetch:    e3Green,
			live:     e3Idle,
			wantCode: exitIO, wantErr: "#12 gh: HTTP 502", wantMerge: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gh := &e3ScriptedGH{script: tc.script}
			m, stdout, stderr := e3Merger(gh, tc.fetch, tc.live)

			code := m.mergeAll(context.Background(), []string{"12", "13"})

			if code != tc.wantCode || stdout.Len() != 0 {
				t.Fatalf("code %d stdout %q stderr %q, want code %d", code, stdout.String(), stderr.String(), tc.wantCode)
			}
			if !strings.Contains(stderr.String(), prMergePrefix+tc.wantErr) || !strings.HasSuffix(stderr.String(), "not attempted: #13\n") {
				t.Errorf("stderr = %q, want %q and #13 not attempted", stderr.String(), tc.wantErr)
			}
			merged := strings.Contains(strings.Join(gh.calls, "\n"), "pr merge")
			if merged != tc.wantMerge {
				t.Errorf("merge attempted = %v, want %v; calls %q", merged, tc.wantMerge, gh.calls)
			}
		})
	}
}

func TestPRMerger_UpdateBranchRefreshesTheHeadBeforeTheMerge(t *testing.T) {
	gh := &e3ScriptedGH{script: map[string][]e3GHReply{
		"pr view 12 --json number":            {{out: openPR(e3Head)}, {out: openPR(e3Head2)}},
		"api ":                                {e3Behind("2"), e3Behind("0")},
		"pr update-branch 12":                 {{out: ""}},
		"pr merge":                            {{out: ""}},
		"pr view 12 --json state,mergeCommit": {{out: e3Merged}},
	}}
	var fetched []string
	fetch := func(_ context.Context, sha string) (ciwatch.RunStatus, error) {
		fetched = append(fetched, sha)
		return e3Green(nil, sha)
	}
	m, stdout, stderr := e3Merger(gh, fetch, e3Idle)
	m.updateBranch = true

	code := m.mergeAll(context.Background(), []string{"12"})

	if code != 0 || stdout.String() != "merged #12 "+mergeSHA+"\n" {
		t.Fatalf("code %d stdout %q stderr %q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), prMergePrefix+"#12 updated with main\n") {
		t.Errorf("stderr = %q, want the update notice", stderr.String())
	}
	if len(fetched) != 1 || fetched[0] != e3Head2 {
		t.Errorf("CI was awaited on %v, want only the updated head %s", fetched, e3Head2)
	}
	if !strings.Contains(strings.Join(gh.calls, "\n"), "pr merge 12 --squash --match-head-commit "+e3Head2) {
		t.Errorf("the merge is not bound to the updated head: %q", gh.calls)
	}
}

func TestPRMerger_AFailedUpdateBranchStopsBeforeCI(t *testing.T) {
	gh := &e3ScriptedGH{script: map[string][]e3GHReply{
		"pr view 12":          {{out: openPR(e3Head)}},
		"api ":                {e3Behind("1")},
		"pr update-branch 12": {{err: errors.New("gh: merge conflict")}},
	}}
	fetched := false
	fetch := func(context.Context, string) (ciwatch.RunStatus, error) {
		fetched = true
		return ciwatch.RunStatus{}, nil
	}
	m, stdout, stderr := e3Merger(gh, fetch, e3Idle)
	m.updateBranch = true

	code := m.mergeAll(context.Background(), []string{"12"})

	if code != exitIO || stdout.Len() != 0 || !strings.Contains(stderr.String(), "#12 gh: merge conflict") {
		t.Fatalf("code %d stdout %q stderr %q", code, stdout.String(), stderr.String())
	}
	if fetched || strings.Contains(stderr.String(), "updated with") {
		t.Errorf("a failed update-branch must not claim an update or wait on CI: %q", stderr.String())
	}
}

func TestFailingWhat_PrefersJobsThenTestThenTheRunURL(t *testing.T) {
	cases := []struct {
		name string
		st   ciwatch.RunStatus
		want string
	}{
		{"jobs win", ciwatch.RunStatus{FailingJobs: []string{"lint", "test (ubuntu)"}, FailingTest: "TestX", RunURL: "u"}, "lint, test (ubuntu)"},
		{"single job", ciwatch.RunStatus{FailingJobs: []string{"lint"}}, "lint"},
		{"test when no jobs", ciwatch.RunStatus{FailingTest: "TestÜnicode", RunURL: "u"}, "TestÜnicode"},
		{"url as the last resort", ciwatch.RunStatus{RunURL: "https://github.com/o/r/actions/runs/9"}, "https://github.com/o/r/actions/runs/9"},
		{"nothing known", ciwatch.RunStatus{}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := failingWhat(tc.st); got != tc.want {
				t.Errorf("failingWhat = %q, want %q", got, tc.want)
			}
		})
	}
}

func e3PolicyRoot(t *testing.T, policyJSON string) string {
	t.Helper()
	root := t.TempDir()
	if policyJSON == "" {
		return root
	}
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(policyJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestNewPRMerger_RefusesAnInvalidPolicy(t *testing.T) {
	cases := []struct{ name, policy, want string }{
		{"malformed", "{", "policy"},
		{"bad merge method", `{"pr":{"merge_method":"fast-forward"}}`, `pr.merge_method must be merge, squash or rebase, got "fast-forward"`},
		{"zero poll", `{"ci_watch":{"poll_s":0}}`, "ci_watch.poll_s must be > 0, got 0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, err := newPRMerger(e3PolicyRoot(t, tc.policy), prMergeArgs{}, e3Buf(), e3Buf())
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
			if m.gh != nil || m.method != "" {
				t.Errorf("a refused policy still built a merger: %+v", m)
			}
		})
	}
}

func e3Buf() *bytes.Buffer { return &bytes.Buffer{} }

func TestNewPRMerger_TakesMethodAndPollFromPolicyAndWaitFromArgs(t *testing.T) {
	cases := []struct {
		name, policy string
		wantMethod   string
		wantPoll     time.Duration
	}{
		{"defaults", "", "merge", 30 * time.Second},
		{"declared", `{"pr":{"merge_method":"rebase"},"ci_watch":{"poll_s":7}}`, "rebase", 7 * time.Second},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, err := newPRMerger(e3PolicyRoot(t, tc.policy), prMergeArgs{wait: 90 * time.Second, updateBranch: true}, e3Buf(), e3Buf())
			if err != nil {
				t.Fatalf("newPRMerger: %v", err)
			}
			if m.method != tc.wantMethod || m.poll != tc.wantPoll || m.wait != 90*time.Second || !m.updateBranch {
				t.Errorf("merger = method %q poll %v wait %v update %v", m.method, m.poll, m.wait, m.updateBranch)
			}
			if m.gh == nil || m.fetch == nil || m.liveRuns == nil || m.now == nil || m.sleep == nil {
				t.Errorf("merger is missing a production dependency: %+v", m)
			}
		})
	}
}

func TestRunPR_AnInvalidPolicyExitsTwo(t *testing.T) {
	root := e3PolicyRoot(t, `{"pr":{"merge_method":"octopus"}}`)
	var stdout, stderr bytes.Buffer

	code := runPR([]string{"merge", "12", "--project-root", root}, nil, &stdout, &stderr)

	if code != exitIO || stdout.Len() != 0 || !strings.HasPrefix(stderr.String(), prMergePrefix+"policy: pr.merge_method") {
		t.Fatalf("code %d stdout %q stderr %q", code, stdout.String(), stderr.String())
	}
}

func TestRunPR_AnUnresolvableWorkingDirectoryExitsTwo(t *testing.T) {
	prev := processGetwd
	t.Cleanup(func() { processGetwd = prev })
	processGetwd = func() (string, error) { return "", errors.New("getwd: no such file or directory") }
	t.Setenv("EVOLVE_PROJECT_ROOT", "")
	var stdout, stderr bytes.Buffer

	code := runPR([]string{"merge", "12"}, nil, &stdout, &stderr)

	if code != exitIO || stderr.String() != prMergePrefix+"getwd: no such file or directory\n" {
		t.Fatalf("code %d stderr %q", code, stderr.String())
	}
}

func TestRunPR_ARootThatIsNoRepositoryCannotProveThePlaneIdle(t *testing.T) {
	root := e3PolicyRoot(t, "")
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(root))
	var stdout, stderr bytes.Buffer

	code := runPR([]string{"merge", "12", "13", "--project-root", root}, nil, &stdout, &stderr)

	if code != exitIO || stdout.Len() != 0 {
		t.Fatalf("code %d stdout %q stderr %q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), prMergePrefix+"git worktree list in ") || !strings.HasSuffix(stderr.String(), "not attempted: #12 #13\n") {
		t.Errorf("stderr = %q", stderr.String())
	}
}
