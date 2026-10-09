package router

import (
	"strings"
)

type Blocker struct {
	Code  string
	Class string
	Stage string
}

type recoveryHandler struct {
	name  string
	match func(b Blocker) (nextPhase string, matched bool)
}

const auditBindingPrefix = "AUDIT_BINDING_"

var shipLocalCodes = map[string]bool{
	"GIT_FF_MERGE_DIVERGED": true,
	"COMMIT_PREFIX_GATE":    true,
	"MANIFEST_GATE":         true,
	"GIT_DETACHED_HEAD":     true,
	"WORKTREE_RESOLVE":      true,
}

var recoveryChain = []recoveryHandler{
	{
		name: "fleet-rebase-conflict-debugger",
		match: func(b Blocker) (string, bool) {
			if b.Code == "GIT_FLEET_REBASE_CONFLICT" {
				return "debugger", true
			}
			return "", false
		},
	},
	{
		name: "integrity-block",
		match: func(b Blocker) (string, bool) {
			if b.Class == "integrity" {
				return PhaseEnd, true
			}
			return "", false
		},
	},
	{
		name: "control-plane-rebuild",
		match: func(b Blocker) (string, bool) {
			if b.Code == "CONTROL_PLANE_VIOLATION" {
				return "build", true
			}
			return "", false
		},
	},
	{
		name: "ship-local-debugger",
		match: func(b Blocker) (string, bool) {
			if shipLocalCodes[b.Code] {
				return "debugger", true
			}
			return "", false
		},
	},
	{
		name: "push-policy-end",
		match: func(b Blocker) (string, bool) {
			if b.Code == "GIT_PUSH_POLICY_REFUSED" {
				return PhaseEnd, true
			}
			return "", false
		},
	},
	{
		name: "precondition-reaudit",
		match: func(b Blocker) (string, bool) {
			if b.Class == "precondition" ||
				strings.HasPrefix(b.Code, auditBindingPrefix) ||
				b.Code == "EGPS_RED_COUNT" {
				return "audit", true
			}
			return "", false
		},
	},
	{
		name: "fleet-rebase-reaudit",
		match: func(b Blocker) (string, bool) {
			if b.Code == "GIT_FLEET_REBASE_NEEDED" {
				return "audit", true
			}
			return "", false
		},
	},
	{
		name: "transient-retry-ship",
		match: func(b Blocker) (string, bool) {
			if b.Class == "transient" {
				return "ship", true
			}
			return "", false
		},
	},
	{
		name: "unknown-debugger",
		match: func(b Blocker) (string, bool) {
			return "debugger", true
		},
	},
}

func Recover(in RouteInput) RouterDecision {
	if in.Blocker == nil {
		return RouterDecision{NextPhase: PhaseEnd, Reason: "recover:no-blocker"}
	}
	b := *in.Blocker
	for _, h := range recoveryChain {
		if next, matched := h.match(b); matched {
			return RouterDecision{
				NextPhase: next,
				Reason:    "recover:" + h.name,
				Evidence: map[string]interface{}{
					"code":  b.Code,
					"class": b.Class,
					"stage": b.Stage,
				},
			}
		}
	}
	return RouterDecision{NextPhase: "debugger", Reason: "recover:unknown-debugger"}
}
