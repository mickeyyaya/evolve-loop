package usageprobe

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/clicontrol"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
)

type Verdict string

const (
	VerdictExhausted   Verdict = "exhausted"
	VerdictHealthy     Verdict = "healthy"
	VerdictUnavailable Verdict = "unavailable"
	VerdictUnknown     Verdict = "unknown"
)

type Evidence struct {
	CLI        string                   `json:"cli"`
	Family     string                   `json:"family"`
	Verdict    Verdict                  `json:"verdict"`
	Windows    []quotastate.UsageWindow `json:"windows,omitempty"`
	Detail     string                   `json:"detail"`
	ObservedAt time.Time                `json:"observed_at"`
	Cached     bool                     `json:"cached,omitempty"`
}

func (e Evidence) Summary() string {
	switch e.Verdict {
	case VerdictExhausted:
		return "quota exhausted, verified by a usage query: " + e.Detail
	case VerdictHealthy:
		return "quota ruled out by a usage query: " + e.Detail
	case VerdictUnavailable:
		return "the usage query itself failed (" + e.Detail + "), which points at an auth, install or network cause"
	}
	return "quota could not be verified: " + e.Detail
}

type EvidenceSource struct {
	Prober    *Prober
	EvolveDir string
	Timeout   time.Duration
	TTL       time.Duration
	Now       func() time.Time
	Act       bool

	mu      sync.Mutex
	cache   map[string]Observation
	flights map[string]*flight
}

type flight struct {
	done chan struct{}
	obs  Observation
	err  error
}

type Query struct {
	CLI    string
	Family string
	Since  time.Time
}

func (s *EvidenceSource) Explain(ctx context.Context, q Query) Evidence {
	obs, cached, err := s.observation(ctx, q)
	switch {
	case errors.Is(err, clicontrol.ErrUnsupported):
		return s.unknown(q, "the "+q.CLI+" CLI declares no usage command")
	case err != nil:
		return s.unknown(q, "the caller stopped waiting for the usage query: "+err.Error())
	}
	ev := verdictOf(obs, q.Family)
	ev.Cached = cached
	return ev
}

func (s *EvidenceSource) unknown(q Query, detail string) Evidence {
	return Evidence{CLI: q.CLI, Family: q.Family, Verdict: VerdictUnknown, Detail: detail, ObservedAt: s.now()}
}

func (s *EvidenceSource) now() time.Time {
	if s.Now == nil {
		return time.Now()
	}
	return s.Now()
}

func (s *EvidenceSource) observation(ctx context.Context, q Query) (Observation, bool, error) {
	f, leader, cached := s.join(q)
	if cached != nil {
		return *cached, true, nil
	}
	if leader {
		go s.lead(context.WithoutCancel(ctx), q, f)
	}
	select {
	case <-f.done:
		return f.obs, false, f.err
	case <-ctx.Done():
		return Observation{}, false, ctx.Err()
	}
}

func (s *EvidenceSource) lead(ctx context.Context, q Query, f *flight) {
	f.obs, f.err = s.query(ctx, q.CLI, q.Family)
	s.land(q.CLI, f)
}

func (s *EvidenceSource) join(q Query) (f *flight, leader bool, cached *Observation) {
	cli := q.CLI
	s.mu.Lock()
	defer s.mu.Unlock()
	if obs, ok := s.fresh(q); ok {
		return nil, false, &obs
	}
	if f, ok := s.flights[cli]; ok {
		return f, false, nil
	}
	if s.flights == nil {
		s.flights = map[string]*flight{}
	}
	f = &flight{done: make(chan struct{})}
	s.flights[cli] = f
	return f, true, nil
}

func (s *EvidenceSource) land(cli string, f *flight) {
	s.mu.Lock()
	delete(s.flights, cli)
	if !errors.Is(f.err, clicontrol.ErrUnsupported) {
		if s.cache == nil {
			s.cache = map[string]Observation{}
		}
		s.cache[cli] = f.obs
	}
	s.mu.Unlock()
	close(f.done)
}

func (s *EvidenceSource) fresh(q Query) (Observation, bool) {
	if obs, ok := s.cache[q.CLI]; ok && s.reusable(obs, q) {
		return obs, true
	}
	if s.EvolveDir == "" {
		return Observation{}, false
	}
	recorded, err := LoadObservations(s.EvolveDir)
	if obs := recorded[q.CLI]; err == nil && s.reusable(obs, q) {
		return obs, true
	}
	return Observation{}, false
}

