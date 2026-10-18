package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strconv"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/filter"
	"github.com/mickeyyaya/evolve-loop/go/internal/events/reader"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var oversightChannels = []string{"loop", "cycle", "errors", "ship"}

var channelRoutes = map[string]string{
	"loop":   "module=loop",
	"cycle":  "kind=phase.dispatched,phase.outcome,phase.aborted,cycle.sealed,ship.landed",
	"errors": "severity>=WARN",
	"ship":   "kind=ship.*",
}

var quotaSignals = map[signalcenter.Code]bool{CodeUsageEvidence: true, CodePaneExhausted: true}

var heartbeatKinds = map[signalcenter.Kind]bool{
	signalcenter.KindPhaseDispatched: true, signalcenter.KindPhaseOutcome: true, signalcenter.KindPhaseAborted: true,
}

type LandingObserver interface {
	CleanLanding()
	Conflict()
}

type Source interface {
	Next(ctx context.Context, deadline time.Time) ([]reader.Item, error)
}

type Oversight struct {
	Stage   string
	Events  Source
	Leases  *Leases
	Intake  *Intake
	Landing LandingObserver
	Locate  func(Lane) (int, bool)
	Resize  func() (target int, paused []string)
	Publish Publish

	cycles    map[int]Lane
	foreign   map[int]uint64
	conflicts map[int]bool
	paused    map[string]bool
}

type held struct {
	lane Lane
	live bool
}

type action struct {
	names  []string
	lane   string
	fields map[string]string
	apply  func()
}

func Subscribe(root, since string, ports reader.Ports) (*reader.Reader, error) {
	start, err := reader.ParseSince(since)
	if err != nil {
		return nil, fmt.Errorf("orchestrator: oversight start: %w", err)
	}
	catalog := make([]reader.Channel, 0, len(oversightChannels))
	for _, name := range oversightChannels {
		route, _, err := filter.Parse(channelRoutes[name], filter.RegisteredCatalog())
		if err != nil {
			return nil, fmt.Errorf("orchestrator: route of channel %s: %w", name, err)
		}
		catalog = append(catalog, reader.Channel{Name: name, Route: route})
	}
	return reader.New(reader.Config{Root: root, Catalog: catalog, Selectors: oversightChannels, Since: start}, ports), nil
}

var errOversightPorts = errors.New("orchestrator: the oversight needs its events, leases, intake, locate and resize ports")

func (o *Oversight) Run(ctx context.Context) error {
	if o.Events == nil || o.Leases == nil || o.Intake == nil || o.Locate == nil || o.Resize == nil {
		return errOversightPorts
	}
	o.cycles, o.foreign, o.conflicts, o.paused = map[int]Lane{}, map[int]uint64{}, map[int]bool{}, map[string]bool{}
	for {
		items, err := o.Events.Next(ctx, time.Time{})
		if err != nil {
			return fmt.Errorf("orchestrator: oversight: read the event channels: %w", err)
		}
		for _, it := range items {
			if it.Record.Signal != nil {
				o.handle(*it.Record.Signal)
			}
		}
	}
}

func (o *Oversight) handle(e signalcenter.Event) {
	switch {
	case e.Code == CodePhaseStalled:
		o.stalled(e)
	case shiperr.IsLandingConflict(e.Code):
		o.rebased(e)
	case quotaSignals[e.Code]:
		o.requota(e)
	case e.Kind == signalcenter.KindCycleSealed:
		o.sealed(e)
	case e.Kind == signalcenter.KindShipLanded:
		o.landed(e)
	case e.Kind == signalcenter.KindLoopHalt:
		o.do(e, action{names: []string{"stop_admission"}, apply: o.Intake.Stop})
	case heartbeatKinds[e.Kind] || e.Module == signalcenter.ModuleObserver:
		o.heartbeat(e)
	}
}

func (o *Oversight) laneOf(cycle int) (held, bool) {
	if cycle <= 0 {
		return held{}, false
	}
	if _, ok := o.cycles[cycle]; !ok {
		o.locate(cycle)
	}
	lane, ok := o.cycles[cycle]
	return held{lane: lane, live: ok && o.lives(lane)}, ok
}

