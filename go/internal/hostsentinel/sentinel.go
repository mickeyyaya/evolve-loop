package hostsentinel

import (
	"context"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type Release struct {
	Scope     string   `json:"scope"`
	Applied   bool     `json:"applied"`
	LeaseHeld bool     `json:"lease_held,omitempty"`
	Report    []string `json:"report,omitempty"`
}

func (r Release) outcome(apply bool) string {
	switch {
	case r.LeaseHeld:
		return "lease_held"
	case !apply:
		return "would"
	case r.Applied:
		return "applied"
	}
	return "failed"
}

const throttleSlack = 10

type Ports struct {
	Sample  func(context.Context) Sample
	Release func(ctx context.Context, breaches []Breach, apply bool) []Release
	Emit    func(signalcenter.Event)
	Save    func(Status) error
	Warn    func(string)
}

type Sentinel struct {
	cfg   Config
	ports Ports
	last  map[signalcenter.Code]time.Time
	hold  atomic.Bool
}

func New(cfg Config, ports Ports) *Sentinel {
	return &Sentinel{cfg: cfg, ports: ports, last: map[signalcenter.Code]time.Time{}}
}

func (s *Sentinel) HoldLanes() bool { return s != nil && s.hold.Load() }

func (s *Sentinel) Start(ctx context.Context) func() {
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(s.cfg.Interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.Tick(ctx)
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}
}

func (s *Sentinel) Tick(ctx context.Context) Status {
	sample := s.ports.Sample(ctx)
	st := Status{Stage: s.cfg.Stage, Sample: sample, Breaches: Evaluate(sample, s.cfg)}
	s.warnFailed(sample)
	apply := s.cfg.Stage == StageEnforce
	if len(st.Breaches) > 0 {
		st.Releases = s.ports.Release(ctx, st.Breaches, apply)
	}
	released := releasedField(st.Releases, apply)
	for _, b := range st.Breaches {
		s.emit(sample.At, signalcenter.Event{Code: b.Code, Reason: string(b.Metric) + " is over its threshold",
			Fields: map[string]string{"metric": string(b.Metric), "value": number(b.Value), "threshold": number(b.Threshold), "stage": string(s.cfg.Stage), "released": released}})
	}
	st.HoldLanes = s.floor(sample)
	if err := s.ports.Save(st); err != nil {
		s.ports.Warn("host sentinel: the last sample was not saved: " + err.Error())
	}
	return st
}

func (s *Sentinel) floor(sample Sample) bool {
	r, ok := sample.Readings[MetricDiskFree]
	if !ok || r.Err != "" {
		return s.hold.Load()
	}
	below := r.Value < float64(s.cfg.FloorBytes)
	hold := below && s.cfg.Stage == StageEnforce
	s.hold.Store(hold)
	if below {
		s.emit(sample.At, signalcenter.Event{Code: CodeFloorHold, Reason: "disk free is under the floor, so no new lane starts and the running lanes continue",
			Fields: map[string]string{"value": number(r.Value), "threshold": number(float64(s.cfg.FloorBytes)), "stage": string(s.cfg.Stage), "held": strconv.FormatBool(hold)}})
	}
	return hold
}

func (s *Sentinel) warnFailed(sample Sample) {
	var metrics, reasons []string
	for _, k := range checks {
		if r, ok := sample.Readings[k.metric]; ok && r.Err != "" {
			metrics = append(metrics, string(k.metric))
			reasons = append(reasons, string(k.metric)+": "+r.Err)
		}
	}
	if len(metrics) == 0 {
		return
	}
	reason := "host sample failed, the loop continues: " + strings.Join(reasons, "; ")
	s.ports.Warn(reason)
	s.emit(sample.At, signalcenter.Event{Code: CodeSampleFailed, Reason: reason, Fields: map[string]string{"metrics": strings.Join(metrics, ",")}})
}

func (s *Sentinel) emit(at time.Time, e signalcenter.Event) {
	if last, seen := s.last[e.Code]; seen && at.Sub(last) < s.cfg.Interval-s.cfg.Interval/throttleSlack {
		return
	}
	s.last[e.Code] = at
	e.Module, e.Origin, e.Kind, e.Severity = signalcenter.ModuleHost, "Sentinel.Tick", signalcenter.KindLoopWarning, signalcenter.SeverityWarn
	s.ports.Emit(e)
}

func releasedField(releases []Release, apply bool) string {
	parts := make([]string, len(releases))
	for i, r := range releases {
		parts[i] = r.Scope + "=" + r.outcome(apply)
	}
	return strings.Join(parts, ",")
}

func number(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
