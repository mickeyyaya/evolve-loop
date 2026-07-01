package panestream

import "sync"

// SignalCenter is a Facade that owns per-session liveness signal state, guarded
// by sync.RWMutex over a keyed map (ADR-0068). It aggregates all active session
// signals into one LivenessState, and exposes a handler-registration API so that
// adding a CLI = register a strategy + add a profile entry (OCP — no switch
// edits required).
//
// Concurrency model: one RWMutex guards both the sessions map and the registry
// map. Writers (Observe, RegisterHandler) hold the write lock for their full
// duration — stateful detector mutation demands exclusive access. Readers
// (Aggregate) hold a read lock. Per-session sharding is deferred to S5.
//
// Aggregation rule: any Converging session ⇒ Converging; else Hung if any
// session is Hung; else BusyButStagnant; else Idle; else 0 (empty center).
// The rule is documented and testable, not implicit.
type SignalCenter struct {
	mu       sync.RWMutex
	sessions map[string]*sessionSignals
	registry map[string]func() LivenessProbe
}

// sessionSignals holds the stateful probe and the most recent liveness verdict
// for one session key, plus the Busy/Changed projections (S4): busy and clean
// are folded from the standalone PaneBusy/cleanPane so the driver checkpoint
// never parses pane chrome a second time itself.
type sessionSignals struct {
	probe   LivenessProbe
	last    LivenessState
	busy    bool
	clean   string
	changed bool
}

// NewSignalCenter returns an empty, ready-to-use SignalCenter.
func NewSignalCenter() *SignalCenter {
	return &SignalCenter{
		sessions: make(map[string]*sessionSignals),
		registry: make(map[string]func() LivenessProbe),
	}
}

// RegisterHandler registers a factory that produces the LivenessProbe for
// sessions whose PaneProfile.Name matches name. The factory is called once per
// new session key when Observe encounters that profile for the first time.
//
// Empty name is a no-op (silently dropped). Duplicate registration is
// last-writer-wins. RegisterHandler is safe for concurrent use.
func (sc *SignalCenter) RegisterHandler(name string, factory func() LivenessProbe) {
	if name == "" {
		return
	}
	sc.mu.Lock()
	sc.registry[name] = factory
	sc.mu.Unlock()
}

// Observe records one liveness observation for sessionKey. On the first call
// for a key, the probe is created via the registry (if profile.Name is
// registered) or DetectorFor (fallback). Subsequent calls reuse the same
// stateful probe. Observe is safe for concurrent use.
func (sc *SignalCenter) Observe(sessionKey, rendered string, profile PaneProfile) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	ss, existed := sc.sessions[sessionKey]
	if !existed {
		var probe LivenessProbe
		if f, found := sc.registry[profile.Name]; found {
			probe = f()
		} else {
			probe = DetectorFor(profile)
		}
		ss = &sessionSignals{probe: probe}
		sc.sessions[sessionKey] = ss
	}

	state, _ := ss.probe.Assess(rendered, profile)
	ss.last = state

	// Busy/Changed projections (S4): folded from the standalone functions so
	// they can never drift from panestream.PaneBusy / the cleaned-content
	// diff. Changed compares this Observe's cleaned content against the PRIOR
	// Observe's for this key (checkpoint-to-checkpoint) — a brand-new key has
	// no prior observation, so its first Changed reads false.
	ss.busy = PaneBusy(rendered, profile)
	clean := cleanPane(rendered)
	ss.changed = existed && clean != ss.clean
	ss.clean = clean
}

// Busy reports the most-recent Observe's busy affordance for sessionKey
// (folded from the standalone PaneBusy for the same observed pane). An
// unknown or empty key reads false — Busy never panics on an unobserved
// session. Safe for concurrent use alongside Observe.
func (sc *SignalCenter) Busy(sessionKey string) bool {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	ss, ok := sc.sessions[sessionKey]
	if !ok {
		return false
	}
	return ss.busy
}

// Changed reports whether the most-recent Observe's cleaned content (chrome
// stripped) differs from the prior Observe's for sessionKey. A key with fewer
// than two observations, or that was never observed, reads false. Safe for
// concurrent use alongside Observe.
func (sc *SignalCenter) Changed(sessionKey string) bool {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	ss, ok := sc.sessions[sessionKey]
	if !ok {
		return false
	}
	return ss.changed
}

// aggregatePriority defines the winner-takes-all aggregation order.
// Any Converging session beats all others; Hung beats BusyButStagnant; etc.
var aggregatePriority = [...]LivenessState{
	LivenessConverging,
	LivenessHung,
	LivenessBusyButStagnant,
	LivenessIdle,
}

// Aggregate returns the single LivenessState for the overall center by applying
// the documented priority rule across all active sessions. Returns 0 when the
// center has no observations (empty, no sessions yet). Aggregate is safe for
// concurrent use alongside Observe and RegisterHandler.
func (sc *SignalCenter) Aggregate() LivenessState {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	seen := make(map[LivenessState]bool, len(sc.sessions))
	for _, ss := range sc.sessions {
		seen[ss.last] = true
	}

	for _, s := range aggregatePriority {
		if seen[s] {
			return s
		}
	}
	return 0
}
