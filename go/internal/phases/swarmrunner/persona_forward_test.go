package swarmrunner

import (
	"context"
	"errors"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/swarm"
)

type probingRunner struct{ err error }

func (probingRunner) Name() string { return "probing" }
func (probingRunner) Run(context.Context, core.PhaseRequest) (core.PhaseResponse, error) {
	return core.PhaseResponse{}, nil
}
func (p probingRunner) PersonaAvailable() error { return p.err }

type plainRunner struct{}

func (plainRunner) Name() string { return "plain" }
func (plainRunner) Run(context.Context, core.PhaseRequest) (core.PhaseResponse, error) {
	return core.PhaseResponse{}, nil
}

// TestDecorator_ForwardsPersonaProber: wrapping must not hide the inner
// runner's persona answer from the planner (token-waste #2).
func TestDecorator_ForwardsPersonaProber(t *testing.T) {
	missing := errors.New("load agent: " + core.ErrAgentDocMissing.Error())
	wrapped := New(probingRunner{err: missing}, nil, swarm.ModeWriter, Config{})
	var prober core.PersonaProber = wrapped
	if err := prober.PersonaAvailable(); !errors.Is(err, missing) {
		t.Fatalf("decorator must forward the inner prober's answer; got %v", err)
	}
	if err := New(plainRunner{}, nil, swarm.ModeWriter, Config{}).PersonaAvailable(); err != nil {
		t.Fatalf("a wrapped non-prober reports available (nil); got %v", err)
	}
}
