// Package observerengine is unit 12 of the component breakdown (ADR-0103):
// the phase-observer engine — the stream-json tail and decoder, the stall
// rules (idle nudge, hard stall, no-progress backstop, dead-process probe),
// the incident responder (the ADR-0044 StallPolicy strategy with the
// record-reflects-reality rule) and the envelope/report sinks the manual
// `evolve phase-observer` subcommand runs. The host (internal/phaseobserver)
// keeps validation, defaults, the real ticker and the select loop and drives
// the engine one Tick at a time; every collaborator is an injected function,
// so the leaf is testable with a stepped clock and no syscall, inbox or tmux.
// The engine reports its own faults as observer.warning under module observer
// through ONE Reporter, which the live core.Observer adapter shares for its
// sink-open and watcher-leak faults. Design:
// docs/architecture/decomposition/12-phaseobserver.md.
package observerengine

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/recovery"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// The unit's codes: seven WARN faults and the one INCIDENT act (the stall
// SIGTERM), registered under module observer with their triage docs.
const (
	CodeNudgeAppendFailed    signalcenter.Code = "OBSERVER_NUDGE_APPEND_FAILED"
	CodeReportWriteFailed    signalcenter.Code = "OBSERVER_REPORT_WRITE_FAILED"
	CodeStallKillSent        signalcenter.Code = "OBSERVER_STALL_KILL_SENT"
	CodeKillFailed           signalcenter.Code = "OBSERVER_KILL_FAILED"
	CodeEventAppendFailed    signalcenter.Code = "OBSERVER_EVENT_APPEND_FAILED"
	CodeStdoutTailFailed     signalcenter.Code = "OBSERVER_STDOUT_TAIL_FAILED"
	CodeEventsSinkOpenFailed signalcenter.Code = "OBSERVER_EVENTS_SINK_OPEN_FAILED"
	CodeWatcherLeaked        signalcenter.Code = "OBSERVER_WATCHER_LEAKED"
)

// severityOf is the ONE place a code's severity is decided (schema-1.0: fixed
// per code); TestSeverityTable_CompleteAndRegistered keeps it total.
var severityOf = map[signalcenter.Code]signalcenter.Severity{
	CodeNudgeAppendFailed:    signalcenter.SeverityWarn,
	CodeReportWriteFailed:    signalcenter.SeverityWarn,
	CodeStallKillSent:        signalcenter.SeverityIncident,
	CodeKillFailed:           signalcenter.SeverityWarn,
	CodeEventAppendFailed:    signalcenter.SeverityWarn,
	CodeStdoutTailFailed:     signalcenter.SeverityWarn,
	CodeEventsSinkOpenFailed: signalcenter.SeverityWarn,
	CodeWatcherLeaked:        signalcenter.SeverityWarn,
}

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleObserver, CodeNudgeAppendFailed, "the soft-stall nudge could not be appended to the agent inbox; the soft_stall_nudge envelope still emits and the nudge is not retried; fields.step=nudge, idle_s, threshold_s, agent")
	signalcenter.RegisterCode(signalcenter.ModuleObserver, CodeReportWriteFailed, "<agent>-observer-report.json could not be marshalled, its directory created, its .tmp written or renamed into place; the subcommand still exits 0 (the report is best-effort); fields.step=report, path")
	signalcenter.RegisterCode(signalcenter.ModuleObserver, CodeStallKillSent, "the observer sent SIGTERM to the agent's process group after a stall INCIDENT — an act, not a fault — after the INCIDENT envelope was appended and before the signal; fields.step=respond, kind (stuck_no_output | stuck_no_progress | process_dead), pgid, action (legacy_enforce | kill_retry), action_reason, idle_s/threshold_s when the incident carries them")
	signalcenter.RegisterCode(signalcenter.ModuleObserver, CodeKillFailed, "the SIGTERM to the agent's process group returned an error (ESRCH: already gone; EPERM: not ours) — the kill was attempted exactly once; fields.step=respond, pgid, signal, kind")
	signalcenter.RegisterCode(signalcenter.ModuleObserver, CodeEventAppendFailed, "one line of <agent>-observer-events.ndjson was lost (fields.op = marshal | mkdir | open | write); reported ONCE per op for the observer's life, later losses on the same op are silent; an INCIDENT is still retained for the report when the open succeeded; fields.step=emit, op, path, event_type")
	signalcenter.RegisterCode(signalcenter.ModuleObserver, CodeStdoutTailFailed, "the agent's stdout log could not be stat'ed (a non-ENOENT error), opened, seeked or fully scanned (a line over the 10 MiB buffer); reported ONCE per op; an absent log stays silent by design (tmux drivers dump it at exit); the prior offset is kept on stat/open/seek, the file size after a scan error; fields.step=tail, op, path")
	signalcenter.RegisterCode(signalcenter.ModuleObserver, CodeEventsSinkOpenFailed, "the live per-phase observer could not open <phase>-observer-events.ndjson, so the phase runs UNOBSERVED (ADR-0030: the observer never blocks the phase); fields.step=open_sink, path")
	signalcenter.RegisterCode(signalcenter.ModuleObserver, CodeWatcherLeaked, "the live observer's watcher goroutine did not exit within the bound after the phase finished; the goroutine and its sink fd are leaked on purpose (closing would race its writes) and the OS reclaims them at exit; fields.step=cancel, timeout_s")
}

