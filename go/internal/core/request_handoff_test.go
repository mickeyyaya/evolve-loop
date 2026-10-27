package core

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestDispatch_HandsTheRequestsOfTheFinishedPhaseToTheChannel(t *testing.T) {
	t.Parallel()
	var cells []RequestCell
	var handoff RequestHandoff = func(_ context.Context, cell RequestCell) error {
		cells = append(cells, cell)
		return nil
	}
	cr := dispatchRunE4(t, &fakeLedger{}, &capturingRunner{}, PhaseBuild, 1, WithRequestHandoff(handoff))

	if _, act, err := cr.dispatch(PhaseBuild); err != nil || act != loopNext {
		t.Fatalf("dispatch = %v %v", act, err)
	}

	if !cr.o.RequestHandoffWired() || NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil)).RequestHandoffWired() {
		t.Fatal("RequestHandoffWired must report the port of WithRequestHandoff only")
	}
	want := RequestCell{ProjectRoot: cr.req.ProjectRoot, Workspace: cr.cs.WorkspacePath, Cycle: 5, Phase: "build"}
	if len(cells) != 1 || cells[0] != want {
		t.Fatalf("the pipeline must read the requests of the build cell once at its handoff, got %+v, want %+v", cells, want)
	}
}

func TestDispatch_AFailedRequestHandoffWarnsAndTheCycleGoesOn(t *testing.T) {
	t.Parallel()
	var stderr strings.Builder
	handoff := func(context.Context, RequestCell) error { return errors.New("request dispatch-1: permission denied") }
	cr := dispatchRunE4(t, &fakeLedger{}, &capturingRunner{}, PhaseBuild, 1, WithRequestHandoff(handoff), WithStderr(&stderr))

	if _, act, err := cr.dispatch(PhaseBuild); err != nil || act != loopNext {
		t.Fatalf("a request store failure must not stop the cycle: %v %v", act, err)
	}
	if !strings.Contains(stderr.String(), "WARN request channel of build: request dispatch-1: permission denied") {
		t.Fatalf("the failure must be loud, got %q", stderr.String())
	}
}
