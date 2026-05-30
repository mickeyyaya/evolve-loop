package router

import (
	"strings"
)

// Blocker is the string-only ship-error envelope the orchestrator passes to
// the router for recovery routing. It mirrors the structured ship error
// defined in the core package (core.ShipError) as plain strings so that the
// router stays a leaf package and never imports core (which would cycle).
type Blocker struct {
	// Code is the ship-error code (e.g. "AUDIT_BINDING_HEAD_MOVED").
	Code string
	// Class is the ship-error class: "transient", "precondition",
	// "integrity", or "config".
	Class string
	// Stage is the ship sub-stage where the error surfaced (for evidence).
	Stage string
}

// recoveryHandler is one link in the recovery Chain of Responsibility. It
// returns the next phase and whether it claimed the blocker.
type recoveryHandler struct {
	name  string
	match func(b Blocker) (nextPhase string, matched bool)
}

// auditBindingPrefix is the code prefix shared by every audit-binding failure.
const auditBindingPrefix = "AUDIT_BINDING_"

// recoveryChain is the ordered Chain of Responsibility for ship-failure
// recovery. Order is load-bearing: integrity is checked before precondition so
// an integrity breach that also looks like a binding precondition still blocks.
var recoveryChain = []recoveryHandler{
	{
		// Integrity breaches never auto-recover — block loudly.
		name: "integrity-block",
		match: func(b Blocker) (string, bool) {
			if b.Class == "integrity" {
				return PhaseEnd, true
			}
			return "", false
		},
	},
	{
		// Stale binding / gate precondition → re-run audit (saga alt path).
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
		// Transient I/O / push races → retry ship (orchestrator bounds depth).
		name: "transient-retry-ship",
		match: func(b Blocker) (string, bool) {
			if b.Class == "transient" {
				return "ship", true
			}
			return "", false
		},
	},
	{
		// Terminal catch-all: any unknown/novel code → LLM debugger phase.
		name: "unknown-debugger",
		match: func(b Blocker) (string, bool) {
			return "debugger", true
		},
	},
}

// Recover is the pure recovery router: given a RouteInput carrying a Blocker, it
// walks the recovery Chain of Responsibility and returns the next-phase
// decision. It is deterministic and shared by every RoutingStrategy (recovery
// needs no LLM). When in.Blocker is nil it defensively returns PhaseEnd.
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
	// Unreachable: the terminal handler always matches.
	return RouterDecision{NextPhase: "debugger", Reason: "recover:unknown-debugger"}
}
