// Package outcome records every terminal phase disposition exactly once, at the recording chokepoint.
// See docs/architecture/packages/internal-core-outcome.md.
package outcome

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/contextfill"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const (
	CodeSidecarSkipped     signalcenter.Code = "OUTCOME_SIDECAR_SKIPPED"
	CodeSidecarWriteFailed signalcenter.Code = "OUTCOME_SIDECAR_WRITE_FAILED"
	CodeTimingSkipped      signalcenter.Code = "OUTCOME_TIMING_SKIPPED"
	CodeTimingWriteFailed  signalcenter.Code = "OUTCOME_TIMING_WRITE_FAILED"
)

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleOutcome, CodeSidecarSkipped, "the phase's <phase>-usage.json sidecar was not written because the workspace is empty (the CWD-relative leak guard); the in-memory record and the phase.outcome event still stand")
	signalcenter.RegisterCode(signalcenter.ModuleOutcome, CodeSidecarWriteFailed, "the phase's <phase>-usage.json sidecar could not be encoded or written; the reason names the step and the error, fields name the phase and the path; the in-memory record stands")
	signalcenter.RegisterCode(signalcenter.ModuleOutcome, CodeTimingSkipped, "phase-timing.json was not written because the workspace is empty; the composed timing set is returned to the caller unchanged")
	signalcenter.RegisterCode(signalcenter.ModuleOutcome, CodeTimingWriteFailed, "phase-timing.json could not be encoded or written (marshal, temp write or rename); the reason names the step and the error; the composed set is still returned")
}

type UsageSidecar struct {
	Phase        string                `json:"phase"`
	CostUSD      float64               `json:"cost_usd"`
	DurationMS   int64                 `json:"duration_ms"`
	AttemptCount int                   `json:"attempt_count"`
	Verdict      string                `json:"verdict"`
	StartedAt    string                `json:"started_at,omitempty"`
	EndedAt      string                `json:"ended_at,omitempty"`
	Archetype    string                `json:"archetype,omitempty"`
	AbortReason  string                `json:"abort_reason,omitempty"`
	Tokens       cyclestate.TokenUsage `json:"tokens,omitempty"`
}

func UsageSidecarPath(workspace, phase string) string {
	return filepath.Join(workspace, phase+"-usage.json")
}

type Recorder struct {
	now       func() time.Time
	archetype func(phase string) string
	emit      func(cycle int, out recovery.PhaseOutcome)
	signals   func() *signalcenter.Center
}

type Option func(*Recorder)

func WithSignals(c func() *signalcenter.Center) Option {
	return func(r *Recorder) {
		if c != nil {
			r.signals = c
		}
	}
}

