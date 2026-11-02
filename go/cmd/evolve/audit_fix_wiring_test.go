package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/auditfix"
	"github.com/mickeyyaya/evolve-loop/go/internal/cli/agentcmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
)

const wiredCited = "pkg/helper_test.go"

func auditFixInput(t *testing.T, worktree string) core.RungInput {
	t.Helper()
	return core.RungInput{
		Rung: retryladder.RungAuditFix, Cycle: 2035, Workspace: t.TempDir(), Worktree: worktree,
		Choice:  retryladder.AuditFixChoice{Choose: true, Input: retryladder.AuditFixInput{Bounds: retryladder.AuditFixBounds{MaxFiles: 3, MaxLines: 40, MaxCodeLines: 10, MaxTokens: 6000}}},
		Failure: phasecontract.FailureBlock{Defects: []string{"H1: " + wiredCited + ":1 missing"}, Prescription: []string{"Add the helper to " + wiredCited}},
	}
}

func wiredSite(t *testing.T, stage string, agent agentSeat) auditFixSite {
	t.Helper()
	pol := policy.Policy{Workflow: &policy.WorkflowPolicy{RetryLadder: &policy.RetryLadderPolicy{AuditFix: &policy.AuditFixPolicy{Stage: stage}}}}
	route := func(profile string) (auditfix.Seat, error) {
		family := map[string]string{"console-builder": "claude", "code-reviewer": "claude", "go-reviewer": "agy"}[profile]
		return auditfix.Seat{Family: family, Tier: "deep"}, nil
	}
	return auditFixSite{root: t.TempDir(), pol: pol, route: route, agent: agent, stderr: io.Discard}
}

func TestNewAuditFixHandler_ReadsTheAuditFixStageOfThePolicy(t *testing.T) {
	t.Parallel()
	in := auditFixInput(t, "/wt/2035")
	got, err := newAuditFixHandler(wiredSite(t, "", nil)).Handle(context.Background(), in)
	if err != nil || got.Reason != auditfix.ReasonShadow {
		t.Fatalf("Handle = %+v, %v, want the policy default shadow", got, err)
	}
	if auditFixRung(wiredSite(t, "", nil)) == nil {
		t.Fatal("auditFixRung returned no option")
	}
}

func TestNewAuditFixHandler_DispatchesTheFixerAndTheReviewerThroughEvolveAgentRun(t *testing.T) {
	t.Parallel()
	repo := gittest.Fixture(t)
	repo.Write(wiredCited, "package pkg\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	var profiles []string
	agent := func(args []string, _ io.Reader, stdout, _ io.Writer) int {
		profiles = append(profiles, args[1])
		verdict := core.VerdictPASS
		if args[1] == "console-builder" {
			wt := args[slices.Index(args, "--worktree")+1]
			if err := os.WriteFile(filepath.Join(wt, wiredCited), []byte("package pkg\n\nvar helper = 1\n"), 0o644); err != nil {
				t.Error(err)
			}
		} else {
			verdict = core.VerdictFAIL
		}
		raw, _ := json.Marshal(agentcmd.Envelope{Profile: args[1], Verdict: verdict})
		_, _ = stdout.Write(raw)
		if verdict == core.VerdictFAIL {
			return 1
		}
		return 0
	}

	got, err := newAuditFixHandler(wiredSite(t, policy.RetryLadderEnforce, agent)).Handle(context.Background(), auditFixInput(t, repo.Dir))

	if err != nil || !got.Escalate {
		t.Fatalf("Handle = %+v, %v, want an escalation after the failed review", got, err)
	}
	if !slices.Equal(profiles, []string{"console-builder", "go-reviewer"}) {
		t.Fatalf("agent runs = %v, want the fixer and then a reviewer of another family", profiles)
	}
}

func TestWireOrchestratorDeps_RegistersTheAuditFixHandler(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	deps := wireOrchestratorDeps(root, evolveDir, io.Discard, routingRun{})
	if deps.RoutingErr != nil {
		t.Fatalf("wireOrchestratorDeps: %v", deps.RoutingErr)
	}
	if got := deps.Orchestrator.RegisteredRungs(); !slices.Equal(got, []retryladder.Rung{retryladder.RungAuditFix}) {
		t.Fatalf("RegisteredRungs = %v, want the audit_fix handler of the cycle", got)
	}
}
