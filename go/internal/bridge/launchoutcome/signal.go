package launchoutcome

import "github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"

// The BRIDGE_EXIT_* projection the Signal Center design promised: one WARN
// code per non-zero launch exit, spelled by class name (fields.exit_code
// carries the number) — the table's signal column, read as Outcome.Signal.
// The naming rule code == "BRIDGE_EXIT_" + upper(name) is a test, not a
// computation. The host emits exactly one per failed Launch, after the
// launch-error persist and the boot-strike record, with the classified error
// as the reason.
const (
	CodeExitSafetyGate              signalcenter.Code = "BRIDGE_EXIT_SAFETY_GATE"
	CodeExitCostLeak                signalcenter.Code = "BRIDGE_EXIT_COST_LEAK"
	CodeExitBadFlags                signalcenter.Code = "BRIDGE_EXIT_BAD_FLAGS"
	CodeExitREPLBootTimeout         signalcenter.Code = "BRIDGE_EXIT_REPL_BOOT_TIMEOUT"
	CodeExitArtifactTimeout         signalcenter.Code = "BRIDGE_EXIT_ARTIFACT_TIMEOUT"
	CodeExitUnknownPrompt           signalcenter.Code = "BRIDGE_EXIT_UNKNOWN_PROMPT"
	CodeExitRespondLoopGuard        signalcenter.Code = "BRIDGE_EXIT_RESPOND_LOOP_GUARD"
	CodeExitRequiredTierUnavailable signalcenter.Code = "BRIDGE_EXIT_REQUIRED_TIER_UNAVAILABLE"
	CodeExitCommandTimeout          signalcenter.Code = "BRIDGE_EXIT_COMMAND_TIMEOUT"
	CodeExitMissingBinary           signalcenter.Code = "BRIDGE_EXIT_MISSING_BINARY"
	CodeExitSignalDeath             signalcenter.Code = "BRIDGE_EXIT_SIGNAL_DEATH"
	CodeExitDriverError             signalcenter.Code = "BRIDGE_EXIT_DRIVER_ERROR"
)

func init() {
	const common = "; the reason is the classified launch error (the exact string the outcome record and the retry backoff parse), fields exit_code, cause_code, transient, ctx_cancelled, launch_error (the persisted <agent>-launch-error.txt when written), call_id, cli, agent, step=classify"
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeExitSafetyGate, "a launch died in a safety gate (exit 2: e.g. --human-input without the host opt-in); plain failure"+common)
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeExitCostLeak, "a launch refused to leak a forbidden credential into the inner CLI's environment (exit 3); plain failure"+common)
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeExitBadFlags, "a launch died in the validate gauntlet — bad flags, a missing profile, an unreadable or empty prompt, no driver for the CLI (exit 10); plain failure"+common)
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeExitREPLBootTimeout, "a tmux REPL never showed its prompt marker within the boot budget (exit 80); transient — the boot strike is recorded before this event"+common)
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeExitArtifactTimeout, "the artifact never appeared within the wait window (exit 81); wraps core.ErrArtifactTimeout — one code whatever the sub-cause, which rides cause_code (context_cancelled, completion_detector_error, submit_wedged, transient_upstream, review_stop, review_pause, incomplete)"+common)
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeExitUnknownPrompt, "the auto-responder met an interactive prompt it could not answer and wrote the escalation report (exit 85); transient"+common)
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeExitRespondLoopGuard, "the auto-respond loop guard tripped (exit 86); transient"+common)
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeExitRequiredTierUnavailable, "--require-full was set and the full model tier is unavailable (exit 99); plain failure"+common)
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeExitCommandTimeout, "the driver was killed by a command-level timeout (exit 124, the gnu timeout convention); transient — infra weather, the sibling of 81"+common)
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeExitMissingBinary, "a required external binary is missing (exit 127); deliberately plain — an absent CLI is an environment defect, and the family fallback sees the raw 127"+common)
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeExitSignalDeath, "the driver died of a signal or failed to start (exit -1): transient under a cancelled context (our own teardown — the deliverable-authority door), plain with a live one (a start failure); the ledger cause stays driver_error"+common)
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeExitDriverError, "a launch exited with a code the table does not name; plain failure, ledger cause driver_error"+common)
}
