package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/wave"
)

var boundaryClock = time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)

type fakeBoundaryGH struct {
	prs      string
	runs     map[string]string
	files    map[string]string
	failCall string
	calls    []string
}

func (f *fakeBoundaryGH) gh(_ context.Context, args ...string) ([]byte, error) {
	call := strings.Join(args, " ")
	f.calls = append(f.calls, call)
	if f.failCall != "" && strings.HasPrefix(call, f.failCall) {
		return nil, errors.New("gh: HTTP 502")
	}
	switch {
	case strings.HasPrefix(call, "pr list"):
		return []byte(f.prs), nil
	case strings.HasPrefix(call, "run list"):
		return []byte(f.runs[args[5]]), nil
	case strings.HasPrefix(call, "api --paginate"):
		return []byte(f.files[strings.Split(args[2], "/")[4]]), nil
	}
	return nil, errors.New("fake gh: unexpected " + call)
}

func greenRun(ago time.Duration) string {
	return `[{"status":"completed","conclusion":"success","updatedAt":"` + boundaryClock.Add(-ago).Format(time.RFC3339) + `"}]`
}

func boundaryGH() *fakeBoundaryGH {
	return &fakeBoundaryGH{
		prs: `[{"number":20,"isDraft":false,"headRefOid":"h20"},{"number":21,"isDraft":false,"headRefOid":"h21"},` +
			`{"number":22,"isDraft":true,"headRefOid":"h22"},{"number":23,"isDraft":false,"headRefOid":"h23"},{"number":24,"isDraft":false,"headRefOid":"h24"}]`,
		runs: map[string]string{
			"h20": greenRun(150 * time.Minute), "h21": greenRun(90 * time.Minute), "h22": greenRun(5 * time.Hour),
			"h23": `[{"status":"completed","conclusion":"failure","updatedAt":"2026-10-20T01:00:00Z"}]`, "h24": `[]`,
		},
		files: map[string]string{"20": "\"docs/a.md\"\n", "21": "\"go/x.go\"\n"},
	}
}

func boundaryPlaneOf(gh *fakeBoundaryGH, live []runlease.LiveRun, pool int, poolKnown bool) func(string) boundaryPlane {
	return func(string) boundaryPlane {
		return boundaryPlane{
			gh:       gh.gh,
			liveRuns: func() ([]runlease.LiveRun, error) { return live, nil },
			poolLive: func() (int, bool, error) { return pool, poolKnown, nil },
			now:      func() time.Time { return boundaryClock },
		}
	}
}

func runPRBoundaryT(t *testing.T, plane func(string) boundaryPlane, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := runPRBoundaryWith(plane, args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

const boundaryPolicy = `{"fleet":{"count":4},"pr":{"merge_window":{"early_boundary_after_s":3600}}}`

func TestPRBoundary_TheLongestGreenWaitPastTheBoundWithAnUnderfilledPoolCallsAnEarlyBoundary(t *testing.T) {
	t.Parallel()
	root := windowPlane(t, boundaryPolicy)
	code, stdout, stderr := runPRBoundaryT(t, boundaryPlaneOf(boundaryGH(), liveLoop, 2, true), "--project-root", root)
	want := "early boundary: #20 waited 2h30m0s (bound 1h0m0s); pool 2/4 lanes: run evolve loop-stop --wait, then evolve wave next --merge 20\n"
	if code != 0 || stdout != want || stderr != "" {
		t.Fatalf("exit %d stdout %q stderr %q; want 0 and %q", code, stdout, stderr, want)
	}
}

func TestPRBoundary_AWindowPRDoesNotWaitAndTheBoundComesFromPolicy(t *testing.T) {
	t.Parallel()
	root := windowPlane(t, `{"fleet":{"count":4,"integration":{"stage":"enforce"}},"pr":{"merge_window":{"stage":"on","early_boundary_after_s":7200}}}`)
	code, stdout, _ := runPRBoundaryT(t, boundaryPlaneOf(boundaryGH(), liveLoop, 2, true), "--json", "--project-root", root)
	var got prBoundaryReport
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode %q: %v", stdout, err)
	}
	want := prBoundaryReport{LoopLive: true, PR: "21", WaitS: 5400, BoundS: 7200, PoolLive: 2, PoolWidth: 4,
		Line: "no early boundary: #21 waited 1h30m0s (bound 2h0m0s); pool 2/4 lanes"}
	if code != exitBoundary || got != want {
		t.Fatalf("exit %d report %+v; want %d and %+v (the docs PR #20 merges in the window)", code, got, exitBoundary, want)
	}
}

func TestPRBoundary_NoLiveLoopIsAlreadyABoundary(t *testing.T) {
	t.Parallel()
	gh := boundaryGH()
	code, stdout, _ := runPRBoundaryT(t, boundaryPlaneOf(gh, nil, 0, false), "--project-root", windowPlane(t, boundaryPolicy))
	if code != exitBoundary || stdout != reasonNoLoop+"\n" || len(gh.calls) != 0 {
		t.Fatalf("exit %d stdout %q calls %v; want the no-loop line and no gh call", code, stdout, gh.calls)
	}
}

func TestPRBoundary_FailsLoudOnWhatItCannotRead(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, policyJSON, failCall, want string
		poolKnown                        bool
	}{
		{"an unknown pool", boundaryPolicy, "", "no pool signal for the live wave", false},
		{"a malformed policy", `{"pr":`, "", "policy: parse", true},
		{"a bad bound", `{"pr":{"merge_window":{"early_boundary_after_s":0}}}`, "", "early_boundary_after_s must be > 0", true},
		{"a bad stage", `{"pr":{"merge_window":{"stage":"maybe"}}}`, "", "pr.merge_window.stage must be off or on", true},
		{"the PR list", boundaryPolicy, "pr list", "gh: HTTP 502", true},
		{"a run list", boundaryPolicy, "run list", "gh: HTTP 502", true},
		{"a file list", boundaryPolicy, "api --paginate", "gh: HTTP 502", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gh := boundaryGH()
			gh.failCall = c.failCall
			code, stdout, stderr := runPRBoundaryT(t, boundaryPlaneOf(gh, liveLoop, 2, c.poolKnown), "--project-root", windowPlane(t, c.policyJSON))
			if code != exitIO || stdout != "" || !strings.HasPrefix(stderr, prBoundaryPrefix) || !strings.Contains(stderr, c.want) {
				t.Errorf("exit %d stdout %q stderr %q; want exitIO naming %q", code, stdout, stderr, c.want)
			}
		})
	}
	for name, body := range map[string]string{"PR list": "not json", "run list": `[{"status":`} {
		gh := boundaryGH()
		if name == "PR list" {
			gh.prs = body
		} else {
			gh.runs["h20"] = body
		}
		if code, _, stderr := runPRBoundaryT(t, boundaryPlaneOf(gh, liveLoop, 2, true), "--project-root", windowPlane(t, boundaryPolicy)); code != exitIO || !strings.Contains(stderr, "decode") {
			t.Errorf("an undecodable %s: exit %d stderr %q", name, code, stderr)
		}
	}
	plane := func(string) boundaryPlane {
		p := boundaryPlaneOf(boundaryGH(), liveLoop, 2, true)("")
		p.liveRuns = func() ([]runlease.LiveRun, error) { return nil, errors.New("git worktree list: exit 128") }
		return p
	}
	if code, _, stderr := runPRBoundaryT(t, plane, "--project-root", windowPlane(t, boundaryPolicy)); code != exitIO || !strings.Contains(stderr, "git worktree list") {
		t.Errorf("an unprovable plane: exit %d stderr %q", code, stderr)
	}
	plane = func(string) boundaryPlane {
		p := boundaryPlaneOf(boundaryGH(), liveLoop, 2, true)("")
		p.poolLive = func() (int, bool, error) { return 0, false, errors.New("waves: read record") }
		return p
	}
	if code, _, stderr := runPRBoundaryT(t, plane, "--project-root", windowPlane(t, boundaryPolicy)); code != exitIO || !strings.Contains(stderr, "waves: read record") {
		t.Errorf("an unreadable wave record: exit %d stderr %q", code, stderr)
	}
}