func NewRecorder(now func() time.Time, archetype func(phase string) string, emit func(cycle int, out recovery.PhaseOutcome), opts ...Option) *Recorder {
	r := &Recorder{now: now, archetype: archetype, emit: emit}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

func (r *Recorder) SignalsWired() bool { return r.center() != nil }

func (r *Recorder) center() *signalcenter.Center {
	if r.signals == nil {
		return nil
	}
	return r.signals()
}

func (r *Recorder) Record(result *cyclestate.CycleResult, timings *[]phasetiming.Entry, workspace string, out recovery.PhaseOutcome) {
	out = r.stamp(out)
	result.PhasesRun = append(result.PhasesRun, cyclestate.Phase(out.Phase))
	*timings = append(*timings, timingEntry(out))
	r.emit(result.Cycle, out)
	if workspace == "" {
		r.warn("Recorder.Record", result.Cycle, out.Phase, CodeSidecarSkipped, "empty workspace: "+out.Phase+"-usage.json not written, the in-memory record kept", nil)
		return
	}
	path := UsageSidecarPath(workspace, out.Phase)
	if out.AttemptCount == 0 && sidecarExists(path) {
		return
	}
	sidecar := UsageSidecar{
		Phase: out.Phase, CostUSD: out.CostUSD, DurationMS: out.DurationMS, AttemptCount: out.AttemptCount, Verdict: out.Verdict,
		StartedAt: out.StartedAt, EndedAt: out.EndedAt, Archetype: out.Archetype, AbortReason: out.AbortReason, Tokens: out.Tokens,
	}
	data, err := json.MarshalIndent(sidecar, "", "  ")
	if err != nil {
		r.warn("Recorder.Record", result.Cycle, out.Phase, CodeSidecarWriteFailed, "usage sidecar marshal failed: "+err.Error(), map[string]string{"path": path})
		return
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		r.warn("Recorder.Record", result.Cycle, out.Phase, CodeSidecarWriteFailed, "usage sidecar write failed: "+err.Error(), map[string]string{"path": path})
	}
}

func (r *Recorder) RecordEnding(timings *[]phasetiming.Entry, out recovery.PhaseOutcome) {
	*timings = append(*timings, timingEntry(r.stamp(out)))
}

func (r *Recorder) stamp(out recovery.PhaseOutcome) recovery.PhaseOutcome {
	out.EndedAt = r.now().UTC().Format(time.RFC3339)
	out.Archetype = r.archetype(out.Phase)
	return out
}

func timingEntry(out recovery.PhaseOutcome) phasetiming.Entry {
	fillRatio, windowHot := contextFillFor(out)
	return phasetiming.Entry{
		Phase: out.Phase, DurationMS: out.DurationMS, BootMS: out.BootMS, Verdict: out.Verdict, CostUSD: out.CostUSD,
		StartedAt: out.StartedAt, EndedAt: out.EndedAt, Archetype: out.Archetype, AttemptCount: out.AttemptCount,
		AbortReason: out.AbortReason, ModelSource: out.ModelSource, ResolvedModel: out.ResolvedModel,
		Tokens: out.Tokens, Diagnostics: out.Diagnostics, ContextFillRatio: fillRatio, ContextWindowHot: windowHot,
	}
}

func sidecarExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (r *Recorder) WritePhaseTimings(workspace string, live []phasetiming.Entry) []phasetiming.Entry {
	if workspace == "" {
		r.warn("Recorder.WritePhaseTimings", 0, "", CodeTimingSkipped, "empty workspace: phase-timing.json not written", nil)
		return live
	}
	timings := composePhaseTimings(workspace, live)
	path := phasetiming.Path(workspace)
	fields := map[string]string{"path": path}
	data, err := json.Marshal(timings)
	if err != nil {
		r.warn("Recorder.WritePhaseTimings", 0, "", CodeTimingWriteFailed, "phase-timing marshal failed: "+err.Error(), fields)
		return timings
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		r.warn("Recorder.WritePhaseTimings", 0, "", CodeTimingWriteFailed, "phase-timing write failed: "+err.Error(), fields)
		return timings
	}
	if err := os.Rename(tmp, path); err != nil {
		r.warn("Recorder.WritePhaseTimings", 0, "", CodeTimingWriteFailed, "phase-timing rename failed: "+err.Error(), fields)
	}
	return timings
}

func composePhaseTimings(workspace string, live []phasetiming.Entry) []phasetiming.Entry {
	prev, err := os.ReadFile(phasetiming.Path(workspace))
	if err != nil {
		return live
	}
	var existing []phasetiming.Entry
	if err := json.Unmarshal(prev, &existing); err != nil || len(existing) == 0 {
		return live
	}
	return append(existing, live...)
}

func contextFillFor(out recovery.PhaseOutcome) (ratio float64, hot bool) {
	ratio, err := contextfill.FillRatio(out.Tokens, contextfill.WindowSizeForTier(out.ResolvedModel))
	if err != nil {
		return 0, false
	}
	return ratio, contextfill.IsHot(ratio)
}

func (r *Recorder) warn(origin string, cycle int, phase string, code signalcenter.Code, reason string, fields map[string]string) {
	r.center().Emit(signalcenter.Event{
		Cycle: cycle, Phase: phase, Module: signalcenter.ModuleOutcome, Origin: origin, Kind: signalcenter.KindOutcomeWarning,
		Severity: signalcenter.SeverityWarn, Code: code, Reason: reason, Fields: fields,
	})
}
