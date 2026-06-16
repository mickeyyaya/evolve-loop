package recipe

import (
	"context"
	"errors"
	"testing"
)

// This file closes the apicover gap for the recipe types/consts that existing
// tests drive through Engine.Run but never NAME by their exported identifier.
// apicover flags those UNCOVERED. Each test below binds a value to an
// explicitly-typed variable (naming the type/const) AND exercises it through
// the real executor, asserting a load-bearing contract.

// TestSessionDriver_SatisfiedAndExercised names the SessionDriver port and
// proves the package's fakeDriver satisfies it, then drives a real one-step
// recipe through Engine.Run via that interface value — exercising every
// SessionDriver method (EnsureSession, SendCommand, AutoRespond, Capture).
func TestSessionDriver_SatisfiedAndExercised(t *testing.T) {
	fd := &fakeDriver{paneSeq: []string{"all done"}}
	// Compile-time + run-time proof of interface satisfaction.
	var d SessionDriver = fd
	eng := &Engine{Driver: d, Clock: noClock{}, PromptMarker: "❯"}

	r := Recipe{Name: "x", Steps: []Step{
		cmdStep("go", "/go", Await{Kind: AwaitAnyOf, Substrs: []string{"done"}, TimeoutS: 6, IntervalS: 1}),
	}}
	res, err := eng.Run(context.Background(), r, "claude-tmux", nil)
	if err != nil {
		t.Fatalf("Run via SessionDriver err=%v", err)
	}
	if res.Status != StatusComplete {
		t.Fatalf("status=%v, want complete", res.Status)
	}
	// The driver was actually exercised through the interface.
	if len(fd.commands) != 1 || fd.commands[0] != "/go" {
		t.Errorf("SessionDriver.SendCommand not exercised: commands=%v", fd.commands)
	}
	if fd.autoCalls == 0 {
		t.Errorf("SessionDriver.AutoRespond not exercised")
	}
}

// TestSendKind_AndAwaitKind_DriveStepBehavior names SendKind and AwaitKind by
// binding their constants to explicitly-typed variables, then proves they
// select real behavior: a KindKeys send routes to SendKeys (not SendCommand),
// and an AwaitPromptMarker await is satisfied by the prompt marker.
func TestSendKind_AndAwaitKind_DriveStepBehavior(t *testing.T) {
	var sk SendKind = KindKeys
	var ak AwaitKind = AwaitPromptMarker

	fd := &fakeDriver{paneSeq: []string{"menu\n❯"}}
	r := Recipe{Name: "x", Steps: []Step{
		{Send: Send{Kind: sk, Body: "Down Enter"}, Await: Await{Kind: ak, TimeoutS: 6, IntervalS: 1}},
	}}
	res, err := newEngine(fd).Run(context.Background(), r, "claude-tmux", nil)
	if err != nil {
		t.Fatalf("Run err=%v", err)
	}
	if res.Status != StatusComplete {
		t.Fatalf("status=%v, want complete", res.Status)
	}
	// KindKeys routed to SendKeys, NOT SendCommand.
	if len(fd.keys) != 1 || fd.keys[0] != "Down Enter" {
		t.Errorf("SendKind KindKeys did not route to SendKeys: keys=%v", fd.keys)
	}
	if len(fd.commands) != 0 {
		t.Errorf("KindKeys must not use SendCommand: commands=%v", fd.commands)
	}
	// The two kinds are the documented string labels.
	if KindKeys != "keys" || KindCommand != "command" {
		t.Errorf("SendKind labels: KindKeys=%q KindCommand=%q", KindKeys, KindCommand)
	}
	if AwaitPromptMarker != "prompt_marker" {
		t.Errorf("AwaitKind label AwaitPromptMarker=%q, want prompt_marker", AwaitPromptMarker)
	}
}

// TestOnTimeoutAbort_FailsRecipe names the OnTimeoutAbort const and proves its
// contract: a step whose await never matches and whose OnTimeout is
// OnTimeoutAbort fails the whole recipe (ErrStepTimeout, StepTimedOut). This is
// the abort half of the on_timeout strategy (the continue half lives in
// executor_test.go's StepTimeoutContinue test).
func TestOnTimeoutAbort_FailsRecipe(t *testing.T) {
	r := Recipe{Name: "x", Steps: []Step{
		{
			Name:      "never",
			Send:      Send{Kind: KindCommand, Body: "/go"},
			Await:     Await{Kind: AwaitAnyOf, Substrs: []string{"nope"}, TimeoutS: 4, IntervalS: 2},
			OnTimeout: OnTimeoutAbort,
		},
	}}
	d := &fakeDriver{paneSeq: []string{"still working"}}
	res, err := newEngine(d).Run(context.Background(), r, "claude-tmux", nil)
	if !errors.Is(err, ErrStepTimeout) {
		t.Fatalf("err=%v, want ErrStepTimeout under OnTimeoutAbort", err)
	}
	if res.Status != StatusFailed {
		t.Errorf("Result.Status=%v, want failed", res.Status)
	}
	if res.Steps[0].Status != StepTimedOut {
		t.Errorf("step Status=%v, want StepTimedOut", res.Steps[0].Status)
	}
	if OnTimeoutAbort != "abort" {
		t.Errorf("OnTimeoutAbort=%q, want abort", OnTimeoutAbort)
	}
}

// TestResult_StepResult_StepStatus_BoundFromProducer names Result, StepResult,
// and StepStatus by binding them to explicitly-typed variables from Engine.Run's
// output and asserting their fields on a successful two-step recipe.
func TestResult_StepResult_StepStatus_BoundFromProducer(t *testing.T) {
	r := Recipe{Name: "plugin-install", Steps: []Step{
		cmdStep("add", "/add", Await{Kind: AwaitAnyOf, Substrs: []string{"added"}, TimeoutS: 6, IntervalS: 1}),
		cmdStep("reload", "/reload", Await{Kind: AwaitPromptMarker, TimeoutS: 6, IntervalS: 1}),
	}}
	d := &fakeDriver{paneSeq: []string{"added", "done\n❯"}}

	var res Result
	res, err := newEngine(d).Run(context.Background(), r, "claude-tmux", nil)
	if err != nil {
		t.Fatalf("Run err=%v", err)
	}
	if res.Recipe != "plugin-install" || res.CLI != "claude-tmux" {
		t.Errorf("Result identity wrong: recipe=%q cli=%q", res.Recipe, res.CLI)
	}
	if res.Status != StatusComplete {
		t.Errorf("Result.Status=%v, want complete", res.Status)
	}
	if len(res.Steps) != 2 {
		t.Fatalf("Result.Steps len=%d, want 2", len(res.Steps))
	}

	var sr StepResult = res.Steps[0]
	if sr.Name != "add" {
		t.Errorf("StepResult.Name=%q, want add", sr.Name)
	}
	var st StepStatus = sr.Status
	if st != StepOK {
		t.Errorf("StepStatus=%v, want StepOK", st)
	}
	// StepStatus is the documented string label set.
	if StepOK != "ok" {
		t.Errorf("StepOK=%q, want ok", StepOK)
	}
}
