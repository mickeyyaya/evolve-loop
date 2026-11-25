package agentrequest

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/agentclass"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type dispatchCap struct {
	policy.DispatchRequestsPolicy
	RequiredArgs []string `json:"required_args"`
}

type inboxAddCap struct {
	policy.InboxRequestsPolicy
	RequiredArgs []string `json:"required_args"`
}

type kindEntry struct {
	kind         string
	lossSeverity signalcenter.Severity
	caps         func(policy.Policy) any
	build        func(policy.Policy, DispatchDeps) Executor
}

var kindTable = []kindEntry{
	{
		kind:         agentclass.KindDispatch,
		lossSeverity: signalcenter.SeverityWarn,
		caps: func(p policy.Policy) any {
			return dispatchCap{p.IsolationDispatchRequests(), agentclass.RequiredArgs(agentclass.KindDispatch)}
		},
		build: func(p policy.Policy, deps DispatchDeps) Executor {
			return NewDispatch(p.IsolationDispatchRequests(), deps)
		},
	},
	{
		kind:         agentclass.KindInboxAdd,
		lossSeverity: signalcenter.SeverityIncident,
		caps: func(p policy.Policy) any {
			return inboxAddCap{p.IsolationInboxRequests(), agentclass.RequiredArgs(agentclass.KindInboxAdd)}
		},
		build: func(p policy.Policy, deps DispatchDeps) Executor {
			return NewInboxAdd(p.IsolationInboxRequests(), deps.File)
		},
	},
}

func Executors(pol policy.Policy, deps DispatchDeps) []Executor {
	out := make([]Executor, len(kindTable))
	for i, k := range kindTable {
		out[i] = k.build(pol, deps)
	}
	return out
}

func Caps(pol policy.Policy) map[string]any {
	out := make(map[string]any, len(kindTable))
	for _, k := range kindTable {
		out[k.kind] = k.caps(pol)
	}
	return out
}

func lossSeverity(file string) signalcenter.Severity {
	kind, _, _ := agentclass.RequestFile(file)
	for _, k := range kindTable {
		if k.kind == kind {
			return k.lossSeverity
		}
	}
	return signalcenter.SeverityWarn
}
