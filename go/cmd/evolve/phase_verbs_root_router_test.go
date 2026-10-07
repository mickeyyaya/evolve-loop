package main

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/runner"
	"github.com/mickeyyaya/evolve-loop/go/pkg/phaseproto"
)

type passingPhase struct{ ran bool }

func (*passingPhase) Name() string { return "intent" }

func (p *passingPhase) Run(context.Context, core.PhaseRequest) (core.PhaseResponse, error) {
	p.ran = true
	return core.PhaseResponse{Phase: "intent", Verdict: core.VerdictPASS}, nil
}

func declaredPhaseProject(t *testing.T) (string, *passingPhase) {
	t.Helper()
	root := campaignProject(t, `{"cli_routing":{"clis":["agy","claude"],"default":["agy","claude"]}}`)
	stub := &passingPhase{}
	t.Cleanup(registry.SnapshotForTest())
	registry.ResetForTesting()
	registry.Register("intent", func(core.PhaseRequest) core.PhaseRunner { return stub })
	return root, stub
}

func requireDeclaredRootRouter(t *testing.T, verb string, ran bool) {
	t.Helper()
	if !ran || runner.DefaultRouter == nil || runner.DefaultRouter.Policy().CLIRouting == nil {
		t.Fatalf("evolve %s on a declared project: ran=%v router=%v; its runner needs the root router the table compiles to, or the runner refuses", verb, ran, runner.DefaultRouter)
	}
}

func TestEvolvePhase_ADeclaredProjectRunsThroughTheRootRouter(t *testing.T) {
	root, stub := declaredPhaseProject(t)
	req, _ := json.Marshal(core.PhaseRequest{Cycle: 3, ProjectRoot: root, Workspace: t.TempDir()})
	var stdout, stderr bytes.Buffer

	runPhase([]string{"intent"}, bytes.NewReader(req), &stdout, &stderr)

	requireDeclaredRootRouter(t, "phase", stub.ran)
}

func TestEvolveServePhase_ADeclaredProjectRunsThroughTheRootRouter(t *testing.T) {
	root, stub := declaredPhaseProject(t)
	env, err := phaseproto.EncodeRequest("corr", core.PhaseRequest{Cycle: 3, ProjectRoot: root, Workspace: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(env)
	var stdout, stderr bytes.Buffer

	lookupCommand("serve-phase").Run([]string{"intent"}, bytes.NewReader(append(raw, '\n')), &stdout, &stderr)

	requireDeclaredRootRouter(t, "serve-phase", stub.ran)
}

func TestEvolveCompose_ADeclaredProjectRunsThroughTheRootRouter(t *testing.T) {
	root, stub := declaredPhaseProject(t)
	req, _ := json.Marshal(core.PhaseRequest{Cycle: 3, ProjectRoot: root, Workspace: t.TempDir()})
	var stdout, stderr bytes.Buffer

	runCompose([]string{"--phases", "intent"}, bytes.NewReader(req), &stdout, &stderr)

	requireDeclaredRootRouter(t, "compose", stub.ran)
}
