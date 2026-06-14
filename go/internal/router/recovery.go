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

// shipLocalCodes are ship-LOCAL preconditions a re-audit cannot re-establish:
// re-running audit re-verifies the same code while the blocking condition
// (merge divergence, prefix-gate scope, detached HEAD, unresolvable worktree)
// lives entirely on the ship side — the cycle-230 audit↔ship loop (3 PASS
// audits, 0 ships). Ship's in-Run repair ladder (ADR-0039 §8) already
// attempted the typed repair before this error surfaced, so the residue goes
// to the LLM debugger phase for triage, never back to audit.
var shipLocalCodes = map[string]bool{
	"GIT_FF_MERGE_DIVERGED": true,
	"COMMIT_PREFIX_GATE":    true,
	"GIT_DETACHED_HEAD":     true,
	"WORKTREE_RESOLVE":      true,
}

// recoveryChain is the ordered Chain of Responsibility for ship-failure
// recovery. Order is load-bearing: integrity is checked before precondition so
// an integrity breach that also looks like a binding precondition still blocks.
var recoveryChain = []recoveryHandler{
	{
		// ADR-0049 G13a: a fleet rebase CONFLICT is genuinely overlapping work the
		// advisor's disjoint-file partition should have separated. It carries the
		// integrity class, but unlike a tamper/drift breach it is RECOVERABLE by
		// triage — route to the LLM debugger (recommend sequential retry / partition
		// split), NOT a blind block. Ordered FIRST so this specific code wins over
		// the generic integrity-block below (the one integrity code that recovers).
		name: "fleet-rebase-conflict-debugger",
		match: func(b Blocker) (string, bool) {
			if b.Code == "GIT_FLEET_REBASE_CONFLICT" {
				return "debugger", true
			}
			return "", false
		},
	},
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
		// Ship-local precondition (repair ladder already declined) → debugger.
		// Ordered AFTER integrity-block (integrity always wins) and BEFORE
		// precondition-reaudit (these codes must never loop back to audit).
		name: "ship-local-debugger",
		match: func(b Blocker) (string, bool) {
			if shipLocalCodes[b.Code] {
				return "debugger", true
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
		// ADR-0049 S5b: a fleet-mode ff-merge divergence (a peer cycle moved main
		// mid-pipeline) is recovered by rebasing the cycle branch onto the new
		// main and re-running AUDIT on the merged tree — the test-the-merged-tree
		// / merge-queue pattern, which produces a FRESH audit binding for the
		// rebased tree (re-pinning in place would be self-referential). NOT a
		// retry-ship (it would just diverge again) and NOT the debugger. The
		// rebase action itself runs in the orchestrator's recoverFromShipError
		// before this re-audit. Ordered BEFORE transient-retry-ship because this
		// transient code needs re-audit, not a blind ship retry.
		name: "fleet-rebase-reaudit",
		match: func(b Blocker) (string, bool) {
			if b.Code == "GIT_FLEET_REBASE_NEEDED" {
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
