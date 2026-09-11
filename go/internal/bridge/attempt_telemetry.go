package bridge

import (
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
	evolog "github.com/mickeyyaya/evolve-loop/go/internal/log"
	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
)

// LLMCallsLogFilename remains as a compatibility alias while llmcalls owns the
// filename and all ledger I/O.
const LLMCallsLogFilename = llmcalls.Filename

type attemptLogContext struct {
	callID  string
	cli     string
	agent   string
	attempt int
}

func (c attemptLogContext) warn(w io.Writer, event, detail string) {
	_, _ = fmt.Fprintf(w, "[engine] WARN: %s call_id=%s cli=%s agent=%s attempt=%d detail=%s\n",
		event, diagnosticField(c.callID), diagnosticField(c.cli), diagnosticField(c.agent), c.attempt, diagnosticField(detail))
}

// diagnosticField bounds untrusted text and emits one ASCII-quoted field. It
// keeps resolver, filesystem, and provider errors on a single parseable line,
// including strings containing newlines, terminal controls, or bidi marks.
func diagnosticField(value string) string {
	return evolog.DiagnosticField(value)
}

func (e *Engine) recordModelAttempt(
	req core.BridgeRequest,
	requestedModel string,
	code int,
	start, end time.Time,
	dispatched modelDispatch,
	launchStderr string,
	resp *core.BridgeResponse,
) {
	attempt := req.Attempt
	if attempt <= 0 {
		attempt = 1
	}
	callID := llmcalls.NewCallID(start)
	logContext := attemptLogContext{callID: callID, cli: req.CLI, agent: req.Agent, attempt: attempt}
	result, usageStatus := e.resolveAttemptTokens(req, start, end, callID, attempt)
	if usageStatus == llmcalls.UsageMeasured || usageStatus == llmcalls.UsagePartial {
		resp.Tokens = core.TokenUsage(result.Usage)
	}
	e.emitTokenWarnings(req, code, start, end, callID, attempt, result)

	if dispatched.source == "" {
		dispatched.source = llmcalls.DispatchNotStarted
	}
	duration := end.Sub(start).Milliseconds()
	var durationMS *int64
	if duration >= 0 {
		durationMS = &duration
	}
	exitCode := code
	tripwire := isTelemetryTripwire(req.CLI, code, start, end, result.Source)
	usage := result.Usage
	if usageStatus == llmcalls.UsageUnavailable || usageStatus == llmcalls.UsageResolverError {
		usage = core.TokenUsage{}
	}
	rec := llmcalls.Record{
		SchemaVersion:   llmcalls.SchemaVersion,
		CallID:          callID,
		TS:              end.UTC().Format(time.RFC3339Nano),
		StartedAt:       start.UTC().Format(time.RFC3339Nano),
		EndedAt:         end.UTC().Format(time.RFC3339Nano),
		TimingScope:     llmcalls.TimingBridgeDispatch,
		Agent:           req.Agent,
		Phase:           req.Agent,
		CLI:             req.CLI,
		Model:           requestedModel,
		RequestedModel:  requestedModel,
		DispatchedModel: dispatched.model,
		DispatchSource:  dispatched.source,
		Attempt:         attempt,
		Tokens:          usage,
		Source:          string(result.Source),
		UsageStatus:     usageStatus,
		DurationMS:      durationMS,
		ExitCode:        &exitCode,
		Tripwire:        tripwire,
		FillPct:         result.FillPct,
		CauseCode:       modelAttemptCause(code, launchStderr),
	}
	if err := llmcalls.AppendWorkspace(req.Workspace, rec); err != nil {
		logContext.warn(e.deps.Stderr, "attempt telemetry append failed",
			fmt.Sprintf("path=%s error=%v", llmcalls.Path(req.Workspace), err))
	}
}

func (e *Engine) resolveAttemptTokens(req core.BridgeRequest, start, end time.Time, callID string, attempt int) (tokenusage.Result, llmcalls.UsageStatus) {
	if e.deps.TokenResolver == nil {
		return tokenusage.Result{Source: tokenusage.SourceNone, FillPct: tokenusage.FillPctUnmeasured}, llmcalls.UsageUnavailable
	}
	var eventsLogPath, scrollback string
	if req.Workspace != "" {
		if req.Agent != "" {
			eventsLogPath = filepath.Join(req.Workspace, req.Agent+"-events.ndjson")
		}
		if data, err := os.ReadFile(filepath.Join(req.Workspace, "tmux-final-scrollback.txt")); err == nil {
			scrollback = string(data)
		}
	}
	result, err := e.deps.TokenResolver(tokenusage.Window{
		Worktree: req.Worktree, ArtifactPath: req.ArtifactPath,
		EventsLogPath: eventsLogPath, Scrollback: scrollback, Driver: req.CLI,
		Start: start, End: end,
	})
	if err != nil {
		(attemptLogContext{callID: callID, cli: req.CLI, agent: req.Agent, attempt: attempt}).warn(
			e.deps.Stderr, "token resolver failed", err.Error())
		return tokenusage.Result{Source: tokenusage.SourceNone, FillPct: tokenusage.FillPctUnmeasured}, llmcalls.UsageResolverError
	}
	status := llmcalls.UsageUnavailable
	switch result.Source {
	case tokenusage.SourceTranscript, tokenusage.SourceEventsResult:
		status = llmcalls.UsageMeasured
	case tokenusage.SourceScrollbackPeak:
		status = llmcalls.UsagePartial
	default:
		result.FillPct = tokenusage.FillPctUnmeasured
	}
	return normalizeAttemptMeasurement(result, status)
}

