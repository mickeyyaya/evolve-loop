package main

import (
	"io"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func holdJourney(t *testing.T) (*trainJourney, *[]signalcenter.Event) {
	t.Helper()
	j := recordedJourney(t, 1, 2)
	var events []signalcenter.Event
	j.env.Emit = func(_ string, _ io.Writer, e signalcenter.Event) { events = append(events, e) }
	j.gh.conclusions = []string{"failure", "failure"}
	j.gh.onCreate = func() { j.gh.main.conclusion = "failure" }
	return j, &events
}

func TestRunBatch_HoldsARedBatchOnARedBaseAndSaysSo(t *testing.T) {
	t.Parallel()
	j, _ := holdJourney(t)
	before := j.originMain()
	rc, out, errb := j.run("batch", "1", "2")
	short := integration.ShortSHA(before)
	if rc != exitRefused || !strings.Contains(errb, short) || !strings.Contains(errb, "--main-red-fix") {
		t.Fatalf("rc=%d err=%q, want the exit of a main refusal that names main %s and the fix flag", rc, errb, short)
	}
	want := "step: hold PRs 1 2 in batch PR #99: main " + short + " is not green (base run " + mainRunID + ")"
	if !strings.Contains(out, want) {
		t.Errorf("output lacks %q:\n%s", want, out)
	}
	if strings.Contains(out, "step: split") || j.originMain() != before || len(j.gh.comments) != 0 || len(j.gh.closed) != 0 {
		t.Errorf("a hold must not split, land, comment or close:\n%s", out)
	}
}

func TestRunBatch_AHoldEmitsOneSignalThroughTheBatchOrigin(t *testing.T) {
	t.Parallel()
	j, events := holdJourney(t)
	before := j.originMain()
	j.run("batch", "1", "2")
	var held []signalcenter.Event
	for _, e := range *events {
		if e.Code == batchHeldCode {
			held = append(held, e)
		}
	}
	if len(held) != 1 {
		t.Fatalf("hold signals = %+v, want exactly one", *events)
	}
	e := held[0]
	if e.Origin != batchEventOrigin || e.Kind != signalcenter.KindShipWarning || e.Severity != signalcenter.SeverityWarn {
		t.Errorf("signal = %+v, want the batch origin, a ship warning and the severity warn", e)
	}
	if e.Fields["base"] != before || e.Fields["base_run"] != mainRunID || e.Fields["batch"] != "99" || e.Fields["prs"] != "1 2" {
		t.Errorf("fields = %v, want the base, its run, the batch and the PRs", e.Fields)
	}
}

func TestRunBatch_AnOverriddenRedBaseStillSplits(t *testing.T) {
	t.Parallel()
	j := recordedJourney(t, 1)
	j.gh.main.conclusion = "failure"
	j.gh.conclusions = []string{"failure", "failure"}
	rc, out, _ := j.run("batch", "--main-red-fix", "1")
	if rc != exitRefused || strings.Contains(out, "step: hold") || !strings.Contains(out, "CI red, --main-red-fix") {
		t.Errorf("rc=%d out=%q, want a split to the single PR and no hold", rc, out)
	}
}

func TestRunBatch_StepLinesNameTheRunTheVerdictCameFrom(t *testing.T) {
	t.Parallel()
	j := recordedJourney(t, 1, 2)
	rc, out, errb := j.run("batch", "1", "2")
	if rc != 0 {
		t.Fatalf("rc=%d err=%s", rc, errb)
	}
	for _, w := range []string{"step: batch PR #99 of PRs 1 2 is green (run 123)", "step: landed PRs 1 2 through batch PR #99 (run 123)"} {
		if !strings.Contains(out, w) {
			t.Errorf("output lacks %q:\n%s", w, out)
		}
	}
	red := recordedJourney(t, 1)
	red.gh.conclusions = []string{"failure", "success"}
	_, out, _ = red.run("batch", "1")
	for _, w := range []string{"is red (run 123)", "step: rerun the failed jobs of batch PR #99 (run 123)"} {
		if !strings.Contains(out, w) {
			t.Errorf("output lacks %q:\n%s", w, out)
		}
	}
}

func TestStepLine_NamesNoneForATipWithNoLoggedRun(t *testing.T) {
	t.Parallel()
	runs := integration.NewRunLog()
	for _, c := range []struct {
		step integration.Step
		want string
	}{
		{integration.Step{Kind: integration.StepVerdict, PRs: []int{4}, Batch: 9, Verdict: integration.Green, Tip: "t"}, "step: batch PR #9 of PRs 4 is green (run none)"},
		{integration.Step{Kind: integration.StepRerun, Batch: 9, Tip: "t"}, "step: rerun the failed jobs of batch PR #9 (run none)"},
		{integration.Step{Kind: integration.StepLand, PRs: []int{4}, Batch: 9, Tip: "t"}, "step: landed PRs 4 through batch PR #9 (run none)"},
		{integration.Step{Kind: integration.StepHold, PRs: []int{4}, Batch: 9, Tip: "t", Base: "abcdef0123"}, "step: hold PRs 4 in batch PR #9: main abcdef0 is not green (base run none)"},
	} {
		if got := stepLine(c.step, runs); got != c.want {
			t.Errorf("stepLine = %q, want %q", got, c.want)
		}
	}
}

func TestIntegrationTrain_SharesOneRunLogBetweenItsCIPorts(t *testing.T) {
	t.Parallel()
	train, runs := (integrationEnv{Root: "/repo"}).train("/repo", io.Discard)
	ci, ok := train.MainCI.(integration.GhCI)
	if runs == nil || !ok || ci.Runs != runs {
		t.Errorf("runs = %v, main CI = %T %+v, want one log shared with the CI port", runs, train.MainCI, ci)
	}
}

func TestRunBatch_AFailedBaseCheckStopsWithoutAHoldSignal(t *testing.T) {
	t.Parallel()
	j, events := holdJourney(t)
	j.gh.onCreate = func() { j.gh.main.listRC = 1 }
	before := j.originMain()
	rc, out, errb := j.run("batch", "1", "2")
	short := integration.ShortSHA(before)
	if rc == 0 || strings.Contains(out, "is not green") || strings.Contains(errb, "--main-red-fix") {
		t.Fatalf("rc=%d out=%q err=%q, want a stop that does not call main red", rc, out, errb)
	}
	if !strings.Contains(out, "step: stop PRs 1 2 in batch PR #99: the check of main "+short+" failed: ") {
		t.Errorf("output lacks the failed-check line:\n%s", out)
	}
	for _, e := range *events {
		if e.Code == batchHeldCode {
			t.Errorf("a failed base check emitted the hold signal %+v", e)
		}
	}
}

func TestStepLine_ARefusalHoldsAndAnyOtherBaseErrorStops(t *testing.T) {
	t.Parallel()
	base := "abcdef0123"
	hold := integration.Step{Kind: integration.StepHold, PRs: []int{4}, Batch: 9, Base: base, Err: &integration.MainRefusal{SHA: base, Reason: "red"}}
	if got, want := stepLine(hold, integration.NewRunLog()), "step: hold PRs 4 in batch PR #9: main abcdef0 is not green (base run none)"; got != want {
		t.Errorf("stepLine = %q, want %q", got, want)
	}
	stop := integration.Step{Kind: integration.StepHold, PRs: []int{4}, Batch: 9, Base: base, Err: integration.ErrRunTimeout}
	if got := stepLine(stop, integration.NewRunLog()); !strings.HasPrefix(got, "step: stop PRs 4 in batch PR #9: the check of main abcdef0 failed: ") || !strings.Contains(got, "timed out") {
		t.Errorf("stepLine = %q, want the failed-check line naming the error", got)
	}
}

func TestCliIOFail_APendingMainHintsToWaitAndARedMainKeepsTheFixHint(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ reason, want, absent string }{
		{string(integration.MainPending), "wait for the CI run of main", mainRedFixFlag},
		{"red", mainRedFixFlag, "wait for the CI run"},
	} {
		var errb strings.Builder
		cliIO{stdout: io.Discard, stderr: &errb}.fail(&integration.MainRefusal{SHA: "abcdef0123", Reason: c.reason})
		if got := errb.String(); !strings.Contains(got, c.want) || strings.Contains(got, c.absent) {
			t.Errorf("reason %q: stderr = %q, want %q and not %q", c.reason, got, c.want, c.absent)
		}
	}
}

func trainOf(env integrationEnv, root string) integration.Train {
	t, _ := env.train(root, io.Discard)
	return t
}
