package bridge

// signal.go — ADR-0101 S3: the bridge module's Signal Center codes and the
// producers' shared shape. Producers: NewEngine (a missing token resolver),
// attemptLogContext.warn (telemetry warnings), attemptLogContext.tripwire
// (a successful-but-silent attempt beyond the threshold),
// attemptLogContext.launchWarn (ADR-0103 unit 10: the Launch spine's four
// step failures registered here and the BRIDGE_EXIT_* classification the
// launchoutcome leaf registers) and paneLivenessHandler (liveness edges the
// LivenessCenter dispatches — module liveness). Each replaces a hand-written
// "[engine] WARN" line 1:1; the WARN-filtered stderr sink at the root renders
// them in the one line format.

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

	// The Launch spine's step failures (ADR-0103 unit 10): each names the host
	// step that could not do its best-effort work; the launch's classified
	// error is unchanged by any of them.
	CodeBootStrikeClearFailed    signalcenter.Code = "BRIDGE_BOOT_STRIKE_CLEAR_FAILED"
	CodeBootStrikeRecordFailed   signalcenter.Code = "BRIDGE_BOOT_STRIKE_RECORD_FAILED"
	CodeLaunchErrorPersistFailed signalcenter.Code = "BRIDGE_LAUNCH_ERROR_PERSIST_FAILED"
	CodeResultReadFailed         signalcenter.Code = "BRIDGE_RESULT_READ_FAILED"

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
	signalcenter.RegisterCode(signalcenter.ModuleLiveness, CodePaneStagnant, "a tmux pane is busy but its output stopped changing (LivenessCenter edge: busy-stagnant)")
	signalcenter.RegisterCode(signalcenter.ModuleLiveness, CodePaneHung, "a tmux pane is hung: no progress and no completion (LivenessCenter edge: hung)")
	signalcenter.RegisterCode(signalcenter.ModuleLiveness, CodePaneExhausted, "a tmux pane shows the CLI's quota/rate-limit exhaustion (LivenessCenter edge: exhausted; the exhaustion gate corroborates before rc 85)")
}

// dispatchIdentity is what every bridge signal of one dispatch carries — ONE
// derivation: cycle and run from the request (the driver's Config carries the
// same values), phase = the agent role. No fallback: every production
// dispatcher stamps Cycle; a request without one is an operator probe whose
// signals stay at cycle 0 (kept in Recent, never filed under a cycle).
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

// paneLivenessHandler adapts one LivenessCenter edge (session, new state) to
// a pane.liveness event stamped with the dispatch's cycle, run and phase —
// the Adapter from the bridge-internal liveness vocabulary to the Signal
// Center's. Idle and converging are INFO; stagnant, hung and exhausted are
// WARN with a code. A nil Center is the Null Object (test affordance).
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

// paneLivenessCode names the rule behind a WARN liveness edge; "" for the
// INFO states.
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