func normalizeAttemptMeasurement(result tokenusage.Result, status llmcalls.UsageStatus) (tokenusage.Result, llmcalls.UsageStatus) {
	if invalidTokenUsage(result.Usage) {
		result.Usage = core.TokenUsage{}
		result.FillPct = tokenusage.FillPctUnmeasured
		result.Warn = joinMeasurementWarning(result.Warn, "token resolver returned a negative counter; usage discarded as unavailable")
		status = llmcalls.UsageUnavailable
	}
	if invalidTokenUsage(result.PeakUsage) || result.PeakPromptTokens < 0 {
		result.PeakUsage = core.TokenUsage{}
		result.PeakPromptTokens = 0
		result.Warn = joinMeasurementWarning(result.Warn, "token resolver returned invalid peak context counters; contributor detail discarded")
	}
	if math.IsNaN(result.FillPct) || math.IsInf(result.FillPct, 0) || result.FillPct < tokenusage.FillPctUnmeasured {
		result.FillPct = tokenusage.FillPctUnmeasured
		result.Warn = joinMeasurementWarning(result.Warn, "token resolver returned an invalid context-fill value; fill recorded as unavailable")
	}
	return result, status
}

func invalidTokenUsage(usage core.TokenUsage) bool {
	return usage.Input < 0 || usage.Output < 0 || usage.CacheRead < 0 || usage.CacheWrite < 0
}

func joinMeasurementWarning(existing, addition string) string {
	if existing == "" {
		return addition
	}
	return existing + "; " + addition
}

func (e *Engine) emitTokenWarnings(req core.BridgeRequest, code int, start, end time.Time, callID string, attempt int, result tokenusage.Result) {
	logContext := attemptLogContext{callID: callID, cli: req.CLI, agent: req.Agent, attempt: attempt}
	if result.Warn != "" {
		logContext.warn(e.deps.Stderr, "token usage warning", result.Warn)
	}
	contributors := result.Usage
	if result.PeakPromptTokens != 0 {
		contributors = result.PeakUsage
	}
	if warning := tokenusage.FillWarnWithContributors(req.Agent, result.FillPct, defaultIfZero(e.deps.ContextFillWarnPct, defaultContextFillWarnPct), contributors); warning != "" {
		logContext.warn(e.deps.Stderr, "CONTEXT-FILL", warning)
	}
	if isTelemetryTripwire(req.CLI, code, start, end, result.Source) {
		cycle := cycleFromWorkspace(req.Workspace)
		if cycle == "" {
			cycle = "cycle-unknown"
		}
		_, _ = fmt.Fprintf(e.deps.Stderr,
			"[engine] TRIPWIRE: unmeasured successful launch call_id=%s cli=%s agent=%s attempt=%d cycle=%s duration_s=%d source=%s action=%s\n",
			diagnosticField(callID), diagnosticField(req.CLI), diagnosticField(req.Agent), attempt,
			diagnosticField(cycle), int(end.Sub(start).Seconds()), diagnosticField(string(tokenusage.SourceNone)),
			diagnosticField("build a per-CLI usage collector"))
	}
}

func isTelemetryTripwire(cli string, code int, start, end time.Time, source tokenusage.Source) bool {
	return code == ExitOK && end.Sub(start) > tripwireSuccessThreshold && source == tokenusage.SourceNone &&
		!strings.HasPrefix(strings.ToLower(cli), "claude")
}

func modelAttemptCause(code int, stderr string) string {
	if code == ExitOK {
		return ""
	}
	if code == ExitArtifactTimeout {
		if cause := artifactTimeoutCauseCode(stderr); cause != "" {
			return cause
		}
	}
	switch code {
	case ExitSafetyGate:
		return "safety_gate"
	case ExitCostLeak:
		return "cost_leak"
	case ExitBadFlags:
		return "bad_flags"
	case ExitREPLBootTimeout:
		return "repl_boot_timeout"
	case ExitArtifactTimeout:
		return "artifact_timeout"
	case ExitUnknownPrompt:
		return "unknown_prompt"
	case ExitRespondLoopGuard:
		return "respond_loop_guard"
	case ExitRequireFullUnmet:
		return "required_tier_unavailable"
	case ExitCmdTimeout:
		return "command_timeout"
	case ExitMissingBinary:
		return "missing_binary"
	default:
		return "driver_error"
	}
}

func artifactTimeoutCauseCode(stderr string) string {
	summary := artifactTimeoutSummary(stderr)
	prefix := artifactTimeoutMarker + "cause="
	if !strings.HasPrefix(summary, prefix) {
		return ""
	}
	fields := strings.Fields(strings.TrimPrefix(summary, prefix))
	if len(fields) == 0 {
		return ""
	}
	value := fields[0]
	switch artifactTimeoutCause(value) {
	case artifactTimeoutContextCancelled, artifactTimeoutDetectorError, artifactTimeoutSubmitWedged,
		artifactTimeoutTransientUpstream, artifactTimeoutReviewStop, artifactTimeoutReviewPause,
		artifactTimeoutIncomplete:
		return value
	}
	return ""
}

// recordTokenUsage preserves the package-internal test seam while routing it
// through the canonical attempt owner. Production Launch supplies its already
// frozen end time and observed dispatch metadata directly.
func (e *Engine) recordTokenUsage(req core.BridgeRequest, model string, code int, start time.Time, resp *core.BridgeResponse) {
	e.recordModelAttempt(req, model, code, start, e.deps.Now(), modelDispatch{}, "", resp)
}
