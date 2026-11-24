package main

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/gateclass"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/selfheal"
)

const (
	builderSeat = "builder"
	auditorSeat = "auditor"
)

func productionAuditRefused(root string) (bool, error) {
	return auditRefusedWith(routingHost(io.Discard, time.Now), root)
}

func auditRefusedWith(host cliroute.Host, root string) (bool, error) {
	router, _, err := loadCLIRouter(root, host)
	if err != nil {
		return false, err
	}
	catalog := routingCatalog(root)
	return repairAuditRefusal(func(agent string) (cliroute.Decision, error) {
		return router.Resolve(cliroute.Request{Agent: agent, Phase: agentPhase(agent, catalog), ProjectRoot: root, DefaultModel: unsetDispatchTier})
	})
}

type seatResolver func(agent string) (cliroute.Decision, error)

func repairAuditRefusal(resolve seatResolver) (bool, error) {
	builder, err := resolve(builderSeat)
	if err != nil {
		return false, fmt.Errorf("resolve the %s seat: %w", builderSeat, err)
	}
	auditor, err := resolve(auditorSeat)
	if err != nil {
		return false, fmt.Errorf("resolve the %s seat: %w", auditorSeat, err)
	}
	if len(builder.Plan.Candidates) == 0 {
		return false, fmt.Errorf("the %s seat resolved no candidate", builderSeat)
	}
	_, err = auditor.IndependentOf(llmroute.Family(builder.Plan.Candidates[0]))
	return errors.Is(err, cliroute.ErrNoIndependentFamily), nil
}

type decidedProposal struct {
	inc selfheal.Incident
	p   selfheal.Proposal
}

type repairCandidate struct {
	inc    selfheal.Incident
	action phasecontract.DiagnosisAction
}

func acceptedRepairs(decided []decidedProposal) []repairCandidate {
	var out []repairCandidate
	for _, d := range decided {
		if action, err := d.p.Dispatchable(); err == nil && action.Kind == phasecontract.ActionOpenRepairItem {
			out = append(out, repairCandidate{inc: d.inc, action: action})
		}
	}
	return out
}

func (d *proposalDecider) openRepairItems(env selfhealEnv, pass proposePass, decided []decidedProposal) {
	cfg := pass.pol.HealingRepairCycle()
	candidates := acceptedRepairs(decided)
	if !d.on || cfg.Stage == policy.HealingStageOff || len(candidates) == 0 {
		return
	}
	center, closeCenter := env.newCenter(pass.root, paths.EvolveDirOf(pass.root), pass.stderr)
	defer closeCenter()
	opts := inboxmover.Options{ProjectRoot: pass.root, Stderr: pass.stderr, Signals: center, IsProtectedPath: guards.IsProtectedSurface,
		PriorityClasses: pass.pol.InboxPriorityConfig().ClassOrder, Origin: inboxbatch.OriginCycle}
	file := func(raw []byte, tier gateclass.Tier, protected []string) error {
		_, err := inboxmover.FileRepair(opts, raw, inboxmover.RepairStamp{Tier: string(tier), ProtectedFiles: protected})
		return err
	}
	plan := selfheal.RepairPlan{Stage: string(cfg.Stage), IsProtected: guards.IsProtectedSurface}
	var err error
	if plan.AuditRefused, err = env.auditRefused(pass.root); err != nil {
		d.warnings = append(d.warnings, "the audit seat of a repair cycle cannot be read, so would_refuse_audit is false: "+err.Error())
	}
	for _, c := range candidates {
		routed, err := selfheal.OpenRepairItem(c.inc, c.action, plan, file)
		if err != nil {
			d.warnings = append(d.warnings, err.Error())
			continue
		}
		center.Emit(routed.Event())
	}
}
