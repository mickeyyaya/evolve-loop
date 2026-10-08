package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
)

type fakeBoundaryCall struct {
	verb string
	args []string
}

type fakeBoundaryVerbs struct {
	calls  []fakeBoundaryCall
	failAt int
	code   int
}

const (
	fakeBoundaryPidLine     = "loop: detached pid 4242, log /plane/.evolve/loop.log"
	fakeBoundaryVerdictLine = "loop: running — pid 4242, run r-4242 (cycle-9001), log /plane/.evolve/loop.log"
	fakeBoundaryGoal        = "drain the console queue"
)

func (f *fakeBoundaryVerbs) dispatch(verb string, args []string, stdout, stderr io.Writer) int {
	f.calls = append(f.calls, fakeBoundaryCall{verb: verb, args: slices.Clone(args)})
	n := len(f.calls)
	fmt.Fprintf(stdout, "fake-step %d %s\n", n, verb)
	if verb == "loop" {
		fmt.Fprintln(stdout, fakeBoundaryPidLine)
		fmt.Fprintln(stdout, fakeBoundaryVerdictLine)
	}
	if n == f.failAt {
		fmt.Fprintf(stderr, "fake %s failed rc=%d\n", verb, f.code)
		return f.code
	}
	return 0
}

func (f *fakeBoundaryVerbs) verbs() []string {
	out := make([]string, 0, len(f.calls))
	for _, c := range f.calls {
		out = append(out, c.verb)
	}
	return out
}

func fakeBoundaryGoalFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "goal.txt")
	if err := os.WriteFile(path, []byte(fakeBoundaryGoal+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func fakeBoundaryHasFlagValue(args []string, name, value string) bool {
	for i, a := range args {
		if a == name+"="+value || (a == name && i+1 < len(args) && args[i+1] == value) {
			return true
		}
	}
	return false
}

func boundaryFlagValue(args []string, name string) string {
	for i, a := range args {
		if a == name && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func TestBoundaryRun_StopsAtFirstFailedStepWithItsExitCode(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		failAt    int
		code      int
		wantVerbs []string
	}{
		{"loop-stop --wait times out", 1, exitRefused, []string{"loop-stop"}},
		{"pr merge refused", 2, exitRefused, []string{"loop-stop", "pr"}},
		{"pr merge I/O failure", 2, exitIO, []string{"loop-stop", "pr"}},
		{"sync-main refused", 3, exitRefused, []string{"loop-stop", "pr", "sync-main"}},
		{"gc I/O failure", 4, exitIO, []string{"loop-stop", "pr", "sync-main", "gc"}},
		{"brake release fails", 5, exitRefused, []string{"loop-stop", "pr", "sync-main", "gc", "loop-stop"}},
		{"log dir switch fails", 6, exitIO, []string{"loop-stop", "pr", "sync-main", "gc", "loop-stop", boundaryLogVerb}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			fake := &fakeBoundaryVerbs{failAt: c.failAt, code: c.code}
			var stdout, stderr bytes.Buffer
			rc := runBoundaryWith(fake.dispatch, []string{"run", "--merge", "12", "--goal-text-file", fakeBoundaryGoalFile(t)}, &stdout, &stderr)
			if rc != c.code {
				t.Errorf("rc = %d, want the failed step's own exit code %d\n%s%s", rc, c.code, stdout.String(), stderr.String())
			}
			if got := fake.verbs(); !slices.Equal(got, c.wantVerbs) {
				t.Errorf("dispatched %v, want exactly %v: the run must stop at the first failure", got, c.wantVerbs)
			}
			if strings.Contains(stdout.String(), fakeBoundaryPidLine) {
				t.Errorf("a loop was launched after a failed step\n%s", stdout.String())
			}
		})
	}
}

func TestBoundaryRun_FullFakeRunEndsWithDetachedPidAndBootVerdict(t *testing.T) {
	t.Parallel()
	fake := &fakeBoundaryVerbs{}
	var stdout, stderr bytes.Buffer
	rc := runBoundaryWith(fake.dispatch, []string{"run", "--merge", "12,15", "--goal-text-file", fakeBoundaryGoalFile(t), "--max-cycles", "3"}, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc = %d, want 0 for a run whose every step succeeds\n%s%s", rc, stdout.String(), stderr.String())
	}
	want := []string{"loop-stop", "pr", "sync-main", "gc", "loop-stop", boundaryLogVerb, "loop"}
	if got := fake.verbs(); !slices.Equal(got, want) {
		t.Fatalf("dispatched %v, want the seven steps %v in order", got, want)
	}
	stop, merge, release, logStep, launch := fake.calls[0].args, fake.calls[1].args, fake.calls[4].args, fake.calls[5].args, fake.calls[6].args
	if !slices.Contains(stop, "--wait") || slices.Contains(stop, "--release") {
		t.Errorf("first loop-stop args %v: want --wait and no --release", stop)
	}
	if len(merge) == 0 || merge[0] != "merge" || !slices.Contains(merge, "12") || !slices.Contains(merge, "15") {
		t.Errorf("pr args %v: want one `merge 12 15` call for the listed PRs", merge)
	}
	if !slices.Contains(release, "--release") || slices.Contains(release, "--wait") {
		t.Errorf("second loop-stop args %v: want --release and no --wait", release)
	}
	runID := boundaryFlagValue(logStep, "--run-id")
	if !gcpolicy.IsLogRunDir(runID) {
		t.Fatalf("%s args %v: want a --run-id that the loop log catalog matches", boundaryLogVerb, logStep)
	}
	root := boundaryFlagValue(logStep, "--project-root")
	wantLog := filepath.Join(root, ".evolve", gcpolicy.LogsDir, runID, gcpolicy.LoopLogName)
	if !slices.Contains(launch, "--detach") || !fakeBoundaryHasFlagValue(launch, "--log", wantLog) {
		t.Errorf("loop args %v: want --detach with --log %s, the new log dir of this launch", launch, wantLog)
	}
	if !strings.Contains(strings.Join(launch, "\x00"), fakeBoundaryGoal) {
		t.Errorf("loop args %v: want the goal text read from --goal-text-file", launch)
	}
	if !fakeBoundaryHasFlagValue(launch, "--max-cycles", "3") {
		t.Errorf("loop args %v: want --max-cycles 3 passed through", launch)
	}
	out := stdout.String()
	pid, verdict := strings.Index(out, fakeBoundaryPidLine), strings.Index(out, fakeBoundaryVerdictLine)
	if pid < 0 || verdict < pid {
		t.Fatalf("the run must report the detached loop's pid and then its boot verdict\n%s", out)
	}
	if strings.Contains(out[pid:], "fake-step") {
		t.Errorf("another step's output follows the detached loop's pid: the run must end with the launch\n%s", out)
	}
}

func TestBoundaryRun_DryRunDispatchesNoStep(t *testing.T) {
	t.Parallel()
	fake := &fakeBoundaryVerbs{}
	var stdout, stderr bytes.Buffer
	rc := runBoundaryWith(fake.dispatch, []string{"run", "--dry-run", "--merge", "12", "--goal-text-file", fakeBoundaryGoalFile(t)}, &stdout, &stderr)
	if rc != 0 {
		t.Fatalf("rc = %d, want 0 for a dry run\n%s%s", rc, stdout.String(), stderr.String())
	}
	if len(fake.calls) != 0 {
		t.Errorf("a dry run dispatched %v; it must only print the plan", fake.verbs())
	}
	for _, step := range []string{"loop-stop", "sync-main", "--release", "--detach", ".evolve/logs/current"} {
		if !strings.Contains(stdout.String(), step) {
			t.Errorf("dry-run plan does not mention %q\n%s", step, stdout.String())
		}
	}
}

func TestBoundaryRun_BadArgumentsDispatchNoStep(t *testing.T) {
	t.Parallel()
	goal := fakeBoundaryGoalFile(t)
	cases := []struct {
		name string
		args []string
	}{
		{"no sub-verb", nil},
		{"unknown sub-verb", []string{"walk", "--goal-text-file", goal}},
		{"missing --goal-text-file", []string{"run", "--merge", "12"}},
		{"goal text file absent", []string{"run", "--merge", "12", "--goal-text-file", filepath.Join(t.TempDir(), "absent.txt")}},
		{"non-numeric PR", []string{"run", "--merge", "12,x", "--goal-text-file", goal}},
		{"zero PR", []string{"run", "--merge", "0", "--goal-text-file", goal}},
		{"unknown flag", []string{"run", "--bogus", "--goal-text-file", goal}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			fake := &fakeBoundaryVerbs{}
			var stdout, stderr bytes.Buffer
			if rc := runBoundaryWith(fake.dispatch, c.args, &stdout, &stderr); rc == 0 {
				t.Errorf("%v: rc = 0, want a refusal before any step\n%s%s", c.args, stdout.String(), stderr.String())
			}
			if len(fake.calls) != 0 {
				t.Errorf("%v: dispatched %v before validating its arguments", c.args, fake.verbs())
			}
		})
	}
}
