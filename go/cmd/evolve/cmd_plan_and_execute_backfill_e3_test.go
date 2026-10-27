package main

import (
	"bytes"
	"math"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestRunPassInMode_PassesTheRequestToThePhaseRun(t *testing.T) {
	t.Parallel()
	var out, errb bytes.Buffer

	code := runPassInMode("no-such-phase", core.PhaseRequest{Cycle: 3}, "plan", &out, &errb)

	if code != 10 || !strings.Contains(errb.String(), `unknown phase "no-such-phase"`) {
		t.Fatalf("code %d stderr %q, want runPhase's 10 for an unknown phase passed through", code, errb.String())
	}
}

func TestRunPassInMode_AnUnencodableRequestExits1(t *testing.T) {
	t.Parallel()
	var out, errb bytes.Buffer

	code := runPassInMode("build", core.PhaseRequest{UpstreamSignals: map[string]any{"x": math.NaN()}}, "plan", &out, &errb)

	if code != 1 || !strings.Contains(errb.String(), "encode the phase request") {
		t.Fatalf("code %d stderr %q, want 1 and an encode error", code, errb.String())
	}
}
