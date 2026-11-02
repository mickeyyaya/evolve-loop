package main

import (
	"context"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/acssuite"
	"github.com/mickeyyaya/evolve-loop/go/internal/auditfix"
	"github.com/mickeyyaya/evolve-loop/go/internal/auditfixports"
	"github.com/mickeyyaya/evolve-loop/go/internal/cli/agentcmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
)

var auditFixReviewProfiles = []string{"code-reviewer", "go-reviewer", "adversarial-review"}

type auditFixSite struct {
	root   string
	pol    policy.Policy
	cfg    config.RoutingConfig
	route  auditfixports.Route
	agent  agentSeat
	stderr io.Writer
}

func auditFixRung(s auditFixSite) core.Option {
	return core.WithRungHandler(retryladder.RungAuditFix, newAuditFixHandler(s))
}

func newAuditFixHandler(s auditFixSite) *auditfix.Handler {
	run := func(_ context.Context, args []string, input string) (auditfixports.SeatResult, error) {
		env, err := runAgentSeat(s.agent, args, input, s.stderr)
		return seatResultOf(env), err
	}
	wf := s.pol.WorkflowConfig()
	return auditfix.New(auditfix.Config{Stage: wf.RetryLadder.AuditFix.Stage, HoldFence: s.pol.HeldProposalsStage() == config.StageEnforce}, auditfix.Ports{
		Auditor:  auditfixports.Fixer{Run: run, Route: s.route, ProjectRoot: s.root},
		Reviewer: auditfixports.Reviewer{Run: run, Route: s.route, ProjectRoot: s.root, Profiles: auditFixReviewProfiles},
		Gates:    auditfixports.Gates{Floor: core.WholeBuildHandoffFloor(composedBuildHandoffFloor(wf, s.cfg)), Protected: protectedMembership, ProjectRoot: s.root, ACS: acssuite.Run},
		Worktree: auditfixports.Worktree{Git: gitexec.Isolated},
	})
}

func seatResultOf(env agentcmd.Envelope) auditfixports.SeatResult {
	return auditfixports.SeatResult{
		RunLabel: env.RunLabel, Family: env.Family, Verdict: env.Verdict,
		Critical: env.Findings.Critical, High: env.Findings.High, Error: env.Error,
	}
}
