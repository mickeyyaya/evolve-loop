package recipe

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"
)

// noClock is the no-op sleep seam so poll loops run instantly.
type noClock struct{}

func (noClock) Sleep(time.Duration) {}

// fakeDriver is a scriptable SessionDriver. paneSeq is consumed in order by
// Capture (last value repeats). The *Err / *At fields drive failure branches.
type fakeDriver struct {
	ensureErr error

	paneSeq []string
	paneIdx int

	commands   []string // bodies sent via SendCommand
	keys       []string // bodies sent via SendKeys
	sendErr    error    // returned by Send* calls
	captureErr error    // returned by Capture calls

	autoEscalateAt int // 1-based tick at which AutoRespond returns escalated=true; 0=never
	autoErr        error
	autoCalls      int
}

func (f *fakeDriver) EnsureSession(context.Context) error { return f.ensureErr }

func (f *fakeDriver) Capture(context.Context) (string, error) {
	if f.captureErr != nil {
		return "", f.captureErr
	}
	if len(f.paneSeq) == 0 {
		return "", nil
	}
	p := f.paneSeq[f.paneIdx]
	if f.paneIdx < len(f.paneSeq)-1 {
		f.paneIdx++
	}
	return p, nil
}

func (f *fakeDriver) SendCommand(_ context.Context, body string) error {
	f.commands = append(f.commands, body)
	return f.sendErr
}

func (f *fakeDriver) SendKeys(_ context.Context, body string) error {
	f.keys = append(f.keys, body)
	return f.sendErr
}

func (f *fakeDriver) AutoRespond(context.Context) (bool, error) {
	f.autoCalls++
	if f.autoErr != nil {
		return false, f.autoErr
	}
	if f.autoEscalateAt > 0 && f.autoCalls >= f.autoEscalateAt {
		return true, nil
	}
	return false, nil
}

func newEngine(d *fakeDriver) *Engine {
	return &Engine{Driver: d, Clock: noClock{}, PromptMarker: "❯"}
}

func cmdStep(name, body string, a Await) Step {
	return Step{Name: name, Send: Send{Kind: KindCommand, Body: body}, Await: a}
}

func TestEngineRun_HappyMultiStep(t *testing.T) {
	r := Recipe{
		Name:   "plugin-install",
		Params: []ParamDecl{{Name: "mkt", Required: true}, {Name: "plg", Required: true}},
		Steps: []Step{
			cmdStep("add", "/plugin marketplace add {{mkt}}", Await{Kind: AwaitAnyOf, Substrs: []string{"added"}, TimeoutS: 10, IntervalS: 1}),
			cmdStep("install", "/plugin install {{plg}}", Await{Kind: AwaitAnyOf, Substrs: []string{"installed"}, TimeoutS: 10, IntervalS: 1}),
			cmdStep("reload", "/reload-plugins", Await{Kind: AwaitPromptMarker, TimeoutS: 10, IntervalS: 1}),
		},
	}
	d := &fakeDriver{paneSeq: []string{"marketplace added", "installed ok", "done\n❯"}}
	got, err := newEngine(d).Run(context.Background(), r, "claude-tmux", Params{"mkt": "https://x", "plg": "ecc@ecc"})
	if err != nil {
		t.Fatalf("Run err=%v", err)
	}
	if got.Status != StatusComplete || len(got.Steps) != 3 {
		t.Fatalf("result=%+v", got)
	}
	for _, s := range got.Steps {
		if s.Status != StepOK {
			t.Errorf("step %s status=%s", s.Name, s.Status)
		}
	}
	wantCmds := []string{"/plugin marketplace add https://x", "/plugin install ecc@ecc", "/reload-plugins"}
	if len(d.commands) != 3 {
		t.Fatalf("commands=%v", d.commands)
	}
	for i, c := range wantCmds {
		if d.commands[i] != c {
			t.Errorf("command[%d]=%q want %q", i, d.commands[i], c)
		}
	}
}

