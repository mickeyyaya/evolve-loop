package main

import (
	"io"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/selfheal"
)

const guardRefusedEnvelope = `{"run_label":"agent-x","error":"bridge: launch exit=81: guard ship denied git push origin main before the launch stalled","cause_code":"guard_refused"}`

func countingAgent(calls *atomic.Int32, stdout string, rc int) func([]string, io.Reader, io.Writer, io.Writer) int {
	return func(args []string, in io.Reader, w io.Writer, e io.Writer) int {
		calls.Add(1)
		return scriptedAgent(stdout, rc)(args, in, w, e)
	}
}

func TestSelfhealPropose_AGuardRefusalStopsThePassAndSkipsTheRestWithoutADecision(t *testing.T) {
	t.Parallel()
	root := validatedProject(t, `{"orchestrator":{"max_per_pass":3}}`)
	var calls atomic.Int32
	env := selfhealTestEnv(fixtureGH)
	env.agent = countingAgent(&calls, guardRefusedEnvelope, 21)

	rc, e, stderr := runProposeJSON(t, env, root)

	if rc != 0 || calls.Load() != 1 || len(e.Data.Proposals) != 3 {
		t.Fatalf("rc=%d agent calls=%d proposals=%d\n%s", rc, calls.Load(), len(e.Data.Proposals), stderr)
	}
	for i, p := range e.Data.Proposals {
		if p.State != selfheal.ProposalSkipped || p.Decision != nil || !strings.Contains(p.Error, core.CauseGuardRefused) {
			t.Errorf("proposal %d: state %q decision %v error %q: skipped, undecided, reason guard_refused", i, p.State, p.Decision, p.Error)
		}
	}
	if !strings.Contains(e.Data.Proposals[0].Error, "guard ship denied git push origin main") {
		t.Errorf("the refused incident keeps the summary of the guard and the path: %q", e.Data.Proposals[0].Error)
	}
}

func TestSelfhealPropose_APlainFailureKeepsThePass(t *testing.T) {
	t.Parallel()
	root := validatedProject(t, `{"orchestrator":{"max_per_pass":3}}`)
	var calls atomic.Int32
	env := selfhealTestEnv(fixtureGH)
	env.agent = countingAgent(&calls, `{"run_label":"agent-x","error":"bridge down","cause_code":"artifact_timeout"}`, 21)

	_, e, _ := runProposeJSON(t, env, root)

	if calls.Load() != 3 || len(e.Data.Proposals) != 3 || slices.ContainsFunc(e.Data.Proposals, func(p selfheal.Proposal) bool { return p.State != selfheal.ProposalFailed }) {
		t.Fatalf("agent calls=%d proposals=%+v: an ordinary stall fails one incident and the pass goes on", calls.Load(), e.Data.Proposals)
	}
}

func TestSelfhealPropose_ASkippedIncidentIsProposedAgainAfterTheBackoff(t *testing.T) {
	root := validatedProject(t, `{"orchestrator":{"max_per_pass":3,"retry_after_hours":2}}`)
	env := selfhealTestEnv(fixtureGH)
	var calls atomic.Int32
	env.agent = countingAgent(&calls, guardRefusedEnvelope, 21)
	_, first, _ := runProposeJSON(t, env, root)
	skipped := map[string]bool{}
	for _, p := range first.Data.Proposals {
		skipped[p.IncidentID] = true
	}

	env.agent = fakeBridgeAgent(t, &diagnosisBridge{})
	_, within, _ := runProposeJSON(t, env, root)
	if slices.ContainsFunc(within.Data.Proposals, func(p selfheal.Proposal) bool { return skipped[p.IncidentID] }) {
		t.Fatalf("within the backoff a skipped incident gives its slot away: %+v", within.Data.Proposals)
	}
	env.now = func() time.Time { return selfhealNow.Add(3 * time.Hour) }
	_, later, _ := runProposeJSON(t, env, root)
	got := 0
	for _, p := range later.Data.Proposals {
		if skipped[p.IncidentID] && p.State != selfheal.ProposalSkipped {
			got++
		}
	}
	if got == 0 {
		t.Fatalf("after the backoff the skipped incidents are proposed again: %+v", later.Data.Proposals)
	}
}
