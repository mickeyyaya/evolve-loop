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

type wiredRunner struct {
	plainRunner
	wired bool
}

func (w wiredRunner) SignalsWired() bool { return w.wired }

// ADR-0103 unit 11 (test 43): a decorator must not narrow the wiring proof of
// what it wraps — SignalsWired forwards to the inner runner, false for a
// runner without the capability.
func TestSwarmDecorator_ForwardsSignalsWired(t *testing.T) {
	if !New(wiredRunner{wired: true}, nil, swarm.ModeWriter, Config{}).SignalsWired() {
		t.Error("a wired inner runner reports wired through the Decorator")
	}
	if New(wiredRunner{wired: false}, nil, swarm.ModeWriter, Config{}).SignalsWired() {
		t.Error("an unwired inner runner reports unwired")
	}
	if New(plainRunner{}, nil, swarm.ModeWriter, Config{}).SignalsWired() {
		t.Error("a runner without the capability reports false")
	}
}
