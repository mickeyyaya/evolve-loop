package core

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestIsQuotaWall_AnUndeliveredPromptIsNoWall(t *testing.T) {
	t.Parallel()
	raw := errors.New("bridge: launch exit=85: unknown interactive prompt")
	for name, tc := range map[string]struct {
		err  error
		wall bool
	}{
		"a bare exit 85 stays a wall":        {raw, true},
		"a rate limit is a wall":             {LaunchCause{Cause: "rate_limit", Err: raw}, true},
		"an undelivered prompt is not":       {LaunchCause{Cause: "unknown_prompt", Err: raw}, false},
		"a walk around the cause keeps it":   {WalkError{CLIs: []string{"agy-tmux"}, Err: LaunchCause{Cause: "unknown_prompt", Err: raw}}, false},
		"another exit is never a quota wall": {LaunchCause{Cause: "rate_limit", Err: errors.New("bridge: launch exit=81: x")}, false},
	} {
		if got := isQuotaWall(tc.err); got != tc.wall {
			t.Errorf("%s: isQuotaWall = %v, want %v", name, got, tc.wall)
		}
	}
	if (LaunchCause{Cause: "unknown_prompt", Err: raw}).Error() != raw.Error() || !errors.Is(LaunchCause{Err: raw}, raw) {
		t.Error("LaunchCause keeps the text and the chain of the error it wraps")
	}
}

func TestRunCycle_AnUndeliveredPromptAtEveryAttemptDoesNotPauseForQuota(t *testing.T) {
	prevHook := QuotaBoundaryCheckpointer
	defer func() { QuotaBoundaryCheckpointer = prevHook }()
	paused := 0
	QuotaBoundaryCheckpointer = func(CycleState, string, time.Time) error {
		paused++
		return nil
	}
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	runners := buildRunners(nil)
	runners[PhaseScout] = &fakeRunner{name: "scout", failErr: LaunchCause{Cause: "unknown_prompt", Err: wrapTransient(85)}, failUntil: 99}
	o := NewOrchestrator(st, &fakeLedger{}, runners)

	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir()})

	if errors.Is(err, ErrAllFamiliesExhausted) || paused != 0 {
		t.Fatalf("err = %v, quota checkpoints = %d: an undelivered prompt is no quota wall, so the cycle must not pause for quota", err, paused)
	}
}