func (o *Oversight) locate(cycle int) {
	epoch := o.Leases.Epoch()
	if seen, ok := o.foreign[cycle]; ok && seen == epoch {
		return
	}
	unlocated := false
	for _, lane := range o.Leases.Live() {
		if c, ok := o.Locate(lane); ok {
			o.cycles[c] = lane
		} else {
			unlocated = true
		}
	}
	if _, ok := o.cycles[cycle]; !ok && !unlocated {
		o.foreign[cycle] = epoch
	}
}

func (o *Oversight) lives(lane Lane) bool {
	cur, ok := o.Leases.Lane(lane.Key)
	return ok && cur.Started.Equal(lane.Started)
}

func (o *Oversight) heartbeat(e signalcenter.Event) {
	at, err := time.Parse(time.RFC3339Nano, e.TS)
	if h, ok := o.laneOf(e.Cycle); ok && h.live && err == nil {
		o.Leases.Beat(h.lane.Key, at)
	}
}

func (o *Oversight) stalled(e signalcenter.Event) {
	if h, ok := o.laneOf(e.Cycle); ok && h.live {
		o.Leases.Stall(h.lane.Key)
	}
}

func (o *Oversight) rebased(e signalcenter.Event) {
	if _, ok := o.laneOf(e.Cycle); ok {
		o.conflicts[e.Cycle] = true
	}
}

func (o *Oversight) sealed(e signalcenter.Event) {
	h, ok := o.laneOf(e.Cycle)
	if !ok {
		return
	}
	delete(o.cycles, e.Cycle)
	delete(o.conflicts, e.Cycle)
	verdict := e.Fields[cyclestate.SealedVerdictField]
	a := action{names: []string{"record_verdict"}, lane: h.lane.Key, fields: map[string]string{"verdict": verdict}}
	if verdict == cyclestate.VerdictFAIL && h.live {
		a.names = append(a.names, "free_cells")
		a.apply = func() { o.Intake.Free(h.lane.Key) }
	}
	o.do(e, a)
}

func (o *Oversight) landed(e signalcenter.Event) {
	conflict := o.conflicts[e.Cycle]
	delete(o.conflicts, e.Cycle)
	h, ok := o.laneOf(e.Cycle)
	if !ok {
		return
	}
	count := "count_clean_landing"
	if conflict {
		count = "count_conflict"
	}
	names := []string{count}
	if h.live {
		names = []string{"free_cells", count, "backfill"}
	}
	o.do(e, action{names: names, lane: h.lane.Key, apply: func() {
		o.countLanding(conflict)
		if h.live {
			o.Intake.Free(h.lane.Key)
		}
	}})
}

func (o *Oversight) countLanding(conflict bool) {
	switch {
	case o.Landing == nil:
	case conflict:
		o.Landing.Conflict()
	default:
		o.Landing.CleanLanding()
	}
}

func (o *Oversight) requota(e signalcenter.Event) {
	target, paused := o.Resize()
	now := make(map[string]bool, len(paused))
	for _, family := range paused {
		now[family] = true
		if !o.paused[family] {
			o.Publish.emit("Oversight.requota", CodeFamilyPaused, signalcenter.SeverityInfo,
				"family "+family+" has no fallback and stops taking new lanes",
				o.stamped(map[string]string{"family": family, "target": strconv.Itoa(target)}))
		}
	}
	o.paused = now
	o.do(e, action{names: []string{"resize"}, fields: map[string]string{"target": strconv.Itoa(target)}, apply: func() { o.Intake.Resize(target) }})
}

func (o *Oversight) do(e signalcenter.Event, a action) {
	if o.Stage == StageEnforce && a.apply != nil {
		a.apply()
	}
	fields := map[string]string{"actions": strings.Join(a.names, ","), "lane": a.lane, "cycle": strconv.Itoa(e.Cycle), "event": string(e.Kind)}
	maps.Copy(fields, a.fields)
	o.Publish.emit("Oversight.do", CodeOversightAction, signalcenter.SeverityInfo, "oversight: "+fields["actions"]+" on "+string(e.Kind), o.stamped(fields))
}

func (o *Oversight) stamped(fields map[string]string) map[string]string {
	out := make(map[string]string, len(fields)+2)
	maps.Copy(out, fields)
	out["stage"], out["applied"] = o.Stage, strconv.FormatBool(o.Stage == StageEnforce)
	return out
}
