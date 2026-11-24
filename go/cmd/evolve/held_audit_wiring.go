package main

import (
	"context"
	"io"
	"sync"
	"time"

	gobridge "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/bridgechain"
	"github.com/mickeyyaya/evolve-loop/go/internal/cli/agentcmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/heldaudit"
	"github.com/mickeyyaya/evolve-loop/go/internal/heldproposal"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var heldAuditBaseDeps = agentRunDeps

const heldAuditProfile = "adversarial-review"

func heldAuditPort(root string, stderr io.Writer) ship.HeldAuditPort {
	deps := heldAuditDeps(root, stderr)
	return func(ctx context.Context, evolveDir, workspace string, rec heldproposal.Record) error {
		return heldaudit.Dispatcher{Deps: deps, EvolveDir: evolveDir}.Dispatch(ctx, workspace, rec)
	}
}

func heldAuditPlanPort(root string, stderr io.Writer) ship.HeldAuditPlanPort {
	deps := heldAuditDeps(root, stderr)
	return func(workspace string, rec heldproposal.Record) heldaudit.Plan {
		return heldaudit.PlanFor(deps, workspace, rec)
	}
}

func heldAuditDeps(root string, stderr io.Writer) heldaudit.Deps {
	load := heldAuditRouterOnce(root, stderr)
	return heldaudit.Deps{
		Seat: heldAuditSeat(root, stderr), Resolve: heldAuditResolver(root, load), Floor: heldAuditFloor(load),
		ModelFamily: gobridge.ModelFamily, Now: time.Now,
	}
}

func heldAuditRouter(root string, stderr io.Writer) (*cliroute.Router, error) {
	router, findings, err := loadCLIRouter(root, routingHost(stderr, time.Now))
	if err != nil {
		reportRoutingFindings(stderr, findings)
	}
	return router, err
}

func heldAuditRouterOnce(root string, stderr io.Writer) func() (*cliroute.Router, error) {
	var once sync.Once
	var router *cliroute.Router
	var err error
	return func() (*cliroute.Router, error) {
		once.Do(func() { router, err = heldAuditRouter(root, stderr) })
		return router, err
	}
}

func heldAuditResolver(root string, load func() (*cliroute.Router, error)) func(string) (cliroute.Decision, error) {
	return func(key string) (cliroute.Decision, error) {
		router, err := load()
		if err != nil {
			return cliroute.Decision{}, err
		}
		return router.Resolve(cliroute.Request{Agent: key, ProjectRoot: root, DefaultModel: policy.DeepTier, NeedsTools: true})
	}
}

func heldAuditFloor(load func() (*cliroute.Router, error)) func(string) string {
	return func(key string) string {
		router, err := load()
		if err != nil {
			return ""
		}
		return router.FloorFamily(key)
	}
}

func heldAuditSeat(root string, stderr io.Writer) heldaudit.SeatRunner {
	return heldAuditSeatWith(root, stderr, heldAuditBaseDeps)
}

func heldAuditSeatWith(root string, stderr io.Writer, base func() agentcmd.Deps) heldaudit.SeatRunner {
	return func(_ context.Context, req heldaudit.SeatRequest) (heldaudit.SeatOutcome, error) {
		deps := base()
		inner := deps.Bridge
		deps.Bridge = func(projectRoot string, signals *signalcenter.Center) core.Bridge {
			return bridgechain.New(inner(projectRoot, signals), nil, bridgechain.WithCandidateFilter(req.Admit))
		}
		seat := agentcmd.NewRouted(deps, agentcmd.Route{Agent: req.Route, Admit: req.Admit})
		args := []string{heldAuditProfile, "--json", "--project-root", root}
		for _, f := range req.Files {
			args = append(args, "--file", f)
		}
		env, err := runAgentSeat(seat, args, req.Input, stderr)
		return seatOutcomeOf(env, err)
	}
}

func seatOutcomeOf(env agentcmd.Envelope, err error) (heldaudit.SeatOutcome, error) {
	if err != nil && (env.Verdict != core.VerdictFAIL || env.Error != "") {
		return heldaudit.SeatOutcome{}, err
	}
	return heldaudit.SeatOutcome{
		Verdict: env.Verdict, Report: env.Report, RunLabel: env.RunLabel, LaunchRecord: env.LaunchRecord, CLI: env.CLI, Tier: env.Model,
	}, nil
}
