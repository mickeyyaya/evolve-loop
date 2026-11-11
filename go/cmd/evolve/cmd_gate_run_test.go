package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentclass"
	"github.com/mickeyyaya/evolve-loop/go/internal/gatecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/gaterun"
)

func gateRunFixture() []gatecontract.Entry {
	return []gatecontract.Entry{
		{Name: "green", Run: func(context.Context, gatecontract.Input, gatecontract.Ports) (gatecontract.Verdict, error) {
			return gatecontract.Verdict{Notes: []string{"judged"}}, nil
		}},
		{Name: "broken", Run: func(context.Context, gatecontract.Input, gatecontract.Ports) (gatecontract.Verdict, error) {
			return gatecontract.Verdict{}, errors.New("did not judge")
		}},
	}
}

func inputFrame(t *testing.T) *bytes.Buffer {
	t.Helper()
	var in bytes.Buffer
	if err := gaterun.WriteFrame(&in, gaterun.Frame{Kind: gaterun.KindInput, Input: &gatecontract.Input{Root: "/lane"}}); err != nil {
		t.Fatal(err)
	}
	return &in
}

func TestRunGateRun_ServesTheNamedEntryAndWritesItsVerdictFrame(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if rc := runGateRunWith(gateRunFixture(), []string{"--entry", "green"}, inputFrame(t), &stdout, &stderr); rc != cliExitOK {
		t.Fatalf("rc = %d, stderr = %q", rc, stderr.String())
	}
	frame, err := gaterun.ReadFrame(bufio.NewReader(&stdout))
	if err != nil || frame.Kind != gaterun.KindVerdict || frame.Verdict.Notes[0] != "judged" {
		t.Errorf("frame = %+v, err = %v", frame, err)
	}
}

func TestRunGateRun_AnEntryThatFailsGivesAnErrorFrameAndExitFailed(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if rc := runGateRunWith(gateRunFixture(), []string{"--entry", "broken"}, inputFrame(t), &stdout, &stderr); rc != cliExitFailed {
		t.Errorf("rc = %d, want %d", rc, cliExitFailed)
	}
	frame, _ := gaterun.ReadFrame(bufio.NewReader(&stdout))
	if frame.Kind != gaterun.KindError || !strings.Contains(frame.Message, "did not judge") {
		t.Errorf("frame = %+v", frame)
	}
	if !strings.Contains(stderr.String(), "did not judge") {
		t.Errorf("stderr = %q, want the cause", stderr.String())
	}
}

func TestRunGateRun_AnUnknownEntryAndBadArgumentsAreUsageErrors(t *testing.T) {
	t.Parallel()
	for name, args := range map[string][]string{
		"unknown entry": {"--entry", "nope"},
		"no entry":      {},
		"extra word":    {"--entry", "green", "more"},
		"bad flag":      {"--bogus"},
	} {
		var stdout, stderr bytes.Buffer
		rc := runGateRunWith(gateRunFixture(), args, strings.NewReader(""), &stdout, &stderr)
		if rc != cliExitUsage {
			t.Errorf("%s: rc = %d, want %d (stderr %q)", name, rc, cliExitUsage, stderr.String())
		}
		if stdout.Len() != 0 {
			t.Errorf("%s: a usage error writes no frame: %q", name, stdout.String())
		}
	}
	var stderr bytes.Buffer
	runGateRunWith(gateRunFixture(), []string{"--entry", "nope"}, strings.NewReader(""), &bytes.Buffer{}, &stderr)
	if !strings.Contains(stderr.String(), `"nope"`) || !strings.Contains(stderr.String(), "green") {
		t.Errorf("the refusal names the entry and the known entries: %q", stderr.String())
	}
}

func TestRunGate_OnlyTheRunVerbExistsAndItUsesTheRegistry(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if rc := runGate([]string{"list"}, strings.NewReader(""), &stdout, &stderr); rc != cliExitUsage || !strings.Contains(stderr.String(), gateRunUsage) {
		t.Errorf("rc = %d stderr = %q", rc, stderr.String())
	}
	stderr.Reset()
	if rc := runGate([]string{"run", "--entry", "no-such-registered-entry"}, strings.NewReader(""), &stdout, &stderr); rc != cliExitUsage {
		t.Errorf("rc = %d for an entry that the registry lacks", rc)
	}
	if rc := runGate(nil, strings.NewReader(""), &stdout, &stderr); rc != cliExitUsage {
		t.Errorf("rc = %d for no verb", rc)
	}
}

func TestGateIsAPipelineVerbThatACellCannotRun(t *testing.T) {
	t.Parallel()
	found := false
	for _, c := range commands {
		found = found || c.Name == "gate"
	}
	if _, core := lookupCoreVerb("gate run"); !found || core {
		t.Errorf("gate verb registered = %v, core verb = %v: only the parent of a hermetic run starts it, so it is a pipeline tool", found, core)
	}
	if m := agentclass.Of([]string{"gate", "run", "--entry", "x"}); m.Class != agentclass.Denied {
		t.Errorf("agent class = %q, want D: no cell has a reason to serve a gate entry", m.Class)
	}
}
