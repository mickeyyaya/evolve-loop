//go:build evolve_test_phases

// A build tag, not an env-var gate, keeps this test-only phase entirely out
// of the production binary's symbol table.
package main

import (
	"context"
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
)

func init() {
	// registry.Register wires this test phase into the resolution path
	// evolve serve-phase (phasecmd.RunServePhase → registry.For) uses.
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