func TestEngineRun_IntervalClampedToTimeout(t *testing.T) {
	// interval_s (5) > timeout_s (3): interval must clamp so at least one
	// Capture happens within the budget rather than oversleeping past it.
	r := Recipe{Name: "x", Steps: []Step{
		cmdStep("go", "/go", Await{Kind: AwaitAnyOf, Substrs: []string{"done"}, TimeoutS: 3, IntervalS: 5}),
	}}
	d := &fakeDriver{paneSeq: []string{"done"}}
	got, err := newEngine(d).Run(context.Background(), r, "claude-tmux", nil)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if got.Status != StatusComplete {
		t.Fatalf("result=%+v", got)
	}
	if d.paneIdx == 0 && len(d.paneSeq) > 1 { // sanity: Capture was invoked
		t.Error("expected at least one Capture within the clamped interval")
	}
}

func TestEngineRun_StepTimeoutAbort(t *testing.T) {
	r := Recipe{Name: "x", Steps: []Step{
		cmdStep("never", "/go", Await{Kind: AwaitAnyOf, Substrs: []string{"nope"}, TimeoutS: 4, IntervalS: 2}),
	}}
	d := &fakeDriver{paneSeq: []string{"still working"}}
	got, err := newEngine(d).Run(context.Background(), r, "claude-tmux", nil)
	if !errors.Is(err, ErrStepTimeout) {
		t.Fatalf("err=%v want ErrStepTimeout", err)
	}
	if got.Status != StatusFailed || got.Steps[0].Status != StepTimedOut {
		t.Fatalf("result=%+v", got)
	}
}

func TestEngineRun_StepTimeoutContinue(t *testing.T) {
	r := Recipe{Name: "x", Steps: []Step{
		{Name: "soft", Send: Send{Kind: KindCommand, Body: "/go"}, Await: Await{Kind: AwaitAnyOf, Substrs: []string{"nope"}, TimeoutS: 4, IntervalS: 2}, OnTimeout: OnTimeoutContinue},
		cmdStep("next", "/done", Await{Kind: AwaitPromptMarker, TimeoutS: 4, IntervalS: 2}),
	}}
	d := &fakeDriver{paneSeq: []string{"still working", "still working", "❯"}}
	got, err := newEngine(d).Run(context.Background(), r, "claude-tmux", nil)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if got.Steps[0].Status != StepTimedOutContinued || got.Steps[1].Status != StepOK {
		t.Fatalf("result=%+v", got)
	}
}

func TestEngineRun_MidStepModalThenSatisfied(t *testing.T) {
	// AutoRespond fires (dismisses a modal) on tick 1; await satisfied tick 2.
	r := Recipe{Name: "x", Steps: []Step{
		cmdStep("go", "/go", Await{Kind: AwaitAnyOf, Substrs: []string{"done"}, TimeoutS: 10, IntervalS: 1}),
	}}
	d := &fakeDriver{paneSeq: []string{"modal up", "done"}}
	got, err := newEngine(d).Run(context.Background(), r, "claude-tmux", nil)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if got.Status != StatusComplete {
		t.Fatalf("result=%+v", got)
	}
	if d.autoCalls < 2 {
		t.Errorf("auto-responder should keep firing each tick, calls=%d", d.autoCalls)
	}
}

func TestEngineRun_AutoRespondEscalation(t *testing.T) {
	r := Recipe{Name: "x", Steps: []Step{
		cmdStep("go", "/go", Await{Kind: AwaitAnyOf, Substrs: []string{"done"}, TimeoutS: 10, IntervalS: 1}),
	}}
	d := &fakeDriver{paneSeq: []string{"stuck"}, autoEscalateAt: 1}
	_, err := newEngine(d).Run(context.Background(), r, "claude-tmux", nil)
	if !errors.Is(err, ErrAutoRespondEscalation) {
		t.Fatalf("err=%v want ErrAutoRespondEscalation", err)
	}
}