// The two scopes the host's Scope type aliases; any other value runs no rule
// (the preserved tautology of the original :351 check).
const (
	ScopePhase = "phase"
	ScopeCycle = "cycle"
)

// The ONE spelling of the three per-agent files under a workspace.
// core.BridgePIDFile keeps a private twin of StdoutSuffix (the orchestrator
// does not depend on this engine); adapters/observer imports both and pins
// them together.
const (
	StdoutSuffix = "-stdout.log"
	EventsSuffix = "-observer-events.ndjson"
	ReportSuffix = "-observer-report.json"
)

// Paths are the three files the observer reads and writes for one agent.
type Paths struct{ Stdout, Events, Report string }

// PathsFor projects the layout for an agent (the host and the live adapter
// consume it; the watchdog and soak-report globs are follow-up F3).
func PathsFor(workspace, name string) Paths {
	return Paths{
		Stdout: filepath.Join(workspace, name+StdoutSuffix),
		Events: filepath.Join(workspace, name+EventsSuffix),
		Report: filepath.Join(workspace, name+ReportSuffix),
	}
}

// Settings is the observer's identity and thresholds, ALREADY defaulted by
// the host — the engine defaults nothing (the host's withDefaults and policy's
// ObserverConfig are the two belief owners).
type Settings struct {
	Cycle          int
	Phase, Agent   string
	Scope          string // ScopePhase | ScopeCycle; anything else runs no rule
	Enforce        bool
	PGID           int // 0 = nothing to kill
	PID            int // the observer's own pid (envelope id + source.observer_pid)
	PollS          int
	StallS         int
	NudgeS         int // 0 = the soft-stall nudge is off
	EOFGraceS      int
	HeartbeatEvery int // 0 = no heartbeat
	MaxNoProgressS int // 0 = the no-progress backstop is off
	NudgeBody      string
	Paths          Paths
}

// Deps are the engine's collaborators — explicit, injected, every one a
// function so the leaf's tests need no syscall, no inbox and no tmux.
type Deps struct {
	// Now is read at exactly the sites the original read its clock:
	// construction, per valid line, per rules tick, per no-progress tick, per
	// emit and the report (the host's count-stepping clocks depend on it).
	Now func() time.Time
	// ProcessAlive probes the agent's process group; nil = the probe is OFF.
	ProcessAlive func(pgid int) bool
	// Kill sends sig to the process group; required when PGID > 0.
	Kill func(pgid int, sig syscall.Signal) error
	// Nudge appends the soft-stall body to the agent inbox; required when
	// NudgeS > 0. The host's closure reads the clock once more (inbox.Append
	// mints the envelope TS) — the seventh clock site, host-side.
	Nudge func(body string) error
	// Policy maps a stall incident to an action; nil = the legacy Enforce
	// branch with an unenriched INCIDENT envelope (Null Object).
	Policy recovery.StallPolicy
}

