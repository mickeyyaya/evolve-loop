package agentrequest

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/agentclass"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
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
	kind  string
	caps  func(policy.Policy) any
	build func(policy.Policy, DispatchDeps) Executor
}

var kindTable = []kindEntry{
	{
		kind: agentclass.KindDispatch,
		caps: func(p policy.Policy) any {
			return dispatchCap{p.IsolationDispatchRequests(), agentclass.RequiredArgs(agentclass.KindDispatch)}
		},
		build: func(p policy.Policy, deps DispatchDeps) Executor {
			return NewDispatch(p.IsolationDispatchRequests(), deps)
		},
	},
	{
		kind: agentclass.KindInboxAdd,
		caps: func(p policy.Policy) any {
			return inboxAddCap{p.IsolationInboxRequests(), agentclass.RequiredArgs(agentclass.KindInboxAdd)}
		},
		build: func(p policy.Policy, _ DispatchDeps) Executor {
			return NewInboxAdd(p.IsolationInboxRequests())
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