func TestEngineRun_AutoRespondError(t *testing.T) {
	r := Recipe{Name: "x", Steps: []Step{cmdStep("go", "/go", Await{Kind: AwaitPromptMarker, TimeoutS: 6, IntervalS: 2})}}
	d := &fakeDriver{paneSeq: []string{"x"}, autoErr: errors.New("tmux gone")}
	_, err := newEngine(d).Run(context.Background(), r, "claude-tmux", nil)
	if err == nil || errors.Is(err, ErrStepTimeout) {
		t.Fatalf("err=%v want auto-respond error", err)
	}
}

func TestEngineRun_FailRegex(t *testing.T) {
	r := Recipe{Name: "x", Steps: []Step{
		cmdStep("install", "/install x", Await{Kind: AwaitAnyOf, Substrs: []string{"installed"}, FailRegex: "not found", TimeoutS: 10, IntervalS: 1}),
	}}
	d := &fakeDriver{paneSeq: []string{"plugin not found"}}
	got, err := newEngine(d).Run(context.Background(), r, "claude-tmux", nil)
	if !errors.Is(err, ErrAwaitFailRegex) {
		t.Fatalf("err=%v want ErrAwaitFailRegex", err)
	}
	if got.Steps[0].Status != StepFailed {
		t.Fatalf("step status=%s", got.Steps[0].Status)
	}
}

