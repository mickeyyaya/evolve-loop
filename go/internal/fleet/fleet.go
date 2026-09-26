// Package fleet plans, launches and lands concurrent evolve cycles (lanes).
// See docs/architecture/packages/internal-fleet.md.
package fleet

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

var errNoLaunch = errors.New("fleet: no LaunchFn configured")

// CycleSpec describes one cycle the supervisor will launch.
type CycleSpec struct {
	GoalHash string   // --goal-hash for `evolve cycle run`
	Scope    []string // todo IDs this cycle owns; mirrored in Env[ipcenv.FleetScopeKey]
	// OutputContract is the cycle's done-definition, passed as --goal; empty keeps the goal-hash goal.
	OutputContract string
	Env            map[string]string // base env overlay; EVOLVE_FLEET is forced on
	// Optional marks a cycle whose exhausted failure is quarantined instead of aborting the campaign.
	Optional bool
}

// Result is one launched cycle's outcome (input order).
type Result struct {
	Index    int
	ExitCode int
	Err      error
}

// LaunchFn launches one cycle to completion and returns its process exit code.
type LaunchFn func(ctx context.Context, spec CycleSpec) (exitCode int, err error)

// Supervisor launches a fleet of concurrent cycles.
type Supervisor struct {
	Launch       LaunchFn
	Concurrency  int           // max concurrent cycles; <=0 → all at once
	CycleTimeout time.Duration // per-launch deadline that reaps a wedged child; 0 = none
}

// Validate reports a nil LaunchFn before any launch is scheduled.
func (s *Supervisor) Validate() error {
	if s.Launch == nil {
		return errNoLaunch
	}
	return nil
}

// Run launches every spec in fleet mode, bounded by Concurrency, and returns results in input order.
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
			results[i] = s.launchOne(ctx, i, spec, limit)
		}()
	}
	wg.Wait()
	return results
}

func (s *Supervisor) launchOne(ctx context.Context, i int, spec CycleSpec, width int) Result {
	if s.Launch == nil {
		return Result{Index: i, ExitCode: -1, Err: errNoLaunch}
	}
	// Copy so the caller's map is never mutated; core.shipRecoveryBudget scales with the width.
	env := make(map[string]string, len(spec.Env)+2)
	for k, v := range spec.Env {
		env[k] = v
	}
	env[ipcenv.FleetKey] = "1"
	if width > 0 {
		env[ipcenv.FleetWidthKey] = strconv.Itoa(width)
	}
	spec.Env = env

	if s.CycleTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.CycleTimeout)
		defer cancel()
	}
	code, err := s.Launch(ctx, spec)
	return Result{Index: i, ExitCode: code, Err: err}
}
