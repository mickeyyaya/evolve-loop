package main

import (
	"bytes"
	"io"
	"slices"
	"strings"
	"testing"
)

var boundaryFullVerbs = []string{
	"loop-stop", "pr", boundaryRatchetVerb, "sync-main", queueApplyVerb, waveBuildVerb, "inbox", "selfheal", "selfheal",
	"quality", "ci", "wave", "gc", "code", "loop-stop", boundaryLogVerb, "loop",
}

const latencyTestRoot = "/plane"

type latencyCheckFake struct {
	fakeBoundaryVerbs
	rcByVerb map[string]int
	onWave   func(args []string) int
}

func (f *latencyCheckFake) dispatch(verb string, args []string, stdout, stderr io.Writer) int {
	rc := f.fakeBoundaryVerbs.dispatch(verb, args, stdout, stderr)
	if verb == "wave" && f.onWave != nil {
		return f.onWave(args)
	}
	if code, ok := f.rcByVerb[verb]; ok {
		return code
	}
	return rc
}

func stepIndexOf(steps []boundaryStep, verb string, firstArg string) int {
	return slices.IndexFunc(steps, func(s boundaryStep) bool {
		return s.verb == verb && len(s.args) > 0 && s.args[0] == firstArg
	})
}

func TestBoundarySteps_RunTheT5AndT6LatencyChecksBetweenTheQualityReportAndGC(t *testing.T) {
	t.Parallel()
	plans := map[string][]boundaryStep{
		"boundary run": boundarySteps(boundaryArgs{goalText: "goal"}, latencyTestRoot),
		"wave next":    waveSteps(boundaryArgs{goalText: "goal"}, latencyTestRoot, nil),
	}
	for name, steps := range plans {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			quality := stepIndexOf(steps, "quality", "report")
			ci := stepIndexOf(steps, "ci", "latency")
			waveCheck := stepIndexOf(steps, "wave", "latency")
			gc := stepIndexOf(steps, "gc", "--lease-wait")
			if quality < 0 || ci < 0 || waveCheck < 0 || gc < 0 {
				t.Fatalf("plan lacks a step: quality %d, ci latency %d, wave latency %d, gc %d\n%v", quality, ci, waveCheck, gc, steps)
			}
			if !(quality < ci && ci < waveCheck && waveCheck < gc) {
				t.Errorf("order: quality %d, ci %d, wave %d, gc %d; want the two checks between the quality report and gc", quality, ci, waveCheck, gc)
			}
			if got, want := strings.Join(steps[ci].args, " "), "latency check --project-root "+latencyTestRoot; got != want {
				t.Errorf("ci step args = %q, want %q", got, want)
			}
			if got, want := strings.Join(steps[waveCheck].args, " "), "latency --check --project-root "+latencyTestRoot; got != want {
				t.Errorf("wave step args = %q, want %q", got, want)
			}
		})
	}
}

func TestBoundaryRun_AFailedLatencyCheckWarnsAndTheBoundaryContinuesToTheLaunch(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		rcs   map[string]int
		warns int
	}{
		{"ci latency check usage error", map[string]int{"ci": exitUsage}, 1},
		{"wave latency check I/O error", map[string]int{"wave": exitIO}, 1},
		{"both checks fail", map[string]int{"ci": exitRefused, "wave": exitIO}, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			fake := &latencyCheckFake{rcByVerb: c.rcs}
			var stdout, stderr bytes.Buffer

			rc := runBoundaryWith(fake.dispatch, []string{"run", "--merge", "12", "--goal-text-file", fakeBoundaryGoalFile(t)}, &stdout, &stderr)

			if rc != 0 {
				t.Errorf("rc = %d, want 0: a latency check never blocks the boundary\n%s%s", rc, stdout.String(), stderr.String())
			}
			if got := fake.verbs(); !slices.Equal(got, boundaryFullVerbs) {
				t.Errorf("dispatched %v, want every step %v", got, boundaryFullVerbs)
			}
			out := stderr.String()
			if got := strings.Count(out, "WARN"); got != c.warns {
				t.Errorf("stderr has %d WARN line(s), want %d\n%s", got, c.warns, out)
			}
			if !strings.Contains(out, "latency") {
				t.Errorf("the WARN does not name the latency check\n%s", out)
			}
			if strings.Contains(out, "stopped there") {
				t.Errorf("the boundary reported a stop for an advisory check\n%s", out)
			}
			if !strings.Contains(stdout.String(), fakeBoundaryPidLine) {
				t.Errorf("the loop was not launched after a failed check\n%s", stdout.String())
			}
		})
	}
}