func TestEngineRun_ContextCancel(t *testing.T) {
	r := Recipe{Name: "x", Steps: []Step{cmdStep("go", "/go", Await{Kind: AwaitPromptMarker, TimeoutS: 10, IntervalS: 1})}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	d := &fakeDriver{paneSeq: []string{"working"}}
	_, err := newEngine(d).Run(ctx, r, "claude-tmux", nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v want context.Canceled", err)
	}
}

func TestEngineRun_EnsureSessionError(t *testing.T) {
	r := Recipe{Name: "x", Steps: []Step{cmdStep("go", "/go", Await{Kind: AwaitPromptMarker, TimeoutS: 4})}}
	d := &fakeDriver{ensureErr: errors.New("no tmux")}
	_, err := newEngine(d).Run(context.Background(), r, "claude-tmux", nil)
	if err == nil {
		t.Fatal("want ensure-session error")
	}
}

func TestEngineRun_UnsupportedCLI(t *testing.T) {
	r := Recipe{Name: "x", PerCLI: map[string][]Step{"claude-tmux": {cmdStep("a", "/a", Await{Kind: AwaitPromptMarker, TimeoutS: 4})}}}
	d := &fakeDriver{}
	_, err := newEngine(d).Run(context.Background(), r, "ollama-tmux", nil)
	if !errors.Is(err, ErrUnsupportedCLI) {
		t.Fatalf("err=%v want ErrUnsupportedCLI", err)
	}
	if len(d.commands) != 0 {
		t.Error("no commands should be sent for unsupported CLI")
	}
}

func TestEngineRun_MissingParam(t *testing.T) {
	r := Recipe{Name: "x", Params: []ParamDecl{{Name: "p", Required: true}}, Steps: []Step{cmdStep("a", "/a {{p}}", Await{Kind: AwaitPromptMarker, TimeoutS: 4})}}
	d := &fakeDriver{}
	_, err := newEngine(d).Run(context.Background(), r, "claude-tmux", nil)
	if !errors.Is(err, ErrMissingParam) {
		t.Fatalf("err=%v want ErrMissingParam", err)
	}
}

func TestEngineRun_KeysKindAndDefaultName(t *testing.T) {
	r := Recipe{Name: "x", Steps: []Step{
		{Send: Send{Kind: KindKeys, Body: "Down Down Enter"}, Await: Await{Kind: AwaitPromptMarker, TimeoutS: 6, IntervalS: 1}},
	}}
	d := &fakeDriver{paneSeq: []string{"❯"}}
	got, err := newEngine(d).Run(context.Background(), r, "claude-tmux", nil)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(d.keys) != 1 || d.keys[0] != "Down Down Enter" {
		t.Fatalf("keys=%v", d.keys)
	}
	if got.Steps[0].Name != "step-1" {
		t.Errorf("default step name=%q want step-1", got.Steps[0].Name)
	}
}

func TestEngineRun_SendError(t *testing.T) {
	r := Recipe{Name: "x", Steps: []Step{cmdStep("go", "/go", Await{Kind: AwaitPromptMarker, TimeoutS: 4})}}
	d := &fakeDriver{sendErr: errors.New("paste failed")}
	_, err := newEngine(d).Run(context.Background(), r, "claude-tmux", nil)
	if err == nil {
		t.Fatal("want send error")
	}
}

func TestEngineRun_UnresolvedParamAtStep(t *testing.T) {
	// Param declared optional+unset, body references it → substitute errors at step time.
	r := Recipe{Name: "x", Params: []ParamDecl{{Name: "p"}}, Steps: []Step{cmdStep("a", "/a {{p}}", Await{Kind: AwaitPromptMarker, TimeoutS: 4})}}
	d := &fakeDriver{}
	_, err := newEngine(d).Run(context.Background(), r, "claude-tmux", nil)
	if !errors.Is(err, ErrUnknownParam) {
		t.Fatalf("err=%v want ErrUnknownParam", err)
	}
	if len(d.commands) != 0 {
		t.Error("no command should be sent when substitution fails")
	}
}

func TestEngine_LogfNilSafe(t *testing.T) {
	e := &Engine{} // Log nil
	e.logf("ignored %d", 1)
}

func TestEngine_LogfWritesWhenSet(t *testing.T) {
	var buf bytes.Buffer
	r := Recipe{Name: "x", Steps: []Step{cmdStep("go", "/go", Await{Kind: AwaitPromptMarker, TimeoutS: 6, IntervalS: 1})}}
	d := &fakeDriver{paneSeq: []string{"❯"}}
	e := &Engine{Driver: d, Clock: noClock{}, PromptMarker: "❯", Log: &buf}
	if _, err := e.Run(context.Background(), r, "claude-tmux", nil); err != nil {
		t.Fatalf("err=%v", err)
	}
	if buf.Len() == 0 {
		t.Error("expected diagnostics written to Log")
	}
}

func TestEngineRun_SendKeysError(t *testing.T) {
	r := Recipe{Name: "x", Steps: []Step{
		{Send: Send{Kind: KindKeys, Body: "Enter"}, Await: Await{Kind: AwaitPromptMarker, TimeoutS: 4}},
	}}
	d := &fakeDriver{sendErr: errors.New("send-keys failed")}
	if _, err := newEngine(d).Run(context.Background(), r, "claude-tmux", nil); err == nil {
		t.Fatal("want send-keys error")
	}
}

func TestEngineRun_CaptureError(t *testing.T) {
	r := Recipe{Name: "x", Steps: []Step{cmdStep("go", "/go", Await{Kind: AwaitPromptMarker, TimeoutS: 6, IntervalS: 1})}}
	d := &fakeDriver{captureErr: errors.New("pane vanished")}
	if _, err := newEngine(d).Run(context.Background(), r, "claude-tmux", nil); err == nil {
		t.Fatal("want capture error")
	}
}

func TestEngineRun_DefensiveBadAwaitCompile(t *testing.T) {
	// A Recipe constructed in-memory (bypassing loader validate) with an
	// uncompilable await regex must fail at the runStep compile guard, not panic.
	r := Recipe{Name: "x", Steps: []Step{
		cmdStep("go", "/go", Await{Kind: AwaitRegex, Regex: "([", TimeoutS: 4}),
	}}
	d := &fakeDriver{}
	if _, err := newEngine(d).Run(context.Background(), r, "claude-tmux", nil); !errors.Is(err, ErrInvalidRecipe) {
		t.Fatalf("err=%v want ErrInvalidRecipe", err)
	}
	if len(d.commands) != 0 {
		t.Error("no command should be sent when await fails to compile")
	}
}
