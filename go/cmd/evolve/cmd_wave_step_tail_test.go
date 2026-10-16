package main

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
)

func diagLines(total int) []string {
	lines := make([]string, 0, total)
	for i := 1; i <= total; i++ {
		lines = append(lines, fmt.Sprintf("diag %02d/%d", i, total))
	}
	return lines
}

func linesIn(text string, lines []string) (in, out []string) {
	for _, l := range lines {
		if strings.Contains(text, l) {
			in = append(in, l)
		} else {
			out = append(out, l)
		}
	}
	return in, out
}

func failingDoctorDispatch(next boundaryDispatch, stderrLines []string, code int) boundaryDispatch {
	return func(verb string, args []string, stdout, stderr io.Writer) int {
		if verb == "loop-stop" {
			fmt.Fprintln(stderr, "noise from an earlier passing step")
		}
		if verb != "doctor" {
			return next(verb, args, stdout, stderr)
		}
		for _, l := range stderrLines {
			fmt.Fprintln(stderr, l)
		}
		return code
	}
}

func runWaveWithFailingDoctor(t *testing.T, stderrLines []string) (waveEnvelope, string) {
	t.Helper()
	p := newWavePlane(t)
	p.seedWave81()
	h := newWaveHarness()
	env := h.env(p.root)
	env.dispatch = failingDoctorDispatch(env.dispatch, stderrLines, 1)
	var stdout, stderr bytes.Buffer
	if rc := runWaveWith(env, []string{"next", "--json", "--project-root", p.root}, &stdout, &stderr); rc != 1 {
		t.Fatalf("rc = %d, want the failed doctor step's 1\n%s", rc, stderr.String())
	}
	return decodeWaveEnvelope(t, stdout.Bytes()), stderr.String()
}

func failedStepDetail(t *testing.T, e waveEnvelope) (string, int) {
	t.Helper()
	for i, s := range e.Steps {
		if !s.OK {
			if s.Verb != "doctor" {
				t.Fatalf("failed step = %+v, want the doctor live step", s)
			}
			return s.Detail, i
		}
	}
	t.Fatalf("no failed step in %+v", e.Steps)
	return "", 0
}

func TestWaveNext_FailedStepDetailCarriesTheLast40StderrLinesBesideRC(t *testing.T) {
	t.Parallel()
	e, _ := runWaveWithFailingDoctor(t, diagLines(55))
	detail, _ := failedStepDetail(t, e)
	if !strings.HasPrefix(detail, "rc=1") {
		t.Errorf("detail %q does not start with rc=1", detail)
	}
	if _, missing := linesIn(detail, diagLines(55)[15:]); len(missing) > 0 {
		t.Errorf("detail lacks %d of the last 40 stderr lines (first %q):\n%s", len(missing), missing[0], detail)
	}
	if kept, _ := linesIn(detail, diagLines(55)[:15]); len(kept) > 0 {
		t.Errorf("detail keeps %v, older than the last 40 lines:\n%s", kept, detail)
	}
}

func TestWaveNext_FailedStepTailHoldsOnlyThatStepsStderr(t *testing.T) {
	t.Parallel()
	e, _ := runWaveWithFailingDoctor(t, diagLines(3))
	detail, i := failedStepDetail(t, e)
	if strings.Contains(detail, "noise from an earlier passing step") {
		t.Errorf("failed step detail carries another step's stderr:\n%s", detail)
	}
	if !strings.Contains(detail, "diag 03/3") {
		t.Errorf("failed step detail %q lacks its own stderr", detail)
	}
	for _, s := range e.Steps[:i] {
		if s.Detail != "rc=0" {
			t.Errorf("passing step %s detail = %q, want rc=0", s.Name, s.Detail)
		}
	}
}

func TestWaveNext_StepStderrStillReachesTheCallersStderr(t *testing.T) {
	t.Parallel()
	_, stderr := runWaveWithFailingDoctor(t, diagLines(3))
	if _, missing := linesIn(stderr, append(diagLines(3), "noise from an earlier passing step")); len(missing) > 0 {
		t.Errorf("caller stderr lacks %v:\n%s", missing, stderr)
	}
}

func TestBoundary_StepFailureWritesTheStepsStderrTailAfterTheFailureLine(t *testing.T) {
	t.Parallel()
	steps := []boundaryStep{{verb: "loop-stop", args: []string{"--wait"}}, {verb: "doctor", args: []string{"live", "claude-tmux"}}}
	dispatch := failingDoctorDispatch(func(string, []string, io.Writer, io.Writer) int { return 0 }, diagLines(55), 1)
	var stdout, stderr bytes.Buffer

	rc := runBoundarySteps(dispatch, steps, &stdout, &stderr)

	if rc != 1 {
		t.Fatalf("rc = %d, want 1", rc)
	}
	log := stderr.String()
	at := strings.Index(log, "failed rc=1")
	if at < 0 {
		t.Fatalf("boundary log has no failure line:\n%s", log)
	}
	after := log[at:]
	if _, missing := linesIn(after, diagLines(55)[15:]); len(missing) > 0 {
		t.Errorf("boundary log after the failure line lacks %d of the last 40 stderr lines (first %q):\n%s", len(missing), missing[0], after)
	}
	if repeated, _ := linesIn(after, append(diagLines(55)[:15], "noise from an earlier passing step")); len(repeated) > 0 {
		t.Errorf("boundary log after the failure line repeats %v, outside the failing step's last 40 lines:\n%s", repeated, after)
	}
}

func TestBoundary_PassingRunWritesNoStderrTail(t *testing.T) {
	t.Parallel()
	steps := []boundaryStep{{verb: "loop-stop", args: []string{"--wait"}}}
	dispatch := failingDoctorDispatch(func(string, []string, io.Writer, io.Writer) int { return 0 }, nil, 1)
	var stdout, stderr bytes.Buffer

	if rc := runBoundarySteps(dispatch, steps, &stdout, &stderr); rc != 0 {
		t.Fatalf("rc = %d, want 0", rc)
	}
	if got := strings.Count(stderr.String(), "noise from an earlier passing step"); got != 1 {
		t.Errorf("passing step stderr appears %d times, want once (no tail on success):\n%s", got, stderr.String())
	}
}
