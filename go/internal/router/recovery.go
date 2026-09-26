package router

import (
	"strings"
)

// Blocker mirrors core.ShipError as plain strings so the router never imports core.
type Blocker struct {
	Code  string // e.g. "AUDIT_BINDING_HEAD_MOVED"
	Class string // "transient", "precondition", "integrity" or "config"
	Stage string // the ship sub-stage, recorded as evidence
}

// recoveryHandler is one link in the recovery Chain of Responsibility.
type recoveryHandler struct {
	name  string
	match func(b Blocker) (nextPhase string, matched bool)
}

const auditBindingPrefix = "AUDIT_BINDING_"

// shipLocalCodes are ship-side preconditions a re-audit cannot re-establish; ship's repair ladder
// has already declined them, so they go to the debugger.
// See ADR-0039.
var shipLocalCodes = map[string]bool{
	"GIT_FF_MERGE_DIVERGED": true,
	"COMMIT_PREFIX_GATE":    true,
	"MANIFEST_GATE":         true,
	"GIT_DETACHED_HEAD":     true,
	"WORKTREE_RESOLVE":      true,
}

// recoveryChain routes ship failures, first match wins. Order is load-bearing: integrity precedes
// every code-keyed handler except the fleet rebase conflict, so an integrity breach always blocks.
var recoveryChain = []recoveryHandler{
	{
		// A fleet rebase conflict is overlapping work the debugger can split: the one
		// integrity-class code that recovers.
		// See ADR-0049.
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
		// Only the build can reshape a diff that touches the control plane; a re-audit
		// would verify the same diff again.
		// See ADR-0064.
		name: "control-plane-rebuild",
		match: func(b Blocker) (string, bool) {
			if b.Code == "CONTROL_PLANE_VIOLATION" {
				return "build", true
			}
			return "", false
		},
	},
	{
		// Before precondition-reaudit: these codes must never loop back to audit.
		name: "ship-local-debugger",
		match: func(b Blocker) (string, bool) {
			if shipLocalCodes[b.Code] {
				return "debugger", true
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
		// The orchestrator has rebased onto the moved main; the merged tree needs a fresh
		// audit binding, and a blind ship retry would diverge again.
		name: "fleet-rebase-reaudit",
		match: func(b Blocker) (string, bool) {
			if b.Code == "GIT_FLEET_REBASE_NEEDED" {
				return "audit", true
			}
			return "", false
		},
	},
	{
		// The orchestrator bounds the retry depth.
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

// Recover routes in.Blocker through the recovery chain; every RoutingStrategy shares it. A nil Blocker ends the cycle.
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
	// Unreachable: the last handler always matches.
	return RouterDecision{NextPhase: "debugger", Reason: "recover:unknown-debugger"}
}
