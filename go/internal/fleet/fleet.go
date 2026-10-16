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

type CycleSpec struct {
	GoalHash       string
	Scope          []string
	OutputContract string
	Env            map[string]string
	Optional       bool
}

type Result struct {
	Index    int
	ExitCode int
	Err      error
}

const ExitDeferred = 5

type LaneStatus string

const (
	LaneOK       LaneStatus = "ok"
	LaneDeferred LaneStatus = "deferred"
	LaneFailed   LaneStatus = "FAIL"
)

func (r Result) Status() LaneStatus {
	switch {
	case r.Err == nil && r.ExitCode == 0:
		return LaneOK
	case r.Err == nil && r.ExitCode == ExitDeferred:
		return LaneDeferred
	default:
		return LaneFailed
	}
}

type LaunchFn func(ctx context.Context, spec CycleSpec) (exitCode int, err error)

type Supervisor struct {
	Launch       LaunchFn
	Concurrency  int
	CycleTimeout time.Duration
}

func (s *Supervisor) Validate() error {
	if s.Launch == nil {
		return errNoLaunch
	}
	return nil
}

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
