// Package fleet is the ADR-0049 S6 concurrent-cycle supervisor: it launches and
// reaps N evolve cycles that run at the SAME time. Each cycle runs in its own
// process with EVOLVE_FLEET=1, so the orchestrator skips the whole-cycle global
// project lock (root-cause R1, orchestrator.fleetMode) and the per-resource
// flocks the safety-net slices put in place — state.json (S2), the ledger chain
// (CA.1), the .evolve/ship.lock integrator (S5) — serialize the shared writes,
// while each cycle's per-run worktree/workspace + run-scoped ship reads (S3) and
// audit binding (S4) keep it isolated. The supervisor is the missing PRODUCER
// for the EVOLVE_FLEET flag (the bridge consumer guard already exists).
package fleet

import (
	"context"
	"errors"
	"sync"
	"time"
)

// fleetEnvKey is forced to "1" on every launched cycle so RunCycle skips the
// global project lock (orchestrator.fleetMode).
const fleetEnvKey = "EVOLVE_FLEET"

// errNoLaunch surfaces a misconfigured supervisor instead of a silent no-op.
var errNoLaunch = errors.New("fleet: no LaunchFn configured")

// fleetScopeEnvKey carries this cycle's assigned todo IDs (comma-joined) to the
// launched `evolve cycle run`, so its triage selects only its disjoint subset
// (ADR-0049 E). Empty / unset ⇒ the cycle works the whole backlog (legacy).
const fleetScopeEnvKey = "EVOLVE_FLEET_SCOPE"

// CycleSpec describes one cycle the supervisor will launch.
type CycleSpec struct {
	GoalHash string   // --goal-hash for `evolve cycle run`
	Scope    []string // todo IDs this cycle owns (disjoint across specs); also in Env[fleetScopeEnvKey]
	// OutputContract is the plan's per-cycle done-definition, threaded to the
	// launched cycle as --goal (Context["goal"]) so the scout executes the PLANNED
	// work instead of free-choosing a task. Combined when a cycle merges several
	// file-sharing todos. Empty = goal-hash only (the generic campaign goal).
	OutputContract string
	Env            map[string]string // base env overlay; EVOLVE_FLEET is forced on
	// Optional is true only when EVERY todo in this cycle's group is optional, so
	// an exhausted failure quarantines+continues rather than aborting the campaign.
	Optional bool
}

// Result is one launched cycle's outcome (input order).
type Result struct {
	Index    int
	ExitCode int
	Err      error
}

// LaunchFn launches one cycle to completion and returns its process exit code.
// Production wiring execs `evolve cycle run --goal-hash <h>` with spec.Env; tests
// inject a fake.
type LaunchFn func(ctx context.Context, spec CycleSpec) (exitCode int, err error)

// Supervisor launches a fleet of concurrent cycles.
type Supervisor struct {
	Launch      LaunchFn
	Concurrency int // max concurrent cycles; <=0 → all at once
	// CycleTimeout bounds each launch: a positive value gives every cycle its own
	// deadline, so a wedged child (e.g. a tmux REPL that never boots) is reaped
	// instead of hanging the whole wave forever. 0 = no per-cycle deadline.
	CycleTimeout time.Duration
}

// Validate reports a misconfigured supervisor — a nil LaunchFn — so the caller
// fails loud at check time rather than after N goroutines each return errNoLaunch.
func (s *Supervisor) Validate() error {
	if s.Launch == nil {
		return errNoLaunch
	}
	return nil
}

// Run launches every spec concurrently (bounded by Concurrency), forcing
// EVOLVE_FLEET=1 on each, waits for all, and returns per-cycle results in input
// order. The caller's spec.Env is never mutated (each launch gets a copy). A nil
// Launch is caught up front by Validate — every result carries errNoLaunch and
// no launch goroutines are spawned (fail loud, never a silent no-op).
func (s *Supervisor) Run(ctx context.Context, specs []CycleSpec) []Result {
	results := make([]Result, len(specs))
	if len(specs) == 0 {
		return results
	}
	if err := s.Validate(); err != nil {
		for i := range results {
			results[i] = Result{Index: i, ExitCode: -1, Err: err}
		}
		return results
	}
	limit := s.Concurrency
	if limit <= 0 || limit > len(specs) {
		limit = len(specs)
	}
	sema := make(chan struct{}, limit)
	var wg sync.WaitGroup
	for i, spec := range specs {
		i, spec := i, spec
		wg.Add(1)
		go func() {
			defer wg.Done()
			sema <- struct{}{}
			defer func() { <-sema }()
			results[i] = s.launchOne(ctx, i, spec)
		}()
	}
	wg.Wait()
	return results
}

func (s *Supervisor) launchOne(ctx context.Context, i int, spec CycleSpec) Result {
	if s.Launch == nil {
		return Result{Index: i, ExitCode: -1, Err: errNoLaunch}
	}
	// Copy the env so the caller's map isn't mutated, then force fleet mode.
	env := make(map[string]string, len(spec.Env)+1)
	for k, v := range spec.Env {
		env[k] = v
	}
	env[fleetEnvKey] = "1"
	spec.Env = env

	if s.CycleTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.CycleTimeout)
		defer cancel()
	}
	code, err := s.Launch(ctx, spec)
	return Result{Index: i, ExitCode: code, Err: err}
}
