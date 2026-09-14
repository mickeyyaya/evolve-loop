// Package launchoutcome is unit 10 of the component breakdown (ADR-0103):
// what one bridge launch exit MEANS. It owns the exit vocabulary as ONE table —
// the class name, the registered BRIDGE_EXIT_* signal code, the attempt
// ledger's cause code and the port sentinel each exit wraps — and the
// cause-line miners (the first diagnostic line, the artifact-timeout summary,
// the typed sub-cause). Classify projects the table onto ONE Outcome — the
// error chain the orchestrator parses, the BRIDGE_EXIT_* signal code and the
// ledger cause; CauseCode is the attempt ledger's own projection of the same
// row (llm-calls.ndjson). Pure: no clock, no filesystem, no Center — the host
// (bridge.Engine.Launch) owns the emit and the request gauntlet. Design:
// docs/architecture/decomposition/10-bridgeengine.md.
package launchoutcome

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// Bridge exit codes — the classifier's spelling of the numeric contract.
// bridge/exitcodes.go declares the same literals for the drivers, cmd/evolve
// and the ACS predicate that regex-scans that file (acs/cycle1580), so the
// numbers are spelled twice on purpose; TestExitCodes_HostAliasesAreTheLeafValues
// (package bridge) is the consumer pin that makes the two spellings one belief.
// Untyped so `code == ExitOK` compiles on an int.
const (
	ExitOK               = 0   // success
	ExitSafetyGate       = 2   // safety-gate (e.g. --human-input without host opt-in)
	ExitCostLeak         = 3   // cost-leak (forbidden env-var leak: ANTHROPIC_API_KEY, …)
	ExitBadFlags         = 10  // bad flags or missing required arg
	ExitREPLBootTimeout  = 80  // REPL boot timeout (*-tmux drivers)
	ExitArtifactTimeout  = 81  // artifact never appeared within the wait window
	ExitUnknownPrompt    = 85  // unknown interactive prompt (escalation report written)
	ExitRespondLoopGuard = 86  // auto-respond loop guard tripped
	ExitRequireFullUnmet = 99  // --require-full set and full tier unavailable
	ExitCmdTimeout       = 124 // driver killed by a command-level timeout (gnu `timeout` convention)
	ExitMissingBinary    = 127 // required external binary missing
)

// ExitSignalDeath is what Go's ExitError.ExitCode() reports for a signal
// death or a failed Start — most commonly exec.CommandContext SIGKILLing the
// driver on our own cancellation. Transient only under a cancelled context
// (the deliverable-authority door, cycle-859); a -1 with a live context is a
// start failure that stays plain.
const ExitSignalDeath = -1

// exitClass is ONE row of the table every projection reads: the exit's
// snake_case class name, its registered signal code, the attempt ledger's
// cause code, the port sentinel the error wraps (nil = plain errors.New) and
// whether that sentinel applies only under a cancelled context.
type exitClass struct {
	code             int
	name             string
	signal           signalcenter.Code
	causeCode        string
	sentinel         error
	sentinelOnCancel bool
}

// exitClasses is the table — 11 rows over the declared non-zero exits;
// driverErrorClass is the default for any other non-zero code; ExitOK has
// no row (success is the zero Outcome).
//
// 124 joins the transient set as the sibling of 81 (a driver killed by a
// command-level timeout is infra weather); 127 deliberately stays plain — an
// absent CLI is an environment defect that must fail loud, and its only
// recovery is the exit-code-triggered family fallback, which sees the raw 127.
var exitClasses = []exitClass{
	{code: ExitSafetyGate, name: "safety_gate", signal: CodeExitSafetyGate, causeCode: "safety_gate"},
	{code: ExitCostLeak, name: "cost_leak", signal: CodeExitCostLeak, causeCode: "cost_leak"},
	{code: ExitBadFlags, name: "bad_flags", signal: CodeExitBadFlags, causeCode: "bad_flags"},
	{code: ExitREPLBootTimeout, name: "repl_boot_timeout", signal: CodeExitREPLBootTimeout, causeCode: "repl_boot_timeout", sentinel: core.ErrTransientBridgeFailure},
	{code: ExitArtifactTimeout, name: "artifact_timeout", signal: CodeExitArtifactTimeout, causeCode: "artifact_timeout", sentinel: core.ErrArtifactTimeout},
	{code: ExitUnknownPrompt, name: "unknown_prompt", signal: CodeExitUnknownPrompt, causeCode: "unknown_prompt", sentinel: core.ErrTransientBridgeFailure},
	{code: ExitRespondLoopGuard, name: "respond_loop_guard", signal: CodeExitRespondLoopGuard, causeCode: "respond_loop_guard", sentinel: core.ErrTransientBridgeFailure},
	{code: ExitRequireFullUnmet, name: "required_tier_unavailable", signal: CodeExitRequiredTierUnavailable, causeCode: "required_tier_unavailable"},
	{code: ExitCmdTimeout, name: "command_timeout", signal: CodeExitCommandTimeout, causeCode: "command_timeout", sentinel: core.ErrTransientBridgeFailure},
	{code: ExitMissingBinary, name: "missing_binary", signal: CodeExitMissingBinary, causeCode: "missing_binary"},
	{code: ExitSignalDeath, name: "signal_death", signal: CodeExitSignalDeath, causeCode: "driver_error", sentinel: core.ErrTransientBridgeFailure, sentinelOnCancel: true},
}

// driverErrorClass is the row for any non-zero exit the table does not name
// (the attempt ledger's `driver_error` default); it is never matched by code.
var driverErrorClass = exitClass{name: "driver_error", signal: CodeExitDriverError, causeCode: "driver_error"}

// classOf returns the row for code, or driverErrorClass for an unknown
// non-zero code. ExitOK has no row: callers gate on it first.
func classOf(code int) exitClass {
	for _, row := range exitClasses {
		if row.code == code {
			return row
		}
	}
	return driverErrorClass
}
