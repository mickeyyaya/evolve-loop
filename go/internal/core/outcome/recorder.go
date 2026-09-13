// Package outcome is unit 01 of the component breakdown (ADR-0103): the C1
// recording chokepoint (ADR-0044) as its own small unit. Every terminal
// disposition of a dispatched phase is recorded exactly once — into the cycle
// result's PhasesRun, the phase-timing log and the <phase>-usage.json sidecar
// — with the end-of-dispatch clock, the phase archetype and the context fill
// stamped here, so no call site re-reads the clock or re-derives the taxonomy.
// The orchestrator injects its clock, its live archetype lookup and its
// phase.outcome emission (the producer keeps its module); the unit's own
// failure modes are outcome.warning signals under module outcome. Design:
// docs/architecture/decomposition/01-outcome-recorder.md.
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

// The unit's codes: one per condition it reports, registered with their reasons.
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

// UsageSidecar is the declared on-disk contract of <phase>-usage.json: the
// terminal attempt's cost, duration, verdict and stamps, beside the timing
// log's entry for the same dispatch (ADR-0044 C1). Legacy sidecars without
// Tokens parse to zero.
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

// UsageSidecarPath is where a phase's sidecar lives in a cycle workspace.
func UsageSidecarPath(workspace, phase string) string {
	return filepath.Join(workspace, phase+"-usage.json")
}

// Recorder is the chokepoint object: one owner of the end-of-dispatch stamps
// and of the three records. Construct it once per orchestrator, after the
// orchestrator's options are applied; every collaborator it holds is read
// LIVE (the clock through a closure, the archetype lookup as a method value,
// the Center through an accessor), so an option applied later still takes.
type Recorder struct {
	now       func() time.Time
	archetype func(phase string) string
	emit      func(cycle int, out recovery.PhaseOutcome)
	signals   func() *signalcenter.Center
}

// Option configures a Recorder at construction (functional options).
type Option func(*Recorder)

// WithSignals installs the accessor of the Signal Center the unit reports its
// own failure modes through. It is an accessor, not a value, for the same
// reason the clock is a closure: the orchestrator's Center is itself an
// option that tests apply after construction, and a snapshot would keep
// reporting into nothing. A nil accessor, or one that returns nil, is the Null
// Object (tests only; the production roots always wire a Center, and
// SignalsWired proves it).
func WithSignals(c func() *signalcenter.Center) Option {
	return func(r *Recorder) {
		if c != nil {
			r.signals = c
		}
	}
}

