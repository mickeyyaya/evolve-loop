package integration

import "context"

type StopGate struct {
	Red   func(pr PR) error
	Admit func(ctx context.Context, main, tip string) (landed func(), err error)
}

func (g StopGate) CI(inner CI) CI {
	return stopCI{CI: inner, red: g.Red}
}

func (g StopGate) Main(inner Main) Main {
	return stopMain{Main: inner, admit: g.Admit}
}

type stopCI struct {
	CI
	red func(pr PR) error
}

func (c stopCI) Watch(ctx context.Context, pr PR) (Verdict, error) {
	verdict, err := c.CI.Watch(ctx, pr)
	if err != nil || verdict != Red {
		return verdict, err
	}
	return verdict, c.red(pr)
}

type stopMain struct {
	Main
	admit func(ctx context.Context, main, tip string) (func(), error)
}

func (m stopMain) FastForward(ctx context.Context, tip string) error {
	head, err := m.Head(ctx)
	if err != nil {
		return err
	}
	landed, err := m.admit(ctx, head, tip)
	if err != nil {
		return err
	}
	if err := m.Main.FastForward(ctx, tip); err != nil {
		return err
	}
	landed()
	return nil
}
