package main

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/launchoutcome"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/selfheal"
)

const deniedStallStderr = "[bridge] [claude-tmux] FAIL: completion never signalled (artifact diagnosis.json; stop-review paused after 1 interval(s) of 300s)\n" +
	`[bridge] artifact-timeout: cause=guard_refused reason="guard role denied /repo/go/internal/guards/role.go: role guard: phase=selfheal-orchestrator may not write outside workspace" ` +
	`phase=selfheal-orchestrator cycle=0 driver=claude-tmux artifact="diagnosis.json" waited=300s interval=300s extends_used=0 max_extends=6 last_review=pause liveness=idle progressed=false busy=false transient=false detector_error=""` + "\n"

const plainStallStderr = "[bridge] [claude-tmux] FAIL: completion never signalled (artifact diagnosis.json; stop-review paused after 1 interval(s) of 300s)\n" +
	`[bridge] artifact-timeout: cause=review_pause reason="the pane shows no progress" ` +
	`phase=selfheal-orchestrator cycle=0 driver=claude-tmux artifact="diagnosis.json" waited=300s interval=300s extends_used=0 max_extends=6 last_review=pause liveness=idle progressed=false busy=false transient=false detector_error=""` + "\n"

type stallBridge struct {
	mu       sync.Mutex
	stderr   string
	requests []core.BridgeRequest
}

func (b *stallBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	b.mu.Lock()
	b.requests = append(b.requests, req)
	b.mu.Unlock()
	out := launchoutcome.Classify(launchoutcome.ExitArtifactTimeout, nil, b.stderr)
	return core.BridgeResponse{ExitCode: out.ExitCode, CauseCode: out.CauseCode, Stderr: b.stderr}, out.Err
}

func (b *stallBridge) Probe(context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

func (b *stallBridge) incidents() map[string]int {
	seen := map[string]int{}
	for _, req := range b.requests {
		seen[incidentIDPattern.FindString(req.Prompt)]++
	}
	return seen
}

func TestJourney_AGuardDenialEndsTheChainAndTheSelfhealPass(t *testing.T) {
	root := proposeProject(t, `{}`)
	b := &stallBridge{stderr: deniedStallStderr}
	env := selfhealTestEnv(fixtureGH)
	env.agent = fakeBridgeAgent(t, b)

	rc, e, stderr := runProposeJSON(t, env, root)

	if rc != 0 || len(e.Data.Proposals) != 2 {
		t.Fatalf("rc=%d proposals=%+v\n%s", rc, e.Data.Proposals, stderr)
	}
	if len(b.requests) != 1 {
		t.Fatalf("launches %v: a denied agent must end the chain after one stall limit, and the second diagnosis must not start", b.incidents())
	}
	refused, skipped := e.Data.Proposals[0], e.Data.Proposals[1]
	if refused.State != selfheal.ProposalFailed || refused.Cause != string(launchoutcome.TimeoutGuardRefused) {
		t.Fatalf("the denied diagnosis: %+v", refused)
	}
	if !strings.Contains(refused.Error, "guard role denied /repo/go/internal/guards/role.go") || strings.Contains(refused.Error, "quota") {
		t.Fatalf("the summary %q must name the guard and the path and never say quota", refused.Error)
	}
	if skipped.State != selfheal.ProposalSkipped || skipped.Cause != refused.Cause || skipped.RunLabel != "" {
		t.Fatalf("each other pending incident is recorded as skipped with the reason: %+v", skipped)
	}
	stored, err := selfheal.LoadProposals(selfheal.ProposalsDir(root + "/.evolve"))
	if err != nil || stored[skipped.IncidentID].State != selfheal.ProposalSkipped {
		t.Fatalf("the skipped record must be stored: %v %+v", err, stored)
	}
}

func TestJourney_AStallWithNoDenialKeepsTheFallbackChainAndThePass(t *testing.T) {
	root := proposeProject(t, `{}`)
	b := &stallBridge{stderr: plainStallStderr}
	env := selfhealTestEnv(fixtureGH)
	env.agent = fakeBridgeAgent(t, b)

	rc, e, stderr := runProposeJSON(t, env, root)

	seen := b.incidents()
	if rc != 0 || len(e.Data.Proposals) != 2 || len(seen) != 2 {
		t.Fatalf("rc=%d launches=%v\n%s", rc, seen, stderr)
	}
	for id, launches := range seen {
		if launches < 2 {
			t.Errorf("%s: %d launch(es); a stall with no denial still walks to the next family", id, launches)
		}
	}
	for _, p := range e.Data.Proposals {
		if p.State != selfheal.ProposalFailed || p.Cause == string(launchoutcome.TimeoutGuardRefused) {
			t.Errorf("a plain stall is a failure with no guard cause: %+v", p)
		}
	}
}