// NewRecorder injects the orchestrator's clock, its live archetype lookup
// (the catalog changes mid-cycle, so this must be a closure, never a
// snapshot) and its phase.outcome emission, which is called at the same
// point of every record — after the in-memory record, before the sidecar.
func NewRecorder(now func() time.Time, archetype func(phase string) string, emit func(cycle int, out recovery.PhaseOutcome), opts ...Option) *Recorder {
	r := &Recorder{now: now, archetype: archetype, emit: emit}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// SignalsWired reports whether a Center is reachable now — the root's wiring proof.
func (r *Recorder) SignalsWired() bool { return r.center() != nil }

// center resolves the Center at the moment of use; nil when none was wired.
func (r *Recorder) center() *signalcenter.Center {
	if r.signals == nil {
		return nil
	}
	return r.signals()
}

// Record is the C1 chokepoint: it stamps EndedAt, Archetype and the context
// fill, appends the phase to result.PhasesRun and the entry to *timings (both
// mutated in place, as every caller relies on), raises the orchestrator's
// phase.outcome through the injected hook, then writes the usage sidecar —
// skipped with a signal when the workspace is empty, because filepath.Join
// on "" is CWD-relative and once leaked sidecars into the test tree.
func (r *Recorder) Record(result *cyclestate.CycleResult, timings *[]phasetiming.Entry, workspace string, out recovery.PhaseOutcome) {
	out.EndedAt = r.now().UTC().Format(time.RFC3339)
	out.Archetype = r.archetype(out.Phase)
	result.PhasesRun = append(result.PhasesRun, cyclestate.Phase(out.Phase))
	fillRatio, windowHot := contextFillFor(out)
	*timings = append(*timings, phasetiming.Entry{
		Phase: out.Phase, DurationMS: out.DurationMS, BootMS: out.BootMS, Verdict: out.Verdict, CostUSD: out.CostUSD,
		StartedAt: out.StartedAt, EndedAt: out.EndedAt, Archetype: out.Archetype, AttemptCount: out.AttemptCount,
		AbortReason: out.AbortReason, ModelSource: out.ModelSource, ResolvedModel: out.ResolvedModel,
		Tokens: out.Tokens, Diagnostics: out.Diagnostics, ContextFillRatio: fillRatio, ContextWindowHot: windowHot,
	})
	r.emit(result.Cycle, out)
	if workspace == "" {
		r.warn("Recorder.Record", result.Cycle, out.Phase, CodeSidecarSkipped, "empty workspace: "+out.Phase+"-usage.json not written, the in-memory record kept", nil)
		return
	}
	sidecar := UsageSidecar{
		Phase: out.Phase, CostUSD: out.CostUSD, DurationMS: out.DurationMS, AttemptCount: out.AttemptCount, Verdict: out.Verdict,
		StartedAt: out.StartedAt, EndedAt: out.EndedAt, Archetype: out.Archetype, AbortReason: out.AbortReason, Tokens: out.Tokens,
	}
	path := UsageSidecarPath(workspace, out.Phase)
	// A plain struct fails to encode only on a non-finite float (a NaN cost
	// from an upstream defect); writing the nil result would truncate the
	// sidecar silently, so the failure is a signal and nothing is written.
	data, err := json.MarshalIndent(sidecar, "", "  ")
	if err != nil {
		r.warn("Recorder.Record", result.Cycle, out.Phase, CodeSidecarWriteFailed, "usage sidecar marshal failed: "+err.Error(), map[string]string{"path": path})
		return
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		r.warn("Recorder.Record", result.Cycle, out.Phase, CodeSidecarWriteFailed, "usage sidecar write failed: "+err.Error(), map[string]string{"path": path})
	}
}

// WritePhaseTimings persists phase-timing.json atomically with APPEND-MERGE
// semantics: entries already on disk (a crashed earlier attempt, the
// pre-resume phases) come first and this invocation's entries are appended —
// the log is a record of real dispatches, so a phase appearing twice is
// reality, not duplication. It returns the composed set every consumer must
// project (the durable log and both dossier paths see one record). Failures
// are signalled, never mask the cycle outcome.
func (r *Recorder) WritePhaseTimings(workspace string, live []phasetiming.Entry) []phasetiming.Entry {
	if workspace == "" {
		r.warn("Recorder.WritePhaseTimings", 0, "", CodeTimingSkipped, "empty workspace: phase-timing.json not written", nil)
		return live
	}
	timings := composePhaseTimings(workspace, live)
	path := phasetiming.Path(workspace)
	fields := map[string]string{"path": path}
	data, err := json.Marshal(timings) // fails only on a non-finite float: signalled, the log left untouched
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

// composePhaseTimings is the ONE composition rule for a cycle's phase-timing
// log: entries already on disk first, this invocation's entries appended;
// an absent, unreadable or empty log yields the live set alone. Unexported:
// the dossier writers project the set WritePhaseTimings returns, never a
// second composition.
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

// contextFillFor derives the terminal attempt's context-window occupancy.
// ResolvedModel is the tier the attempt ran at, so it is the honest lookup
// key; anything that is not a canonical tier yields an absent fill (0, false)
// — never a guessed window, and the error never propagates. Unexported so the
// fill is derived in Record alone (the ADR-0044 chokepoint rule).
func contextFillFor(out recovery.PhaseOutcome) (ratio float64, hot bool) {
	ratio, err := contextfill.FillRatio(out.Tokens, contextfill.WindowSizeForTier(out.ResolvedModel))
	if err != nil {
		return 0, false
	}
	return ratio, contextfill.IsHot(ratio)
}

// warn is the unit's one producer: an outcome.warning WARN under module
// outcome, the origin naming the method, the phase and the cycle when known.
// A nil Center is the Null Object (Emit on nil is a no-op).
func (r *Recorder) warn(origin string, cycle int, phase string, code signalcenter.Code, reason string, fields map[string]string) {
	r.center().Emit(signalcenter.Event{
		Cycle: cycle, Phase: phase, Module: signalcenter.ModuleOutcome, Origin: origin, Kind: signalcenter.KindOutcomeWarning,
		Severity: signalcenter.SeverityWarn, Code: code, Reason: reason, Fields: fields,
	})
}