func (s *EvidenceSource) reusable(o Observation, q Query) bool {
	now := s.now()
	young := o.CLI != "" && now.Sub(o.ObservedAt) < s.TTL
	describesTheFailure := !o.ObservedAt.Before(q.Since) || !rulesOut(o, q.Family)
	return young && describesTheFailure && !resetPassed(o, q.Family, now)
}

func resetPassed(o Observation, family string, now time.Time) bool {
	var latest time.Time
	for _, w := range windowsFor(o.Windows, family) {
		if w.ExhaustsFamily() && w.ResetsAt != nil && w.ResetsAt.After(latest) {
			latest = *w.ResetsAt
		}
	}
	return !latest.IsZero() && !now.Before(latest)
}

func (s *EvidenceSource) query(ctx context.Context, cli, family string) (Observation, error) {
	qctx, cancel := context.WithTimeout(ctx, s.Timeout)
	defer cancel()
	pane, err := s.Prober.Probe(qctx, cli)
	obs := Observation{CLI: cli, ObservedAt: s.now()}
	switch {
	case errors.Is(err, clicontrol.ErrUnsupported):
		return obs, err
	case err != nil:
		obs.Error = err.Error()
	default:
		r := s.Prober.read(cli, pane)
		obs.Windows = r.windows
		if r.regexWall {
			obs.RegexWallFamily = family
		}
		s.act(cli, family, r)
	}
	s.remember(obs)
	return obs, nil
}

func (s *EvidenceSource) act(cli, family string, r reading) {
	switch {
	case !s.Act:
	case len(r.windows) > 0:
		s.Prober.judgeWindows(cli, r.windows)
	case r.regexWall:
		s.Prober.benchFamily(family, r.pane)
	}
}

func (s *EvidenceSource) remember(obs Observation) {
	if !s.Act || s.EvolveDir == "" {
		return
	}
	if err := RecordObservation(s.EvolveDir, obs); err != nil && s.Prober.Log != nil {
		fmt.Fprintf(s.Prober.Log, "[usage-evidence] WARN %s: recording the usage query failed: %v\n", obs.CLI, err)
	}
}

func rulesOut(obs Observation, family string) bool {
	return verdictOf(obs, family).Verdict == VerdictHealthy
}

func verdictOf(obs Observation, family string) Evidence {
	ev := Evidence{CLI: obs.CLI, Family: family, ObservedAt: obs.ObservedAt}
	switch {
	case obs.Error != "":
		ev.Verdict, ev.Detail = VerdictUnavailable, obs.Error
	case obs.RegexWallFamily == family:
		ev.Verdict, ev.Detail = VerdictExhausted, "the "+obs.CLI+" usage screen matched its manifest's exhausted_regex for "+family+"; no window was read"
	default:
		ev.Windows = windowsFor(obs.Windows, family)
		ev.Verdict, ev.Detail = windowVerdict(ev.Windows, obs, family)
	}
	return ev
}

func windowsFor(windows []quotastate.UsageWindow, family string) []quotastate.UsageWindow {
	var out []quotastate.UsageWindow
	for _, w := range windows {
		if w.Family == family {
			out = append(out, w)
		}
	}
	return out
}

func windowVerdict(windows []quotastate.UsageWindow, obs Observation, family string) (Verdict, string) {
	if len(windows) == 0 {
		return VerdictUnknown, fmt.Sprintf("the %s usage screen showed %d window(s), none for %s", obs.CLI, len(obs.Windows), family)
	}
	var familyScoped, drained, perModel []string
	for _, w := range windows {
		switch {
		case w.ExhaustsFamily():
			drained = append(drained, w.Evidence())
		case w.Model != "" && w.Exhausted:
			perModel = append(perModel, w.Evidence())
		case w.Model == "":
			familyScoped = append(familyScoped, w.Evidence())
		}
	}
	if len(drained) > 0 {
		return VerdictExhausted, strings.Join(drained, "; ")
	}
	detail := strings.Join(familyScoped, "; ")
	if len(perModel) > 0 {
		detail += "; per-model window exhausted, which is no family cause but fails a phase on that model: " + strings.Join(perModel, "; ")
	}
	return VerdictHealthy, detail
}
