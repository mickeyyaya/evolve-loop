//go:build evolve_test_phases

package main

import (
	"context"
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
)

func init() {
	registry.Register("echo", func(req core.PhaseRequest) core.PhaseRunner {
		return &echoPhaseRunner{}
	})
}

type echoPhaseRunner struct{}

func (e *echoPhaseRunner) Name() string { return "echo" }

func (e *echoPhaseRunner) Run(_ context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	return core.PhaseResponse{
		Phase:        "echo",
		Verdict:      core.VerdictPASS,
		ArtifactsDir: fmt.Sprintf("cycle-%d", req.Cycle),
	}, nil
}