func TestPRBoundary_UsageAndAnAbsentPlane(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"--bogus"}, {"12"}, {"--project-root"}} {
		var stdout, stderr bytes.Buffer
		if code := runPR(append([]string{"boundary"}, args...), nil, &stdout, &stderr); code != exitUsage || !strings.Contains(stderr.String(), prBoundaryUsage) {
			t.Errorf("runPR boundary %q = %d, stderr %q", args, code, stderr.String())
		}
	}
	var stdout, stderr bytes.Buffer
	if code := runPR(nil, nil, &stdout, &stderr); code != exitUsage || !strings.Contains(stderr.String(), prBoundaryUsage) {
		t.Errorf("the pr usage must name the boundary verb: %q", stderr.String())
	}
	if code, _, stderr := runPRBoundaryT(t, boundaryPlaneOf(boundaryGH(), liveLoop, 2, true), "--project-root", filepath.Join(t.TempDir(), "absent")); code != exitIO || stderr == "" {
		t.Errorf("an absent root: exit %d stderr %q", code, stderr)
	}
}

func TestDefaultBoundaryPlane_ReadsThePoolOfTheLastRecordedWave(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	plane := defaultBoundaryPlane(root)
	if live, known, err := plane.poolLive(); err != nil || known || live != 0 {
		t.Fatalf("no wave record: poolLive = %d, %v, %v; want unknown", live, known, err)
	}
	if err := wave.NewStore(filepath.Join(root, ".evolve")).Save(wave.Record{Number: 1, RunID: "run-1", StartedAt: boundaryClock}); err != nil {
		t.Fatal(err)
	}
	if live, known, err := plane.poolLive(); err != nil || known || live != 0 {
		t.Errorf("a record with no pool signal: poolLive = %d, %v, %v; want unknown", live, known, err)
	}
	if err := os.Mkdir(filepath.Join(root, ".evolve", signalcenter.StreamFileName), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, known, err := plane.poolLive(); err == nil || known || !strings.Contains(err.Error(), "read the loop signals") {
		t.Errorf("an unreadable signal stream: known %v, err %v; want the read error", known, err)
	}
	if _, err := plane.liveRuns(); err == nil {
		t.Error("a root that is no git checkout cannot prove the plane idle")
	}
	if plane.gh == nil || plane.now == nil {
		t.Error("the default plane must wire gh and the clock")
	}
}

func TestPRBoundary_AnUnreadableWorkingDirectoryIsAnIOError(t *testing.T) {
	breakWorkingDirectory(t)
	gh := boundaryGH()
	code, stdout, stderr := runPRBoundaryT(t, boundaryPlaneOf(gh, liveLoop, 2, true))
	if code != exitIO || stdout != "" || !strings.Contains(stderr, prBoundaryPrefix+"getwd: no such file or directory") || len(gh.calls) != 0 {
		t.Errorf("exit %d stdout %q stderr %q calls %v; want exitIO and the getwd error", code, stdout, stderr, gh.calls)
	}
}
