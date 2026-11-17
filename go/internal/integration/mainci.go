package integration

import (
	"context"
	"errors"
	"fmt"
)

type MainCIState string

const (
	MainGreen         MainCIState = "green"
	MainPending       MainCIState = "pending"
	MainRedOverridden MainCIState = "red, overridden"
)

type MainCI interface {
	Watch(ctx context.Context, pr PR) (Verdict, error)
}

type MainState struct {
	SHA   string
	State MainCIState
}

type MainRefusal struct {
	SHA    string
	Reason string
}

func (r *MainRefusal) Error() string {
	return fmt.Sprintf("main %s is not green (%s), so a batch would test a red base", ShortSHA(r.SHA), r.Reason)
}

func (t Train) checkMain(ctx context.Context, base string) (MainState, error) {
	verdict, err := t.MainCI.Watch(ctx, PR{Tip: base})
	if err == nil && verdict == Green {
		return MainState{SHA: base, State: MainGreen}, nil
	}
	if errors.Is(err, ErrRunPending) {
		return MainState{SHA: base, State: MainPending}, nil
	}
	reason, notGreen := redReason(err)
	if !notGreen {
		return MainState{}, err
	}
	if t.MainRedFix {
		return MainState{SHA: base, State: MainRedOverridden}, nil
	}
	return MainState{}, &MainRefusal{SHA: base, Reason: reason}
}

func redReason(err error) (string, bool) {
	var ended *RunConclusionError
	switch {
	case err == nil:
		return string(Red), true
	case errors.Is(err, ErrNoRun):
		return "no CI run", true
	case errors.As(err, &ended):
		return ended.Conclusion, true
	}
	return "", false
}
