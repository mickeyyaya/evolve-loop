package bridge

import (
	"cmp"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/launchoutcome"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
	evolog "github.com/mickeyyaya/evolve-loop/go/internal/log"
	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
)

const LLMCallsLogFilename = llmcalls.Filename

type attemptLogContext struct {
	signals *signalcenter.Center
	dispatchIdentity
	callID  string
	cli     string
	agent   string
	attempt int
}

func (e *Engine) attemptContext(req core.BridgeRequest, callID string, attempt int) attemptLogContext {
	return attemptLogContext{signals: e.deps.Signals, dispatchIdentity: requestIdentity(req),
		callID: callID, cli: req.CLI, agent: req.Agent, attempt: attempt}
}

func (c attemptLogContext) event(origin string, kind signalcenter.Kind, code signalcenter.Code, reason string) signalcenter.Event {
	return signalcenter.Event{
		Cycle: c.cycle, RunID: c.runID, Phase: c.phase, Attempt: c.attempt,
		Module: signalcenter.ModuleBridge, Origin: origin, Kind: kind,
		Severity: signalcenter.SeverityWarn, Code: code, Reason: reason,
		Fields: map[string]string{"call_id": c.callID, "cli": c.cli, "agent": c.agent},
	}
}

func (c attemptLogContext) warn(code signalcenter.Code, detail string) {
	c.launchWarn("attemptLogContext.warn", "", code, detail, nil)
}

func (c attemptLogContext) tripwire(durationMS int64) {
	e := c.event("attemptLogContext.tripwire", signalcenter.KindBridgeTripwire, CodeTelemetryTripwire,
		"unmeasured successful launch: past the tripwire threshold with source=none — build a per-CLI usage collector")
	e.Fields["duration_ms"] = strconv.FormatInt(durationMS, 10)
	c.signals.Emit(e)
}

func (c attemptLogContext) launchWarn(origin, step string, code signalcenter.Code, reason string, fields map[string]string) {
	e := c.event(origin, signalcenter.KindBridgeWarning, code, reason)
	if step != "" {
		e.Fields["step"] = step
	}
	for k, v := range fields {
		e.Fields[k] = v
	}
	c.signals.Emit(e)
}

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
	marks ...func(*llmcalls.Record),
) attemptLogContext {
	attempt := req.Attempt
	if attempt <= 0 {
		attempt = 1
	}
	callID := llmcalls.NewCallID(start)
	logContext := e.attemptContext(req, callID, attempt)
	result, usageStatus := e.resolveAttemptTokens(req, start, end, callID, attempt)
	if usageStatus == llmcalls.UsageMeasured || usageStatus == llmcalls.UsagePartial {
		resp.Tokens = core.TokenUsage(result.Usage)
	}
	e.emitTokenWarnings(req, code, start, end, callID, attempt, result)

	dispatched.source = cmp.Or(dispatched.source, llmcalls.DispatchNotStarted)
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
		CauseCode:       launchoutcome.CauseCode(code, launchStderr),
		ServedModel:     result.ServedModel,
	}
	for _, mark := range marks {
		mark(&rec)
	}
	if err := llmcalls.AppendWorkspace(req.Workspace, rec); err != nil {
		logContext.warn(CodeTelemetryAppendFailed,
			fmt.Sprintf("attempt telemetry append failed: path=%s error=%v", llmcalls.Path(req.Workspace), err))
	}
	return logContext
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
	result, err := e.deps.TokenResolver(manifestTokenWindow(tokenusage.Window{
		Worktree: req.Worktree, ArtifactPath: req.ArtifactPath,
		EventsLogPath: eventsLogPath, Scrollback: scrollback, Driver: req.CLI,
		Start: start, End: end,
	}))
	if err != nil {
		(e.attemptContext(req, callID, attempt)).warn(CodeTokenResolverFailed, "token resolver failed: "+err.Error())
		return tokenusage.Result{Source: tokenusage.SourceNone, FillPct: tokenusage.FillPctUnmeasured}, llmcalls.UsageResolverError
	}
	status := llmcalls.UsageUnavailable
	switch result.Source {
	case tokenusage.SourceTranscript, tokenusage.SourceEventsResult, tokenusage.SourceAgyConversation:
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
	logContext := e.attemptContext(req, callID, attempt)
	if result.Warn != "" {
		logContext.warn(CodeTokenUsageWarning, result.Warn)
	}
	contributors := result.Usage
	if result.PeakPromptTokens != 0 {
		contributors = result.PeakUsage
	}
	if warning := tokenusage.FillWarnWithContributors(req.Agent, result.FillPct, defaultIfZero(e.deps.ContextFillWarnPct, defaultContextFillWarnPct), contributors); warning != "" {
		logContext.warn(CodeContextFillHigh, warning)
	}
	if isTelemetryTripwire(req.CLI, code, start, end, result.Source) {
		logContext.tripwire(end.Sub(start).Milliseconds())
	}
}

func isTelemetryTripwire(cli string, code int, start, end time.Time, source tokenusage.Source) bool {
	return code == ExitOK && end.Sub(start) > tripwireSuccessThreshold && source == tokenusage.SourceNone && tokenusage.HasCollector(manifestTokenWindow(tokenusage.Window{Driver: cli}).Collector)
}

func (e *Engine) recordTokenUsage(req core.BridgeRequest, model string, code int, start time.Time, resp *core.BridgeResponse) {
	e.recordModelAttempt(req, model, code, start, e.deps.Now(), modelDispatch{}, "", resp)
}

func markTry(run launchRun, try *retryladder.Try) func(*llmcalls.Record) {
	return func(rec *llmcalls.Record) {
		try.Mark(rec)
		rec.Effort = run.effort
		rec.ServedModel = cmp.Or(rec.ServedModel, run.servedModel)
	}
}

func observeLaunchEffort(deps Deps, effort string) {
	if deps.onLaunchEffort != nil {
		deps.onLaunchEffort(effort)
	}
}

func observeServedModel(deps Deps, model string) {
	if deps.onServedModel != nil {
		deps.onServedModel(model)
	}
}

func manifestTokenWindow(w tokenusage.Window) tokenusage.Window {
	m, err := LoadManifest(w.Driver)
	if err != nil {
		return w
	}
	w.Collector, w.ContextWindow, w.TokenLineRegex = m.Collectors[CollectorTokens], m.ContextWindow, m.TokenLineRegex
	return w
}
