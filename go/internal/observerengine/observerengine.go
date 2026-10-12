// Package observerengine tails a phase's stream, applies the stall rules and
// writes the observer's events and report, one Tick at a time.
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

const (
	ScopePhase = "phase"
	ScopeCycle = "cycle"
)

const (
	StdoutSuffix = "-stdout.log"
	EventsSuffix = "-observer-events.ndjson"
	ReportSuffix = "-observer-report.json"
)

type Paths struct{ Stdout, Events, Report string }

func PathsFor(workspace, name string) Paths {
	return Paths{
		Stdout: filepath.Join(workspace, name+StdoutSuffix),
		Events: filepath.Join(workspace, name+EventsSuffix),
		Report: filepath.Join(workspace, name+ReportSuffix),
	}
}

type Settings struct {
	Cycle          int
	Phase, Agent   string
	Scope          string
	Enforce        bool
	PGID           int
	PID            int
	PollS          int
	StallS         int
	NudgeS         int
	EOFGraceS      int
	HeartbeatEvery int
	MaxNoProgressS int
	NudgeBody      string
	Paths          Paths
}

type Deps struct {
	Now          func() time.Time
	ProcessAlive func(pgid int) bool
	Kill         func(pgid int, sig syscall.Signal) error
	Nudge        func(body string) error
	Policy       recovery.StallPolicy
}

type Engine struct {
	s          Settings
	d          Deps
	rep        *Reporter
	openAppend func(path string) (io.WriteCloser, error)

	traceID          string
	startedAt        time.Time
	startedAtISO     string
	lastEventTS      time.Time
	lastProgressTS   time.Time
	lastByteOff      int64
	eventCount       int
	toolCallCount    int
	errorCount       int
	toolResultCnt    int
	rateLimitCnt     int
	cumulativeCost   float64
	cacheReadTok     int
	cacheCreateTok   int
	nudged           bool
	processDeadFired bool
	incidents        []map[string]any
	pollCounter      int
	eofQuietCount    int
	faultsSeen       map[string]bool
}

type Option func(*Engine)

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

func WithSignals(c func() *signalcenter.Center) Option {
	return func(e *Engine) { e.rep = NewReporter(c) }
}

func (e *Engine) SignalsWired() bool { return e.rep.Wired() }

func (e *Engine) reportFault(origin string, code signalcenter.Code, reason string, fields map[string]string) {
	e.rep.Report(origin, e.s.Cycle, e.s.Phase, code, reason, fields)
}

func (e *Engine) faultOnce(origin string, code signalcenter.Code, op, reason string, fields map[string]string) {
	key := string(code) + "/" + op
	if e.faultsSeen[key] {
		return
	}
	e.faultsSeen[key] = true
	e.reportFault(origin, code, reason, fields)
}

type Reporter struct{ signals func() *signalcenter.Center }

func NewReporter(signals func() *signalcenter.Center) *Reporter {
	return &Reporter{signals: signals}
}

func (r *Reporter) center() *signalcenter.Center {
	if r == nil || r.signals == nil {
		return nil
	}
	return r.signals()
}

func (r *Reporter) Wired() bool { return r.center() != nil }

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

func openAppendFile(path string) (io.WriteCloser, error) {
	return os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
}