// Engine is the observer's state and rules. Single-goroutine by contract: the
// host's poll loop is the sole caller, so no field is locked.
type Engine struct {
	s          Settings
	d          Deps
	rep        *Reporter
	openAppend func(path string) (io.WriteCloser, error)

	traceID        string
	startedAt      time.Time
	startedAtISO   string
	lastEventTS    time.Time
	lastProgressTS time.Time // bumped by tool_use / tool_result only (WS-E1)
	lastByteOff    int64
	eventCount     int
	toolCallCount  int
	errorCount     int
	toolResultCnt  int
	rateLimitCnt   int
	cumulativeCost float64
	cacheReadTok   int
	cacheCreateTok int
	nudged         bool
	// processDeadFired guards the dead-process INCIDENT to exactly one
	// emission — the group cannot come back; re-firing per tick is spam.
	processDeadFired bool
	incidents        []map[string]any
	pollCounter      int
	eofQuietCount    int
	faultsSeen       map[string]bool // code/op → reported once
}

// Option configures an Engine at construction (functional options).
type Option func(*Engine)

// New builds the engine over already-defaulted Settings and its ports; the
// ONE construction read of the clock seeds the trace id, the start stamp and
// both liveness clocks (the original :236-244 verbatim).
func New(s Settings, d Deps, opts ...Option) *Engine {
	now := d.Now()
	e := &Engine{
		s: s, d: d, rep: NewReporter(nil), openAppend: openAppendFile,
		traceID:      fmt.Sprintf("cycle-%d-%s-%d", s.Cycle, s.Phase, now.Unix()),
		startedAt:    now,
		startedAtISO: now.UTC().Format(tsLayout),
		lastEventTS:  now, lastProgressTS: now,
		faultsSeen: map[string]bool{},
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// WithSignals installs the accessor of the Signal Center the engine reports
// through — read at every use, so a Center installed after construction is
// reached. A nil accessor, or one returning nil, is the Null Object.
func WithSignals(c func() *signalcenter.Center) Option {
	return func(e *Engine) { e.rep = NewReporter(c) }
}

// SignalsWired reports whether the engine currently reaches a Center.
func (e *Engine) SignalsWired() bool { return e.rep.Wired() }

// reportFault stamps the engine's cycle and phase onto a report.
func (e *Engine) reportFault(origin string, code signalcenter.Code, reason string, fields map[string]string) {
	e.rep.Report(origin, e.s.Cycle, e.s.Phase, code, reason, fields)
}

// faultOnce reports a recurring fault (a broken sink or log recurs every
// PollS) exactly once per code+op for the observer's life.
func (e *Engine) faultOnce(origin string, code signalcenter.Code, op, reason string, fields map[string]string) {
	key := string(code) + "/" + op
	if e.faultsSeen[key] {
		return
	}
	e.faultsSeen[key] = true
	e.reportFault(origin, code, reason, fields)
}

// Reporter is the module's ONE producer: an observer.warning under module
// observer with the severity the code's table row fixes. Exported so the live
// adapter reports its two faults through the same helper. It never panics —
// a nil Reporter, accessor or Center is the Null Object, and an unregistered
// code degrades to WARN plus the Center's own drift stamp.
type Reporter struct{ signals func() *signalcenter.Center }

// NewReporter binds the late-bound Center accessor.
func NewReporter(signals func() *signalcenter.Center) *Reporter {
	return &Reporter{signals: signals}
}

func (r *Reporter) center() *signalcenter.Center {
	if r == nil || r.signals == nil {
		return nil
	}
	return r.signals()
}

// Wired reports whether the reporter currently reaches a Center.
func (r *Reporter) Wired() bool { return r.center() != nil }

// Report emits one event: origin is the exported entry point whose call
// produced it (Engine.Start | Engine.Tick | Engine.Shutdown |
// Engine.WriteReport | CoreAdapter.Start), fields.step names the step.
func (r *Reporter) Report(origin string, cycle int, phase string, code signalcenter.Code, reason string, fields map[string]string) {
	severity := severityOf[code]
	if severity == "" {
		severity = signalcenter.SeverityWarn
	}
	r.center().Emit(signalcenter.Event{
		Cycle: cycle, Phase: phase, Module: signalcenter.ModuleObserver, Origin: origin,
		Kind: signalcenter.KindObserverWarning, Severity: severity, Code: code, Reason: reason, Fields: fields,
	})
}

// openAppendFile is the production opener of the events file: append-only,
// created on first use (the directory is the caller's op).
func openAppendFile(path string) (io.WriteCloser, error) {
	return os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
}
