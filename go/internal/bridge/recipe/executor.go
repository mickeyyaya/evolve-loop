package recipe

import (
	"context"
	"fmt"
	"io"
	"time"
)

const defaultAwaitIntervalS = 2

type SessionDriver interface {
	EnsureSession(ctx context.Context) error
	Capture(ctx context.Context) (string, error)
	SendCommand(ctx context.Context, body string) error
	SendKeys(ctx context.Context, tokens string) error
	AutoRespond(ctx context.Context) (escalated bool, err error)
}

type Clock interface {
	Sleep(d time.Duration)
}

type Engine struct {
	Driver       SessionDriver
	Clock        Clock
	Log          io.Writer
	PromptMarker string
}

func (e *Engine) logf(format string, a ...any) {
	if e.Log != nil {
		fmt.Fprintf(e.Log, format, a...)
	}
}

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
		return sr, err
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
