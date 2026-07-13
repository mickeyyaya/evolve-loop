package core

// ship_recovery_width_test.go — RED contract for width-scaled-binding-retry
// (cycle 765, inbox weight 0.93, cycle-759 incident: AUDIT_BINDING_HEAD_MOVED
// exhausted a FIXED budget of 2 recoveries and aborted a clean cycle under
// normal width-3 landing-queue contention).
//
// Contract encoded here (Builder implements; DO NOT modify these tests):
//
//  1. The fleet supervisor advertises lane width via the SSOT IPC env key
//     ipcenv.FleetWidthKey ("EVOLVE_FLEET_WIDTH"), read from CycleRequest.Env
//     (never os.Getenv — fleet siblings must not leak into each other).
//  2. shipRecoveryBudget(code ShipErrorCode, fleetWidth int) int is the pure
//     budget classifier: contention-class codes (every AUDIT_BINDING_* code
//     and GIT_FLEET_REBASE_NEEDED) get max(2, fleetWidth+1); every other
//     code keeps the constant maxRecoveryDepth. Absent/garbage/non-positive
//     width resolves to 1 (solo), i.e. the constant budget.
//  3. Between contention re-audit attempts the orchestrator sleeps a JITTERED
//     positive backoff through the existing backoffSleep seam (so TestMain's
//     no-op keeps the suite fast and siblings don't re-collide in lockstep).
//     Jitter must be drawn at millisecond granularity or finer so distinct
//     attempts virtually never sleep identical durations.
//
// White-box (package core): reuses the internal fakes from orchestrator_test.go
// and the backoffSleep seam, which the external core_test package cannot reach.

import (
	"context"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

// widthShipStub returns errOnFail for its first failFirst calls, then PASS.
// Mirrors core_test's shipErrorStub (that harness lives in the external test
// package and cannot be imported here without a cycle).
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

// runWidthRecoveryCycle runs one full cycle over the internal fakes with the
// given ship runner and Env, returning RunCycle's error.
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

// persistentContentionShip builds a ship runner that always fails with the
// cycle-759 contention error (a sibling landed during the audit→ship gap).
func persistentContentionShip() *widthShipStub {
	return &widthShipStub{
		failFirst: 99,
		errOnFail: NewShipError(CodeAuditBindingHeadMoved, ShipClassPrecondition,
			StageVerifyClass, "sibling landed during audit→ship gap"),
	}
}

// AC1 (RED): recovery attempts for contention-class ship errors scale with
// fleet width — budget = max(2, fleetWidth+1) — so a width-N fleet is not
// aborted by pure landing-queue contention after a constant 2 attempts.
// Ship call count = 1 initial + budget (each recovery re-audits then re-ships).
func TestShipRecovery_ContentionBudgetScalesWithFleetWidth(t *testing.T) {
	cases := []struct {
		name          string
		width         string // "" = env key absent
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

// AC2 (RED): between contention re-audit attempts the orchestrator applies a
// jittered positive backoff through the backoffSleep seam so fleet siblings
// don't re-collide in lockstep. NOT t.Parallel: swaps the package seam
// (same save/restore discipline as the executeRetryBackoff unit tests).
func TestShipRecovery_JitteredBackoffBetweenReaudits(t *testing.T) {
	prev := backoffSleep
	var sleeps []time.Duration
	backoffSleep = func(d time.Duration) { sleeps = append(sleeps, d) }
	defer func() { backoffSleep = prev }()

	ship := persistentContentionShip()
	// width 4 → budget max(2,5) = 5 contention recoveries → ≥2 inter-attempt
	// backoffs even under the loosest "between attempts" reading.
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
	// Anti-lockstep: with millisecond-or-finer jitter, >=5 draws collapsing to
	// one identical duration is (deliberately) astronomically unlikely.
	if len(distinct) < 2 {
		t.Errorf("all %d backoff sleeps were identical (%v) — backoff is not jittered", len(sleeps), sleeps[0])
	}
}

// AC3: non-contention transients keep the CONSTANT budget — fleet width must
// not inflate retries for errors that aren't landing-queue contention.
// (Behaviorally green pre-change; pinned so Builder's scaling cannot leak
// beyond the contention class. RED today via this file's compile dependency
// on the new contract symbols.)
func TestShipRecovery_NonContentionTransientKeepsConstantBudget(t *testing.T) {
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

// AC1b (RED): shipRecoveryBudget is the pure budget classifier. Pins
// GIT_FLEET_REBASE_NEEDED as contention-class WITHOUT needing a live git
// rebase (recoverFromShipError performs a real rebase for that code, which a
// unit cycle cannot stage), plus the solo/garbage-width floor.
func TestShipRecovery_BudgetClassifierScalesOnlyContentionCodes(t *testing.T) {
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
