package agentrequest

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentclass"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/selfheal"
)

type Executor interface {
	Kind() string
	Stage() policy.RequestStage
	admit(f frame, req Request, count int) (string, string)
	run(ctx context.Context, p Pending, rec Record) (Record, Result)
}

func capCheck(cell Cell, count, limit int) (string, string) {
	if count >= limit {
		return PreCycleCap, fmt.Sprintf("cycle %d already has %d requests, and the cap is %d", cell.Cycle, count, limit)
	}
	return "", ""
}

type inboxAdd struct {
	pol policy.InboxRequestsPolicy
}

func NewInboxAdd(pol policy.InboxRequestsPolicy) Executor { return inboxAdd{pol: pol} }

func (inboxAdd) Kind() string                 { return agentclass.KindInboxAdd }
func (e inboxAdd) Stage() policy.RequestStage { return e.pol.Stage }

func (e inboxAdd) admit(f frame, req Request, count int) (string, string) {
	if pre, reason := capCheck(f.cell, count, e.pol.PerCycle); pre != "" {
		return pre, reason
	}
	return "", ""
}

func (e inboxAdd) run(_ context.Context, _ Pending, rec Record) (Record, Result) {
	if e.pol.Stage != policy.RequestStageEnforce {
		rec.Decision, rec.Reason = DecisionApproved, "the request passed the preconditions and would be filed; the intake lands with the inbox item agent-request-inbox-add-has-no-executor"
		return rec, Result{}
	}
	rec.Decision, rec.Precondition = DecisionRefused, PreIntake
	rec.Reason = "the request passed the preconditions, but the intake that files the item is not built (plan agent-request-inbox-add, component C3), so no item was filed"
	return rec, Result{}
}

type JudgeFunc func(ctx context.Context, p Pending) (string, error)

type DispatchFunc func(ctx context.Context, p Pending) (Result, error)

type dispatch struct {
	pol   policy.DispatchRequestsPolicy
	judge JudgeFunc
	start DispatchFunc
	now   func() time.Time
}

type DispatchDeps struct {
	Judge JudgeFunc
	Run   DispatchFunc
	Now   func() time.Time
}

func NewDispatch(pol policy.DispatchRequestsPolicy, deps DispatchDeps) Executor {
	return dispatch{pol: pol, judge: deps.Judge, start: deps.Run, now: deps.Now}
}

func (dispatch) Kind() string                 { return agentclass.KindDispatch }
func (e dispatch) Stage() policy.RequestStage { return e.pol.Stage }

func (e dispatch) admit(f frame, req Request, count int) (string, string) {
	if !slices.Contains(e.pol.Roles, req.Role()) {
		return PreRole, fmt.Sprintf("the role %q is not one of the allowed roles %v (policy isolation.dispatch_requests.roles)", req.Role(), e.pol.Roles)
	}
	if pre, reason := capCheck(f.cell, count, e.pol.PerCycle); pre != "" {
		return pre, reason
	}
	if f.depth > e.pol.MaxDepth {
		return PreDepthCap, fmt.Sprintf("an agent that a request dispatched wrote the request at depth %d, and the cap is %d", f.depth, e.pol.MaxDepth)
	}
	return "", ""
}

func (e dispatch) run(ctx context.Context, p Pending, rec Record) (Record, Result) {
	refused := func(pre, reason string) (Record, Result) {
		rec.Decision, rec.Precondition, rec.Reason = DecisionRefused, pre, reason
		return rec, Result{}
	}
	pending, err := writeEvidence(p, p.raw)
	if err != nil {
		return refused(PreEvidence, err.Error())
	}
	if pre, reason := e.judgeRequest(ctx, pending); pre != "" {
		return refused(pre, reason)
	}
	if e.pol.Stage != policy.RequestStageEnforce {
		rec.Decision = DecisionApproved
		return rec, Result{}
	}
	result, err := e.start(ctx, pending)
	if err != nil {
		rec.Decision, rec.Reason = DecisionFailed, err.Error()
		return rec, result
	}
	rec.Decision, rec.RunLabel, rec.Report = DecisionRun, result.RunLabel, result.Report
	return rec, result
}

func (e dispatch) judgeRequest(ctx context.Context, p Pending) (string, string) {
	raw, err := e.judge(ctx, p)
	var d phasecontract.Diagnosis
	if err == nil {
		d, err = phasecontract.ParseDiagnosis([]byte(raw), p.Name, p.EntryIDs)
	}
	if err != nil {
		return PreJudge, err.Error()
	}
	v := selfheal.Validator{Inventory: selfheal.Inventory{Requests: map[string]bool{p.Name: true}}, Stage: string(e.pol.Stage)}
	decision := v.Decide(selfheal.Incident{ID: p.Name, Tier: selfheal.TierItem, Subject: p.Request.Role()}, d.Action, e.now())
	switch {
	case decision.Verdict != selfheal.VerdictAccepted:
		return PreCatalog, decision.Precondition + ": " + decision.Reason
	case d.Action.Kind != phasecontract.ActionDispatchPhaseAgent:
		return PreJudged, cmp.Or(d.Action.Params.Reason, d.Hypothesis)
	}
	return "", ""
}
