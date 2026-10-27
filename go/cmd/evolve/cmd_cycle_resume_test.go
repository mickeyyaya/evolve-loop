package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func pausedCycle(t *testing.T, rp *core.ResumePoint, err error) *[]string {
	t.Helper()
	roots := &[]string{}
	prev := cycleResumeLoad
	t.Cleanup(func() { cycleResumeLoad = prev })
	cycleResumeLoad = func(root string, _ io.Writer) (*core.ResumePoint, error) {
		*roots = append(*roots, root)
		return rp, err
	}
	return roots
}

var pausedAtBuild = &core.ResumePoint{CycleID: 41, Phase: "build", Reason: "quota", StatePath: "/p/.evolve/cycle-state.json"}

func TestCycleResume_LaunchesTheLoopOnThePausedCycle(t *testing.T) {
	root := newLoopStatusRoot(t)
	pausedCycle(t, pausedAtBuild, nil)
	launches := installRecordingLoopLauncher(t)
	var stdout, stderr bytes.Buffer
	if rc := runCycle([]string{"resume", "--cycle", "41", "--project-root", root}, nil, &stdout, &stderr); rc != cliExitOK {
		t.Fatalf("rc = %d stderr = %s", rc, stderr.String())
	}
	if len(*launches) != 1 || !(*launches)[0].Resume || (*launches)[0].ProjectRoot != root {
		t.Fatalf("launches = %+v, want one resume launch on %s", *launches, root)
	}
}

func TestCycleResume_DryRunNamesThePausedCycleAndStartsNothing(t *testing.T) {
	root := newLoopStatusRoot(t)
	roots := pausedCycle(t, pausedAtBuild, nil)
	launches := installRecordingLoopLauncher(t)
	var stdout, stderr bytes.Buffer
	if rc := runCycle([]string{"resume", "--dry-run", "--json", "--project-root", root}, nil, &stdout, &stderr); rc != cliExitOK {
		t.Fatalf("rc = %d stderr = %s", rc, stderr.String())
	}
	var got cycleResumeResult
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not one result: %v\n%s", err, stdout.String())
	}
	want := cycleResumeResult{Cycle: 41, Phase: "build", Reason: "quota", State: "/p/.evolve/cycle-state.json", DryRun: true}
	if got != want || len(*launches) != 0 || len(*roots) != 1 || (*roots)[0] != root {
		t.Errorf("result = %+v launches = %d roots = %v, want %+v and no launch", got, len(*launches), *roots, want)
	}
	stdout.Reset()
	if rc := runCycle([]string{"resume", "--dry-run", "--project-root", root}, nil, &stdout, &stderr); rc != cliExitOK || !strings.Contains(stdout.String(), "cycle resume: would resume cycle 41 at phase build") {
		t.Errorf("text form: rc = %d stdout = %q", rc, stdout.String())
	}
}

func TestCycleResume_EachRefusalHasItsExitCode(t *testing.T) {
	cases := []struct {
		name string
		args []string
		rp   *core.ResumePoint
		err  error
		live bool
		want int
	}{
		{"a stray word", []string{"now"}, pausedAtBuild, nil, false, cliExitUsage},
		{"an unknown flag", []string{"--bogus"}, pausedAtBuild, nil, false, cliExitUsage},
		{"a negative cycle", []string{"--cycle", "-1"}, pausedAtBuild, nil, false, cliExitUsage},
		{"a live run lease", nil, pausedAtBuild, nil, true, cliExitRefused},
		{"no paused cycle", nil, nil, core.ErrNoCheckpoint, false, cliExitFailed},
		{"another paused cycle", []string{"--cycle", "40"}, pausedAtBuild, nil, false, cliExitInput},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := newLoopStatusRoot(t)
			if c.live {
				liveWaveLease(t, root, time.Now())
			}
			pausedCycle(t, c.rp, c.err)
			launches := installRecordingLoopLauncher(t)
			var stdout, stderr bytes.Buffer
			rc := runCycle(append([]string{"resume", "--project-root", root}, c.args...), nil, &stdout, &stderr)
			if rc != c.want || stderr.Len() == 0 || len(*launches) != 0 {
				t.Errorf("rc = %d stderr = %q launches = %d, want %d, a message and no launch", rc, stderr.String(), len(*launches), c.want)
			}
		})
	}
}

func TestCycleResume_ReadsTheCheckpointOfTheRoot(t *testing.T) {
	root := newLoopStatusRoot(t)
	launches := installRecordingLoopLauncher(t)
	var stdout, stderr bytes.Buffer
	rc := runCycle([]string{"resume", "--project-root", root}, nil, &stdout, &stderr)
	if rc != cliExitFailed || !strings.Contains(stderr.String(), "no paused cycle") || len(*launches) != 0 {
		t.Errorf("rc = %d stderr = %q launches = %d, want 1 and no launch for a root with no checkpoint", rc, stderr.String(), len(*launches))
	}
}

func TestCycleResume_AnUnreadableWorkingDirectoryIsAnIOError(t *testing.T) {
	breakWorkingDirectory(t)
	pausedCycle(t, pausedAtBuild, errors.New("unused"))
	var stdout, stderr bytes.Buffer
	if rc := runCycle([]string{"resume"}, nil, &stdout, &stderr); rc != cliExitIO {
		t.Errorf("rc = %d, want 2", rc)
	}
}
