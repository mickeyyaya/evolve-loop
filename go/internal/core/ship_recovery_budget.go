package core

import (
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

// Width-scaled contention recovery (cycle-765, cycle-759 incident): with N
// fleet lanes racing one main, P(HEAD moves during your audit→ship window)
// grows with N, but the recovery budget was a constant maxRecoveryDepth=2 —
// at width 3+ that guarantees a steady abort rate that looks like "loop
// failure" while being pure landing-queue contention. Contention-class codes
// get a budget that scales with fleet width; everything else keeps the
// constant budget so width can never inflate retries for genuine failures.

// isContentionShipCode reports whether a ship error is landing-queue
// contention — a sibling lane moved main between this lane's audit and ship —
// rather than a defect in this lane's own work: every AUDIT_BINDING_* code
// plus GIT_FLEET_REBASE_NEEDED.
func isContentionShipCode(code ShipErrorCode) bool {
	return strings.HasPrefix(string(code), "AUDIT_BINDING_") || code == CodeGitFleetRebaseNeeded
}

// shipRecoveryBudget is the pure recovery-budget classifier: contention-class
// codes get max(maxRecoveryDepth, fleetWidth+1) — each sibling can bump HEAD
// once, plus one clean pass — while every other code keeps the constant
// maxRecoveryDepth. A non-positive width means solo, i.e. the constant budget.
func shipRecoveryBudget(code ShipErrorCode, fleetWidth int) int {
	if !isContentionShipCode(code) {
		return maxRecoveryDepth
	}
	if b := fleetWidth + 1; b > maxRecoveryDepth {
		return b
	}
	return maxRecoveryDepth
}

// fleetWidthFromEnv resolves the supervisor-advertised lane width from the
// cycle request's env overlay (ipcenv.FleetWidthKey). Deliberately NOT
// os.Getenv: CycleRequest.Env is the per-lane IPC contract, so siblings cannot
// leak width into each other. Absent/garbage/non-positive ⇒ 1 (solo).
func fleetWidthFromEnv(env map[string]string) int {
	w, err := strconv.Atoi(strings.TrimSpace(env[ipcenv.FleetWidthKey]))
	if err != nil || w < 1 {
		return 1
	}
	return w
}

// contentionBackoffBase/Cap bound the jittered inter-attempt backoff: linear
// growth per attempt, capped so recovery stays well under a lane's phase
// budget even at the widest scaled depth.
const (
	contentionBackoffBase = 2 * time.Second
	contentionBackoffCap  = 30 * time.Second
)

// contentionBackoff draws the jittered pause before contention recovery
// attempt depth+1. Full jitter over a linearly-growing window ((depth+1)×base,
// capped) desynchronizes siblings that hit the same HEAD-moved error in
// lockstep — a fixed backoff would just re-collide them on the next attempt.
// Nanosecond-granularity draws over a multi-second window make identical
// durations across attempts astronomically unlikely; always positive.
func contentionBackoff(depth int) time.Duration {
	window := time.Duration(depth+1) * contentionBackoffBase
	if window > contentionBackoffCap {
		window = contentionBackoffCap
	}
	return time.Millisecond + time.Duration(rand.Int64N(int64(window)))
}
