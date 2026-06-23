//go:build evolve_test_phases

// Test-only phase registration. Compiled into the `evolve` binary only
// when built with `-tags evolve_test_phases`, which the serve-phase
// subprocess integration test does. Production builds never include it,
// so there is zero runtime surface in shipped binaries.
//
// Why a build tag and not an env-var-gated registration: keeps the
// echo phase entirely out of the production binary's symbol table, and
// makes the dependency explicit in `go build` invocations rather than
// hiding it behind runtime state.
package main

import (
	"context"
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
)

func init() {
	// Register into the phases registry that `evolve serve-phase`
	// (phasecmd.RunServePhase → registry.For) resolves against. The former
	// package-local `phaseFactories` map was removed when serve-phase moved to
	// internal/cli/phasecmd, which silently bit-rotted this test build
	// (undefined: phaseFactories) — see ADR-0062/T1.7 follow-up.
	registry.Register("echo", func(req core.PhaseRequest) core.PhaseRunner {
		return &echoPhaseRunner{}
	})
}

// echoPhaseRunner reflects the request's Cycle into ArtifactsDir so
// the test can assert round-trip integrity through the wire.
type echoPhaseRunner struct{}

func (e *echoPhaseRunner) Name() string { return "echo" }

func (e *echoPhaseRunner) Run(_ context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	return core.PhaseResponse{
		Phase:        "echo",
		Verdict:      core.VerdictPASS,
		ArtifactsDir: fmt.Sprintf("cycle-%d", req.Cycle),
	}, nil
}
