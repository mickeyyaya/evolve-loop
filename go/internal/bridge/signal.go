package bridge

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/panestream"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	CodeTokenResolverMissing  signalcenter.Code = "BRIDGE_TOKEN_RESOLVER_MISSING"
	CodeTokenResolverFailed   signalcenter.Code = "BRIDGE_TOKEN_RESOLVER_FAILED"
	CodeTokenUsageWarning     signalcenter.Code = "BRIDGE_TOKEN_USAGE_WARNING"
	CodeContextFillHigh       signalcenter.Code = "BRIDGE_CONTEXT_FILL_HIGH"
	CodeTelemetryAppendFailed signalcenter.Code = "BRIDGE_TELEMETRY_APPEND_FAILED"
	CodeTelemetryTripwire     signalcenter.Code = "BRIDGE_TELEMETRY_TRIPWIRE"

	// CodeBootStrikeClearFailed: the boot-strike store could not clear a
	// driver's consecutive boot-timeout strike after a non-80 exit.
	CodeBootStrikeClearFailed    signalcenter.Code = "BRIDGE_BOOT_STRIKE_CLEAR_FAILED"
	CodeBootStrikeRecordFailed   signalcenter.Code = "BRIDGE_BOOT_STRIKE_RECORD_FAILED"
	CodeLaunchErrorPersistFailed signalcenter.Code = "BRIDGE_LAUNCH_ERROR_PERSIST_FAILED"
	CodeResultReadFailed         signalcenter.Code = "BRIDGE_RESULT_READ_FAILED"

	// CodeFreshSessionRetry: a launch whose pane died with a session-recoverable
	// cause gets one fresh session of the same CLI.
	CodeFreshSessionRetry signalcenter.Code = "BRIDGE_FRESH_SESSION_RETRY"

	CodeCompletedOnWorktreeEvidence signalcenter.Code = "BRIDGE_COMPLETED_ON_WORKTREE_EVIDENCE"

	CodePaneStagnant  signalcenter.Code = "LIVENESS_PANE_STAGNANT"
	CodePaneHung      signalcenter.Code = "LIVENESS_PANE_HUNG"
	CodePaneExhausted signalcenter.Code = "LIVENESS_PANE_EXHAUSTED"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeTokenResolverMissing, "the engine was built without a token resolver; lifecycle and outcome records continue without token counts (fail-open)")
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeTokenResolverFailed, "the token resolver returned an error for a completed attempt; usage recorded as resolver-error")
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeTokenUsageWarning, "the token resolver measured the attempt with a caveat (invalid counters, partial measurement); the caveat is the reason")
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeContextFillHigh, "an attempt's context fill crossed the configured warn threshold; the reason names the fill and the threshold")
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeTelemetryAppendFailed, "the per-attempt telemetry record could not be appended to the workspace ledger; fields name the path")
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeTelemetryTripwire, "a successful attempt ran past the tripwire threshold with no measurable token usage — telemetry blind spot, not a phase failure")
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeBootStrikeClearFailed, "the boot-strike store could not clear the driver's consecutive boot-timeout strike after a non-80 exit (the REPL booted); the strike count may stay stale and bench the driver early; fields step=clear_boot_strike, call_id, cli, agent")
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeBootStrikeRecordFailed, "the boot-strike store could not record the driver's boot-timeout strike after exit 80; the bench never escalates for this driver; the launch error still wraps the transient sentinel; fields step=record_boot_strike, call_id, cli, agent")
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeLaunchErrorPersistFailed, "the captured launch stderr could not be persisted as <workspace>/<agent>-launch-error.txt after a non-zero exit (the forensic file a validate-gauntlet death leaves); the classified error is unchanged and the BRIDGE_EXIT_* event carries no launch_error field; fields step=persist_launch_error, path, call_id, cli, agent")
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeResultReadFailed, "the launch exited 0 but its result (the artifact, or the stdout scrollback under the stdout completion contract) could not be read into the response; Launch still returns nil with an empty Stdout — the on-disk report is the verdict source; fields step=read_result, path, completion, call_id, cli, agent")
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeFreshSessionRetry, "a fatal-pane fast-fail ended an ephemeral-session launch on a session-recoverable cause (dead_shell, cli_self_updated: the REPL process is gone, the CLI and account are fine); the dead dispatch is on the ledger marked fresh_session_retry and ONE fresh session of the same CLI runs before the caller's chain walks on — never for a named session, a delivered run, or a deadline with no room for another wait interval (F31); fields call_id (the dead dispatch), cli, agent")
	signalcenter.RegisterCode(signalcenter.ModuleBridge, CodeCompletedOnWorktreeEvidence, "a correction re-dispatch of a source-writing phase went idle without rewriting its deliverable, and the agent had changed the worktree since dispatch (host state under .evolve/, the workspace and the deliverable's own locations never count), so the bridge completed on the carried deliverable instead of letting the idle wait end in exit 81; core's phase verify and review gate then judge the unchanged deliverable; the reason is the completion note; fields deliverable, changed_paths, paths (at most eight, then +N more), cli")
	signalcenter.RegisterCode(signalcenter.ModuleLiveness, CodePaneStagnant, "a tmux pane is busy but its output stopped changing (LivenessCenter edge: busy-stagnant)")
	signalcenter.RegisterCode(signalcenter.ModuleLiveness, CodePaneHung, "a tmux pane is hung: no progress and no completion (LivenessCenter edge: hung)")
	signalcenter.RegisterCode(signalcenter.ModuleLiveness, CodePaneExhausted, "a tmux pane shows the CLI's quota/rate-limit exhaustion (LivenessCenter edge: exhausted; the exhaustion gate corroborates before rc 85)")
}

type dispatchIdentity struct {
	cycle int
	runID string
	phase string
}

func requestIdentity(req core.BridgeRequest) dispatchIdentity {
	return dispatchIdentity{cycle: req.Cycle, runID: req.RunID, phase: req.Agent}
}

func configIdentity(cfg *Config) dispatchIdentity {
	return dispatchIdentity{cycle: cfg.Cycle, runID: cfg.RunID, phase: cfg.Agent}
}

func paneLivenessHandler(signals *signalcenter.Center, id dispatchIdentity) panestream.LivenessHandler {
	return func(ev panestream.LivenessEvent) {
		state := ev.State.String()
		e := signalcenter.Event{
			Cycle: id.cycle, RunID: id.runID, Phase: id.phase, Module: signalcenter.ModuleLiveness, Origin: "paneLivenessHandler",
			Kind: signalcenter.KindPaneLiveness, Severity: signalcenter.SeverityInfo,
			Reason: "pane " + ev.SessionKey + " is " + state,
			Fields: map[string]string{"session": ev.SessionKey, "state": state},
		}
		if code := paneLivenessCode(ev.State); code != "" {
			e.Severity, e.Code = signalcenter.SeverityWarn, code
		}
		signals.Emit(e)
	}
}

func paneLivenessCode(s panestream.LivenessState) signalcenter.Code {
	switch s {
	case panestream.LivenessBusyButStagnant:
		return CodePaneStagnant
	case panestream.LivenessHung:
		return CodePaneHung
	case panestream.LivenessExhausted:
		return CodePaneExhausted
	}
	return ""
}
