package panestream

import "sync"

// SignalCenter is a Facade that owns per-session liveness signal state
// (ADR-0068). It aggregates all active session signals into one LivenessState,
// and exposes a handler-registration API so that adding a CLI = register a
// strategy + add a profile entry (OCP — no switch edits required).
//
// Concurrency model (S5, measured — see ADR-0068 "Consequences"): a global
// RWMutex (mu) guards ONLY the sessions/registry map STRUCTURE (insert,
// lookup, RegisterHandler); it is never held across the stateful, per-CLI
// probe.Assess() call. Each sessionSignals carries its OWN sync.Mutex guarding
// its probe/last/busy/clean/changed fields — this is what lets independent
// sessions' Observe calls run without funneling through one lock.
//
// Lock ordering (invariant, enforced by code shape, not just convention): the
// global structural lock is always acquired AND FULLY RELEASED before any
// per-session lock is acquired. No code path holds both locks at once, and no
// code path acquires a per-session lock before the global lock.
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
//
// mu is this session's OWN lock (S5): it — not SignalCenter.mu — owns
// probe/last/busy/clean/changed. Every read and every write of those fields
// goes through mu, so Observe (writer) and Aggregate/Busy/Changed (readers)
// can never observe a torn update.
type sessionSignals struct {
	mu      sync.Mutex
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
//
// S5: the global lock (sc.mu) is held ONLY for the map lookup/insert below —
// never across probe.Assess(), which runs under the session's OWN lock
// (ss.mu). This is what lets Observe calls on distinct session keys proceed
// without serializing on one process-global mutex (ADR-0068, measured).
func (sc *SignalCenter) Observe(sessionKey, rendered string, profile PaneProfile) {
	sc.mu.Lock()
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
	sc.mu.Unlock()

	ss.mu.Lock()
	defer ss.mu.Unlock()

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
//
// S5: the global lock guards only the map lookup; ss.busy itself is read
// under ss.mu — the SAME lock Observe writes it under — so no torn read is
// possible.
func (sc *SignalCenter) Busy(sessionKey string) bool {
	sc.mu.RLock()
	ss, ok := sc.sessions[sessionKey]
	sc.mu.RUnlock()
	if !ok {
		return false
	}
	ss.mu.Lock()
	defer ss.mu.Unlock()
	return ss.busy
}

// Changed reports whether the most-recent Observe's cleaned content (chrome
// stripped) differs from the prior Observe's for sessionKey. A key with fewer
// than two observations, or that was never observed, reads false. Safe for
// concurrent use alongside Observe.
//
// S5: same pattern as Busy — the global lock guards only the map lookup;
// ss.changed is read under ss.mu, the SAME lock Observe writes it under.
func (sc *SignalCenter) Changed(sessionKey string) bool {
	sc.mu.RLock()
	ss, ok := sc.sessions[sessionKey]
	sc.mu.RUnlock()
	if !ok {
		return false
	}
	ss.mu.Lock()
	defer ss.mu.Unlock()
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
//
// S5: the global lock is held only long enough to snapshot the *sessionSignals
// pointers (map structure) — it is released BEFORE any per-session lock is
// taken (lock ordering invariant). Each ss.last is then read under that
// session's own lock, the SAME lock Observe writes it under, so no torn read
// is possible.
func (sc *SignalCenter) Aggregate() LivenessState {
	sc.mu.RLock()
	snapshot := make([]*sessionSignals, 0, len(sc.sessions))
	for _, ss := range sc.sessions {
		snapshot = append(snapshot, ss)
	}
	sc.mu.RUnlock()

	seen := make(map[LivenessState]bool, len(snapshot))
	for _, ss := range snapshot {
		ss.mu.Lock()
		seen[ss.last] = true
		ss.mu.Unlock()
	}

	for _, s := range aggregatePriority {
		if seen[s] {
			return s
		}
	}
	return 0
}
