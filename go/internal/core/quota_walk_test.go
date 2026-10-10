package core

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"
)

func TestRunCycle_AWalledPauseHandsTheWalkedCLIsToTheCheckpoint(t *testing.T) {
	prevHook := QuotaBoundaryCheckpointer
	t.Cleanup(func() { QuotaBoundaryCheckpointer = prevHook })
	var walked []string
	QuotaBoundaryCheckpointer = func(cs CycleState, _ string, _ time.Time) error {
		walked = cs.QuotaWalkCLIs
		return nil
	}
	walk := WalkError{CLIs: []string{"claude-tmux", "agy-claude-tmux"}, Err: wrapTransient(85)}
	runners := buildRunners(nil)
	runners[PhaseScout] = &fakeRunner{name: "scout", failErr: walk, failUntil: 99}
	o := NewOrchestrator(&fakeStorage{state: State{}}, &fakeLedger{}, runners)

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()})

	if !errors.Is(err, ErrAllFamiliesExhausted) {
		t.Fatalf("err=%v, want the quota pause", err)
	}
	if !slices.Equal(walked, walk.CLIs) {
		t.Errorf("checkpoint saw walked CLIs %v, want %v: the reset evidence must come from the families this walk met", walked, walk.CLIs)
	}
}

func TestWalkError_KeepsTheWrappedErrorsTextAndIdentity(t *testing.T) {
	inner := wrapTransient(85)
	walk := WalkError{CLIs: []string{"claude-tmux"}, Err: inner}

	if walk.Error() != inner.Error() || !errors.Is(walk, inner) {
		t.Errorf("WalkError text=%q is=%v, want the wrapped error unchanged: the walk is metadata, not a new cause", walk.Error(), errors.Is(walk, inner))
	}
}
