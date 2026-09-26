package panestream

import "sync"

// LivenessCenter is the Facade that owns per-session liveness state and aggregates it into one LivenessState.
//
// mu guards only the maps and the handler list, and is always released before a session's own
// lock is taken, so no path holds both.
// See ADR-0068.
type LivenessCenter struct {
	mu       sync.RWMutex
	sessions map[string]*sessionSignals
	registry map[string]func() LivenessProbe
	handlers []LivenessHandler // edge-triggered observers from RegisterLivenessHandler
}

// LivenessEvent is one session's liveness transition, dispatched to registered handlers.
type LivenessEvent struct {
	SessionKey string
	State      LivenessState
}

// LivenessHandler reacts to a liveness transition; it runs inline in Observe, so it must be cheap and non-blocking.
type LivenessHandler func(LivenessEvent)

// sessionSignals.mu, not LivenessCenter.mu, guards every other field, so readers never see a torn update.
type sessionSignals struct {
	mu      sync.Mutex
	probe   LivenessProbe
	last    LivenessState
	busy    bool
	clean   string
	changed bool
}

// NewLivenessCenter returns an empty, ready-to-use LivenessCenter.
func NewLivenessCenter() *LivenessCenter {
	return &LivenessCenter{
		sessions: make(map[string]*sessionSignals),
		registry: make(map[string]func() LivenessProbe),
	}
}

// RegisterHandler sets the probe factory for sessions whose profile Name matches; an empty name is ignored, the last registration wins.
func (sc *LivenessCenter) RegisterHandler(name string, factory func() LivenessProbe) {
	if name == "" {
		return
	}
	sc.mu.Lock()
	sc.registry[name] = factory
	sc.mu.Unlock()
}

// RegisterLivenessHandler subscribes h to every session's liveness transitions; a nil h is ignored.
func (sc *LivenessCenter) RegisterLivenessHandler(h LivenessHandler) {
	if h == nil {
		return
	}
	sc.mu.Lock()
	sc.handlers = append(sc.handlers, h)
	sc.mu.Unlock()
}

// Observe assesses one pane snapshot for sessionKey, creating its probe from the registry or DetectorFor on first use.
func (sc *LivenessCenter) Observe(sessionKey, rendered string, profile PaneProfile) {
	sc.mu.Lock()
	ss, existed := sc.sessions[sessionKey]
	if !existed {
		var probe LivenessProbe
		if f, found := sc.registry[profile.Name]; found {
			probe = f()
		} else {
			probe = DetectorFor(profile)
		}
		ss = &sessionSignals{probe: NewExhaustionProbe(probe)}
		sc.sessions[sessionKey] = ss
	}
	handlers := append([]LivenessHandler(nil), sc.handlers...) // copied so dispatch runs outside the lock
	sc.mu.Unlock()

	ss.mu.Lock()
	prev := ss.last
	state, _ := ss.probe.Assess(rendered, profile)
	ss.last = state

	ss.busy = PaneBusy(rendered, profile)
	clean := cleanPane(rendered)
	ss.changed = existed && clean != ss.clean
	ss.clean = clean
	ss.mu.Unlock()

	// Dispatch only on a transition and outside both locks, so a slow or re-entrant handler cannot
	// serialize other sessions or deadlock. A key's first observation is a transition from 0.
	if state != prev {
		ev := LivenessEvent{SessionKey: sessionKey, State: state}
		for _, h := range handlers {
			h(ev)
		}
	}
}

// Busy reports the busy affordance from sessionKey's latest Observe; an unobserved key reads false.
func (sc *LivenessCenter) Busy(sessionKey string) bool {
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

// Changed reports whether sessionKey's cleaned content differed between its last two Observes; fewer than two reads false.
func (sc *LivenessCenter) Changed(sessionKey string) bool {
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

// BusyOf reports PaneBusy for a pane without touching session state; it is safe on a nil receiver.
func (sc *LivenessCenter) BusyOf(rendered string, profile PaneProfile) bool {
	return PaneBusy(rendered, profile)
}

// ExhaustedOf reports whether the pane shows profile's quota wall, without touching session state; it is safe on a nil receiver.
func (sc *LivenessCenter) ExhaustedOf(rendered string, profile PaneProfile) bool {
	return matchExhaustedPattern(profile.ExhaustedRegex, rendered)
}

// aggregatePriority lists states from most to least dominant.
var aggregatePriority = [...]LivenessState{
	LivenessExhausted,
	LivenessConverging,
	LivenessHung,
	LivenessBusyButStagnant,
	LivenessIdle,
}

// Aggregate returns the most dominant state across all sessions, or 0 when none has been observed.
func (sc *LivenessCenter) Aggregate() LivenessState {
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
