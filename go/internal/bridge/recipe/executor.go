package recipe

import (
	"context"
	"fmt"
	"io"
	"time"
)

// defaultAwaitIntervalS is the poll cadence when a step's await sets none. 2s
// matches the parent driver's artifact-wait tick.
const defaultAwaitIntervalS = 2

// SessionDriver is the minimal tmux surface the engine drives. It is a strict
// subset of the bridge's capabilities, declared HERE (consumer-owned port) so
// the recipe package depends on nothing from bridge. The production
// implementation is a thin adapter in the bridge package.
type SessionDriver interface {
	// EnsureSession launches-or-attaches the target REPL session.
	EnsureSession(ctx context.Context) error
	// Capture returns the current pane contents.
	Capture(ctx context.Context) (string, error)
	// SendCommand pastes body then presses Enter (slash-command idiom).
	SendCommand(ctx context.Context, body string) error
	// SendKeys sends body as raw tmux key tokens, no trailing Enter.
	SendKeys(ctx context.Context, tokens string) error
	// AutoRespond runs one auto-responder tick (dismissing any modal that
	// appeared between steps) and reports whether it escalated/abandoned.
	AutoRespond(ctx context.Context) (escalated bool, err error)
}

// Clock is the sleep seam — a no-op in tests so the poll loop runs instantly.
type Clock interface {
	Sleep(d time.Duration)
}

// Engine executes recipes against a SessionDriver. Construct it with the
// resolved prompt marker (from the CLI's manifest) so AwaitPromptMarker works.
type Engine struct {
	Driver       SessionDriver
	Clock        Clock
	Log          io.Writer // diagnostics; nil discards
	PromptMarker string
}

func (e *Engine) logf(format string, a ...any) {
	if e.Log != nil {
		fmt.Fprintf(e.Log, format, a...)
	}
}

// Run executes r for cli with params. The Template Method skeleton: resolve
// steps → merge params → ensure session → run each step → done. It always
// returns a Result describing how far it got, alongside any error.
func (e *Engine) Run(ctx context.Context, r Recipe, cli string, params Params) (Result, error) {
	res := Result{Recipe: r.Name, CLI: cli, Status: StatusFailed}

	steps, err := r.stepsFor(cli)
	if err != nil {
		return res, err
	}
	merged, err := r.mergeParams(params)
	if err != nil {
		return res, err
	}
	if err := e.Driver.EnsureSession(ctx); err != nil {
		return res, fmt.Errorf("recipe: ensure session: %w", err)
	}

	for i, step := range steps {
		sr, err := e.runStep(ctx, step, merged, i)
		res.Steps = append(res.Steps, sr)
		if err != nil {
			return res, err
		}
	}
	res.Status = StatusComplete
	return res, nil
}

// runStep sends one step's body then polls until its await condition is
// satisfied, fails, or times out — running the auto-responder each tick so
// modals that pop between steps are still handled (never bypassed).
func (e *Engine) runStep(ctx context.Context, step Step, params Params, idx int) (StepResult, error) {
	name := step.Name
	if name == "" {
		name = fmt.Sprintf("step-%d", idx+1)
	}
	sr := StepResult{Name: name, Status: StepFailed}

	body, err := substitute(step.Send.Body, params)
	if err != nil {
		return sr, err
	}
	ca, err := step.Await.compile()
	if err != nil {
		return sr, err // defensive: load-time validate() already checked this
	}

	if err := e.send(ctx, step.Send.Kind, body); err != nil {
		return sr, err
	}
	e.logf("[recipe:%s] step %q sent %q\n", step.Send.Kind, name, body)

	interval := step.Await.IntervalS
	if interval <= 0 {
		interval = defaultAwaitIntervalS
	}
	if interval > step.Await.TimeoutS {
		// Guarantee at least one pane check within the budget — otherwise a
		// recipe with interval_s > timeout_s would sleep past the deadline and
		// never call Capture.
		interval = step.Await.TimeoutS
	}

	outcome := matchPending
	elapsed := 0
	for elapsed < step.Await.TimeoutS {
		e.Clock.Sleep(time.Duration(interval) * time.Second)
		elapsed += interval
		sr.ElapsedS = elapsed

		if err := ctx.Err(); err != nil {
			return sr, err
		}
		escalated, err := e.Driver.AutoRespond(ctx)
		if err != nil {
			return sr, fmt.Errorf("recipe: auto-respond: %w", err)
		}
		if escalated {
			return sr, fmt.Errorf("%w: step %q", ErrAutoRespondEscalation, name)
		}
		pane, err := e.Driver.Capture(ctx)
		if err != nil {
			return sr, fmt.Errorf("recipe: capture: %w", err)
		}
		sr.PaneTail = lastLines(pane, 20)
		outcome = ca.eval(pane, e.PromptMarker)
		if outcome == matchFailed {
			return sr, fmt.Errorf("%w: step %q", ErrAwaitFailRegex, name)
		}
		if outcome == matchSatisfied {
			break
		}
	}

	if outcome != matchSatisfied {
		if step.OnTimeout == OnTimeoutContinue {
			sr.Status = StepTimedOutContinued
			e.logf("[recipe] step %q timed out after %ds; continuing (on_timeout=continue)\n", name, step.Await.TimeoutS)
			return sr, nil
		}
		sr.Status = StepTimedOut
		return sr, fmt.Errorf("%w: step %q after %ds", ErrStepTimeout, name, step.Await.TimeoutS)
	}
	sr.Status = StepOK
	return sr, nil
}

// send dispatches a step body to the correct INJECT transport. An empty/
// unknown kind defaults to command (the safe slash-command idiom).
func (e *Engine) send(ctx context.Context, kind SendKind, body string) error {
	if kind == KindKeys {
		if err := e.Driver.SendKeys(ctx, body); err != nil {
			return fmt.Errorf("recipe: send keys: %w", err)
		}
		return nil
	}
	if err := e.Driver.SendCommand(ctx, body); err != nil {
		return fmt.Errorf("recipe: send command: %w", err)
	}
	return nil
}
