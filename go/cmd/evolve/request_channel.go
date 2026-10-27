package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentrequest"
	"github.com/mickeyyaya/evolve-loop/go/internal/cli/agentcmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type agentSeat func(args []string, stdin io.Reader, stdout, stderr io.Writer) int

func runAgentSeat(agent agentSeat, args []string, input string, stderr io.Writer) (agentcmd.Envelope, error) {
	var stdout bytes.Buffer
	rc := agent(append([]string{"run"}, args...), strings.NewReader(input), &stdout, stderr)
	var run agentcmd.Envelope
	if err := json.Unmarshal(stdout.Bytes(), &run); err != nil {
		return run, fmt.Errorf("evolve agent run %s: exit %d with no JSON envelope: %w", args[0], rc, err)
	}
	if rc != 0 {
		return run, fmt.Errorf("evolve agent run %s: exit %d: %s", args[0], rc, run.Error)
	}
	return run, nil
}

func requestHandoff(pol policy.Policy, agent agentSeat, emit func(signalcenter.Event), stderr io.Writer) core.RequestHandoff {
	ch := agentrequest.Channel{
		Policy: pol.IsolationDispatchRequests(),
		Judge: func(_ context.Context, p agentrequest.Pending) (string, error) {
			run, err := runAgentSeat(agent, []string{orchestratorProfile, "--json", "--project-root", p.Cell.ProjectRoot}, judgeInput(p), stderr)
			return run.Report, err
		},
		Dispatch: func(_ context.Context, p agentrequest.Pending) (agentrequest.Result, error) {
			args := []string{p.Request.Role(), "--json", "--keep", "--project-root", p.Cell.ProjectRoot}
			for _, in := range p.Inputs {
				args = append(args, "--file", in)
			}
			run, err := runAgentSeat(agent, args, dispatchInput(p), stderr)
			return agentrequest.Result{Report: run.Report, RunLabel: run.RunLabel, Workspace: run.Workspace}, err
		},
		Emit: emit,
		Now:  time.Now,
	}
	return func(ctx context.Context, cell core.RequestCell) error {
		return ch.Handoff(ctx, agentrequest.Cell(cell))
	}
}

func judgeInput(p agentrequest.Pending) string {
	return fmt.Sprintf("Judge the dispatch request %s of the %s agent of cycle %d (role %s, depth %d).\n\n"+
		"Its evidence workspace is %s. List the evidence index with `evolve context index --workspace %s --json`. "+
		"Open the required entry first with `evolve context open q1 --workspace %s --json`.\n\n"+
		"Write diagnosis.json with the schema %s and the incident_id %s, as your instructions say for a dispatch request.\n",
		p.Name, p.Cell.Phase, p.Cell.Cycle, p.Request.Role(), p.Depth, p.Evidence, p.Evidence, p.Evidence, phasecontract.DiagnosisSchema, p.Name)
}

func dispatchInput(p agentrequest.Pending) string {
	return fmt.Sprintf("The %s agent of cycle %d asked for you through the dispatch request %s.\n\nWhy: %s\n\nExpected output: %s\n\nThe attached files are the inputs of the request.\n",
		p.Cell.Phase, p.Cell.Cycle, p.Name, p.Request.Why, p.Request.ExpectedOutput.Artifact)
}