func TestBoundaryRun_PassingLatencyChecksPrintNoWarning(t *testing.T) {
	t.Parallel()
	fake := &latencyCheckFake{}
	var stdout, stderr bytes.Buffer

	rc := runBoundaryWith(fake.dispatch, []string{"run", "--goal-text-file", fakeBoundaryGoalFile(t)}, &stdout, &stderr)

	if rc != 0 || strings.Contains(stderr.String(), "WARN") {
		t.Errorf("rc = %d, stderr %q; want 0 and no WARN when both checks pass", rc, stderr.String())
	}
	if !slices.Contains(fake.verbs(), "ci") || !slices.Contains(fake.verbs(), "wave") {
		t.Errorf("dispatched %v, want both latency checks", fake.verbs())
	}
}

func TestBoundaryRun_ABreachThatFilesAnItemDoesNotBlockTheBoundary(t *testing.T) {
	t.Parallel()
	p := checkPlane(t, checkWave{live: 4}, checkWave{live: 4}, checkWave{live: 2})
	h := newWaveHarness()
	fake := &latencyCheckFake{}
	fake.onWave = func(args []string) int { return h.run(args...) }
	steps := boundarySteps(boundaryArgs{goalText: "goal"}, p.root)
	var stdout, stderr bytes.Buffer

	rc := runBoundarySteps(fake.dispatch, steps, &stdout, &stderr)

	if rc != 0 {
		t.Fatalf("rc = %d, want 0 after a breach\n%s%s", rc, stdout.String(), stderr.String())
	}
	if got := inboxFiles(t, p); len(got) != 1 || !strings.HasSuffix(got[0], "latency-wave-breach-from-wave-2.json") {
		t.Errorf("inbox = %v, want the one item the real wave latency check files for the breach", got)
	}
	if !strings.Contains(h.stdout.String(), "LATENCY_BUDGET_EXCEEDED") {
		t.Errorf("the wave latency check was not run with --check against the plane\n%s", h.stdout.String())
	}
	if got := fake.verbs(); got[len(got)-1] != "loop" {
		t.Errorf("the boundary stopped before the launch: %v", got)
	}
}

func TestBoundaryDispatch_RegistersTheLatencyCheckVerbs(t *testing.T) {
	t.Parallel()
	dispatchers := map[string]boundaryDispatch{
		"boundary run": dispatchPlaneVerbs(nil),
		"wave next":    dispatchWaveVerb(nil),
	}
	calls := map[string][]string{
		"ci":   {"latency", "check", "--project-root", t.TempDir()},
		"wave": {"latency", "--check", "--project-root", t.TempDir()},
	}
	for name, dispatch := range dispatchers {
		for verb, args := range calls {
			t.Run(name+" "+verb, func(t *testing.T) {
				t.Parallel()
				var stdout, stderr bytes.Buffer
				dispatch(verb, args, &stdout, &stderr)
				if strings.Contains(stderr.String(), "no handler for step") {
					t.Errorf("the %s dispatcher has no handler for %q\n%s", name, verb, stderr.String())
				}
			})
		}
	}
}

func TestWaveNext_AFailedLatencyCheckWarnsAndTheWaveLaunches(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		failAt int
		verb   string
	}{
		{"ci latency check fails", 12, "ci"},
		{"wave latency check fails", 13, "wave"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			p := newWavePlane(t)
			p.seedWave81()
			h := newWaveHarness()
			h.verbs.failAt, h.verbs.code = c.failAt, exitIO

			rc := h.run("next", "--json", "--merge", "816", "--project-root", p.root)

			if rc != 0 {
				t.Fatalf("rc = %d, want 0; stderr:\n%s", rc, h.stderr.String())
			}
			e := decodeWaveEnvelope(t, h.stdout.Bytes())
			if !e.Launched || !e.Recorded || e.FailedStep != "" || e.Error != "" {
				t.Errorf("envelope launched %t recorded %t failed_step %q error %q; want a launched, recorded wave", e.Launched, e.Recorded, e.FailedStep, e.Error)
			}
			if got := waveStepVerbs(e); !slices.Equal(got, waveFullSteps) {
				t.Errorf("steps = %q, want every step %q", got, waveFullSteps)
			}
			if step := e.Steps[c.failAt-1]; step.Verb != c.verb || step.OK {
				t.Errorf("step %d = %+v, want the failed %s check recorded as not ok", c.failAt, step, c.verb)
			}
			if !strings.Contains(h.stderr.String(), "WARN") || !strings.Contains(h.stderr.String(), "latency") {
				t.Errorf("stderr lacks a latency WARN\n%s", h.stderr.String())
			}
		})
	}
}
