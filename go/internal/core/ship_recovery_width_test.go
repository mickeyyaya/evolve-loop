package core

import (
	"context"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

type widthShipStub struct {
	failFirst int
	errOnFail error
	calls     int
}

func (s *widthShipStub) Name() string { return "ship" }
func (s *widthShipStub) Run(_ context.Context, _ PhaseRequest) (PhaseResponse, error) {
	s.calls++
	if s.calls <= s.failFirst {
		return PhaseResponse{Phase: "ship", Verdict: VerdictFAIL}, s.errOnFail
	}
	return PhaseResponse{Phase: "ship", Verdict: VerdictPASS}, nil
}

func runWidthRecoveryCycle(t *testing.T, ship PhaseRunner, env map[string]string) error {
	t.Helper()
	st := &fakeStorage{}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	runners[PhaseShip] = ship
	o := NewOrchestrator(st, led, runners)
	_, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: t.TempDir(),
		GoalHash:    "width-scaled-binding-retry-goal",
		Env:         env,
		Context:     map[string]string{"commit_message": "test commit"},
	})
	return err
}

func persistentContentionShip() *widthShipStub {
	return &widthShipStub{
		failFirst: 99,
		errOnFail: NewShipError(CodeAuditBindingHeadMoved, ShipClassPrecondition,
			StageVerifyClass, "sibling landed during audit→ship gap"),
	}
}

func TestShipRecovery_ContentionBudgetScalesWithFleetWidth(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		width         string
		wantShipCalls int
	}{
		{"width-absent-keeps-constant-budget", "", 3},
		{"width-1-keeps-constant-budget", "1", 3},
		{"width-3-scales-to-4-attempts", "3", 5},
		{"width-5-scales-to-6-attempts", "5", 7},
		{"width-garbage-falls-back-to-constant", "not-a-number", 3},
		{"width-negative-falls-back-to-constant", "-2", 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ship := persistentContentionShip()
			env := map[string]string{}
			if tc.width != "" {
				env[ipcenv.FleetWidthKey] = tc.width
			}
			err := runWidthRecoveryCycle(t, ship, env)
			if err == nil {
				t.Fatal("a never-resolving contention error must still abort once the scaled budget is spent")
			}
			if ship.calls != tc.wantShipCalls {
				t.Errorf("ship calls = %d, want %d (1 initial + max(2, width+1) contention recoveries)",
					ship.calls, tc.wantShipCalls)
			}
		})
	}
}

func TestShipRecovery_JitteredBackoffBetweenReaudits(t *testing.T) {
	prev := backoffSleep
	var sleeps []time.Duration
	backoffSleep = func(d time.Duration) { sleeps = append(sleeps, d) }
	defer func() { backoffSleep = prev }()

	ship := persistentContentionShip()
	err := runWidthRecoveryCycle(t, ship, map[string]string{ipcenv.FleetWidthKey: "4"})
	if err == nil {
		t.Fatal("persistent contention error must abort after the scaled budget")
	}
	if len(sleeps) < 2 {
		t.Fatalf("recorded %d backoff sleeps, want >= 2 (jittered backoff between contention re-audits)", len(sleeps))
	}
	distinct := map[time.Duration]bool{}
	for i, d := range sleeps {
		if d <= 0 {
			t.Errorf("sleep[%d] = %v, want > 0 (a zero backoff re-collides siblings instantly)", i, d)
		}
		if d > 60*time.Second {
			t.Errorf("sleep[%d] = %v, want <= 60s (recovery must stay bounded)", i, d)
		}
		distinct[d] = true
	}
	if len(distinct) < 2 {
		t.Errorf("all %d backoff sleeps were identical (%v) — backoff is not jittered", len(sleeps), sleeps[0])
	}
}

func TestShipRecovery_NonContentionTransientKeepsConstantBudget(t *testing.T) {
	t.Parallel()

	ship := &widthShipStub{
		failFirst: 99,
		errOnFail: NewShipError(CodeGitPushRejected, ShipClassTransient,
			StageAtomicShip, "push race"),
	}
	err := runWidthRecoveryCycle(t, ship, map[string]string{ipcenv.FleetWidthKey: "5"})
	if err == nil {
		t.Fatal("a never-resolving transient must abort once the constant budget is spent")
	}
	if ship.calls != 3 {
		t.Errorf("ship calls = %d, want 3 (1 initial + constant maxRecoveryDepth=2; width must NOT scale non-contention retries)",
			ship.calls)
	}
}

func TestShipRecovery_BudgetClassifierScalesOnlyContentionCodes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		code  ShipErrorCode
		width int
		want  int
	}{
		{"binding-head-moved-width-3", CodeAuditBindingHeadMoved, 3, 4},
		{"fleet-rebase-needed-width-3", CodeGitFleetRebaseNeeded, 3, 4},
		{"binding-head-moved-width-1-floors-at-2", CodeAuditBindingHeadMoved, 1, 2},
		{"binding-head-moved-width-0-floors-at-2", CodeAuditBindingHeadMoved, 0, 2},
		{"transient-push-rejected-never-scales", CodeGitPushRejected, 6, 2},
		{"integrity-tree-drift-never-scales", CodeIntegrityTreeDrift, 6, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shipRecoveryBudget(tc.code, tc.width); got != tc.want {
				t.Errorf("shipRecoveryBudget(%s, %d) = %d, want %d", tc.code, tc.width, got, tc.want)
			}
		})
	}
}
