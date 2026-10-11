package core

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/advisor"
)

func deadShellThen(stdout string) *scriptedBridge {
	return &scriptedBridge{scriptedLaunches{responses: []BridgeResponse{deadShell, {Stdout: stdout}}}}
}

func assertOneFreshSession(t *testing.T, b *scriptedBridge) {
	t.Helper()
	if len(b.sent) != 2 || !b.sent[1].FreshSession || b.sent[1].Agent != b.sent[0].Agent {
		t.Fatalf("launches = %+v, want the dead session, then one fresh session of the same request", b.sent)
	}
}

func TestPlanJudge_SurvivesADeadShellFirstSession(t *testing.T) {
	b := deadShellThen(`{"score":0.8,"rationale":"covers the bug path","missing_phases":[]}`)

	v := NewPlanJudge(b).GradePlan(context.Background(), judgeInput(), samplePlan())

	assertOneFreshSession(t, b)
	if v.Score != 0.8 {
		t.Fatalf("score = %v, want the fresh session's 0.8", v.Score)
	}
}

func TestFailureAdvisor_SurvivesADeadShellFirstSession(t *testing.T) {
	b := deadShellThen(`{"cause":"dead_shell","pane_substr":"credential vault locked","justification":"the REPL exited to a locked vault prompt"}`)

	adv, err := NewFailureAdvisor(b).Advise(context.Background(), baseFailureInput())

	assertOneFreshSession(t, b)
	if err != nil || adv == nil || adv.Cause != "dead_shell" {
		t.Fatalf("advice = %+v, %v; want the fresh session's verdict", adv, err)
	}
}

func TestBridgeRetryAdjudicator_SurvivesADeadShellFirstSession(t *testing.T) {
	b := deadShellThen(`{"action":"retry@build","reentry_phase":"build","justification":"the change is wrong"}`)

	got := NewBridgeRetryAdjudicator(b, AgentIdentity{CLI: "claude-tmux", AgentLabel: "failure-adjudicator"}, t.TempDir()).
		Adjudicate(CycleState{WorkspacePath: t.TempDir(), CycleID: 9}, retryEnvelope{})

	assertOneFreshSession(t, b)
	if got == nil || got.Action != retryActionRetryBuild {
		t.Fatalf("adjudication = %+v, want the fresh session's proposal", got)
	}
}

func TestPhaseAdvisorLauncher_SurvivesADeadShellFirstSession(t *testing.T) {
	b := deadShellThen("advice")

	resp, err := bridgeLauncher{b: b}.Launch(context.Background(), advisor.LaunchRequest{CLI: "claude-tmux", Agent: "phase-advisor"})

	assertOneFreshSession(t, b)
	if err != nil || resp.Stdout != "advice" {
		t.Fatalf("resp = %+v, %v; want the fresh session's output", resp, err)
	}
}

func TestFailureAdvisor_RefusesAnEmptyWorkspaceBeforeAnyLaunch(t *testing.T) {
	b := deadShellThen("unused")
	in := baseFailureInput()
	in.Workspace = ""

	_, err := NewFailureAdvisor(b).Advise(context.Background(), in)

	if err == nil || !strings.Contains(err.Error(), "empty workspace") || len(b.sent) != 0 {
		t.Fatalf("Advise = %v with %d launches, want the empty-workspace refusal and no launch", err, len(b.sent))
	}
}

func TestBridgeRetryAdjudicator_ASecondDeadSessionLeavesThePolicyDefault(t *testing.T) {
	b := &scriptedBridge{scriptedLaunches{responses: []BridgeResponse{deadShell, deadShell}}}

	got := NewBridgeRetryAdjudicator(b, AgentIdentity{CLI: "claude-tmux"}, t.TempDir()).
		Adjudicate(CycleState{WorkspacePath: t.TempDir(), CycleID: 9}, retryEnvelope{})

	if got != nil || len(b.sent) != 2 {
		t.Fatalf("adjudication = %+v after %d launches, want nil (the policy default) after one fresh session", got, len(b.sent))
	}
}
