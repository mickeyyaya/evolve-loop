// Package observer is the per-phase stall detector that `evolve loop` runs
// beside every phase: it watches the phase's output for progress and writes
// NDJSON events. See docs/architecture/packages/internal-adapters-observer.md.
package observer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/observerengine"
	"github.com/mickeyyaya/evolve-loop/go/internal/panewatch"
)

const (
	DefaultPollS  = 5 * time.Second
	DefaultStallS = 600 * time.Second
	DefaultNudgeS = 300 * time.Second

	observerEventsSuffix = observerengine.EventsSuffix

	activityScanMaxFiles = 500
)

const (
	eventStallNoProgress = "stall_no_progress"
	eventStallNoOutput   = "stall_no_output"
	sourcePane           = "pane"
	sourceStdout         = "stdout"
)

type Event struct {
	TS       string `json:"ts"`
	Type     string `json:"type"`
	Severity string `json:"severity"`
	Cycle    int    `json:"cycle"`
	Phase    string `json:"phase"`
	Agent    string `json:"agent"`
	Reason   string `json:"reason,omitempty"`
	Source   string `json:"source,omitempty"`
	Session  string `json:"session,omitempty"`
	Busy     bool   `json:"busy,omitempty"`
}

type Config struct {
	StallS       time.Duration
	PollS        time.Duration
	Cycle        int
	Phase        string
	Agent        string
	StdoutLog    string
	WorkspaceDir string

	LivenessProbe func() bool

	OnEvent func(Event)

	PaneWatch func() (panewatch.Snapshot, bool, error)
}

type Observer struct {
	cfg         Config
	sink        io.Writer
	quit        chan struct{}
	once        sync.Once
	sinkWriteMu sync.Mutex
	nowFunc     func() time.Time
}

func New(cfg Config, sink io.Writer) *Observer {
	if cfg.StallS == 0 {
		cfg.StallS = DefaultStallS
	}
	if cfg.PollS == 0 {
		cfg.PollS = DefaultPollS
	}
	return &Observer{
		cfg:     cfg,
		sink:    sink,
		quit:    make(chan struct{}),
		nowFunc: time.Now,
	}
}

func (o *Observer) Watch(ctx context.Context) error {
	o.emit("started", "info", "observer attached")
	st := watchState{lastGrowth: o.nowFunc(), lastSize: o.statSize(), lastActivity: o.newestActivity()}

	ticker := time.NewTicker(o.cfg.PollS)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			o.emit("stopped", "info", "context canceled")
			return ctx.Err()
		case <-o.quit:
			o.emit("stopped", "info", "stop requested")
			return nil
		case <-ticker.C:
			o.poll(&st)
		}
	}
}

type watchState struct {
	lastGrowth   time.Time
	lastSize     int64
	lastActivity time.Time
	paneProgress time.Time
	stallEmitted bool
	paneReadBad  bool
}

func (st *watchState) progressed(now time.Time) {
	st.lastGrowth = now
	st.stallEmitted = false
}

func (o *Observer) stallDue(st *watchState) bool {
	return !st.stallEmitted && o.nowFunc().Sub(st.lastGrowth) >= o.cfg.StallS
}

func (o *Observer) poll(st *watchState) {
	if snap, ok := o.paneSnapshot(st); ok {
		o.pollPane(st, snap)
		return
	}
	o.pollOutput(st)
}

func (o *Observer) paneSnapshot(st *watchState) (panewatch.Snapshot, bool) {
	if o.cfg.PaneWatch == nil {
		return panewatch.Snapshot{}, false
	}
	snap, ok, err := o.cfg.PaneWatch()
	if err != nil && !st.paneReadBad {
		st.paneReadBad = true
		o.emit("pane_watch_unreadable", "info", err.Error()+" (falling back to stdout and workspace activity)")
	}
	return snap, ok && err == nil
}

func (o *Observer) pollPane(st *watchState, snap panewatch.Snapshot) {
	if snap.ProgressAt.After(st.paneProgress) {
		st.paneProgress = snap.ProgressAt
		st.progressed(o.nowFunc())
		return
	}
	if !o.stallDue(st) {
		return
	}
	state := "idle at its prompt"
	if snap.Busy {
		state = "busy"
	}
	o.emitEvent(Event{Type: eventStallNoProgress, Severity: "incident", Source: sourcePane, Session: snap.Session, Busy: snap.Busy,
		Reason: fmt.Sprintf("pane %s showed no new transcript for %s (%s)", snap.Session, o.cfg.StallS, state)})
	st.stallEmitted = true
}

func (o *Observer) pollOutput(st *watchState) {
	sz := o.statSize()
	act := o.newestActivity()
	if sz > st.lastSize || act.After(st.lastActivity) {
		st.lastSize = sz
		if act.After(st.lastActivity) {
			st.lastActivity = act
		}
		st.progressed(o.nowFunc())
		return
	}
	if !o.stallDue(st) {
		return
	}
	if o.cfg.LivenessProbe != nil && o.cfg.LivenessProbe() {
		st.progressed(o.nowFunc())
		o.emit("stall_probe_active", "info",
			fmt.Sprintf("no fs growth for %s but liveness probe active", o.cfg.StallS))
		return
	}
	o.emitEvent(Event{Type: eventStallNoOutput, Severity: "incident", Source: sourceStdout, Reason: o.outputStallReason()})
	st.stallEmitted = true
}

func (o *Observer) outputStallReason() string {
	if o.cfg.WorkspaceDir == "" {
		return fmt.Sprintf("no stdout growth for %s", o.cfg.StallS)
	}
	return fmt.Sprintf("no stdout growth and no workspace file activity for %s", o.cfg.StallS)
}

func (o *Observer) newestActivity() time.Time {
	if o.cfg.WorkspaceDir == "" {
		return time.Time{}
	}
	var newest time.Time
	seen := 0
	_ = filepath.Walk(o.cfg.WorkspaceDir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasSuffix(info.Name(), observerEventsSuffix) {
			return nil
		}
		if seen >= activityScanMaxFiles {
			return filepath.SkipAll
		}
		seen++
		if mt := info.ModTime(); mt.After(newest) {
			newest = mt
		}
		return nil
	})
	return newest
}

func (o *Observer) Stop() error {
	o.once.Do(func() { close(o.quit) })
	return nil
}

func (o *Observer) statSize() int64 {
	if o.cfg.StdoutLog == "" {
		return 0
	}
	info, err := os.Stat(o.cfg.StdoutLog)
	if err != nil {
		return 0
	}
	return info.Size()
}

func (o *Observer) emit(eventType, severity, reason string) {
	o.emitEvent(Event{Type: eventType, Severity: severity, Reason: reason})
}

func (o *Observer) emitEvent(e Event) {
	e.TS = o.nowFunc().UTC().Format(time.RFC3339Nano)
	e.Cycle, e.Phase, e.Agent = o.cfg.Cycle, o.cfg.Phase, o.cfg.Agent
	o.sinkWriteMu.Lock()
	if b, err := json.Marshal(e); err == nil {
		_, _ = o.sink.Write(append(b, '\n'))
	}
	o.sinkWriteMu.Unlock()
	if o.cfg.OnEvent != nil {
		o.cfg.OnEvent(e)
	}
}
