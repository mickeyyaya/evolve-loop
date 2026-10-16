package panestream

import "sync"

type LivenessCenter struct {
	mu       sync.RWMutex
	sessions map[string]*sessionSignals
	registry map[string]func() LivenessProbe
	handlers []LivenessHandler
}

type LivenessEvent struct {
	SessionKey string
	State      LivenessState
}

type LivenessHandler func(LivenessEvent)

type sessionSignals struct {
	mu      sync.Mutex
	probe   LivenessProbe
	last    LivenessState
	busy    bool
	clean   string
	changed bool
}

func NewLivenessCenter() *LivenessCenter {
	return &LivenessCenter{
		sessions: make(map[string]*sessionSignals),
		registry: make(map[string]func() LivenessProbe),
	}
}

func (sc *LivenessCenter) RegisterHandler(name string, factory func() LivenessProbe) {
	if name == "" {
		return
	}
	sc.mu.Lock()
	sc.registry[name] = factory
	sc.mu.Unlock()
}

func (sc *LivenessCenter) RegisterLivenessHandler(h LivenessHandler) {
	if h == nil {
		return
	}
	sc.mu.Lock()
	sc.handlers = append(sc.handlers, h)
	sc.mu.Unlock()
}

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
	handlers := append([]LivenessHandler(nil), sc.handlers...)
	sc.mu.Unlock()

	ss.mu.Lock()
	prev := ss.last
	state, _ := ss.probe.Assess(rendered, profile)
	ss.last = state

	ss.busy = PaneBusy(rendered, profile)
	clean := cleanPaneFor(rendered, profile)
	ss.changed = existed && clean != ss.clean
	ss.clean = clean
	ss.mu.Unlock()

	if state != prev {
		ev := LivenessEvent{SessionKey: sessionKey, State: state}
		for _, h := range handlers {
			h(ev)
		}
	}
}

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

func (sc *LivenessCenter) BusyOf(rendered string, profile PaneProfile) bool {
	return PaneBusy(rendered, profile)
}

func (sc *LivenessCenter) ExhaustedOf(rendered string, profile PaneProfile) bool {
	return matchExhaustedPattern(profile.ExhaustedRegex, rendered)
}

var aggregatePriority = [...]LivenessState{
	LivenessExhausted,
	LivenessConverging,
	LivenessHung,
	LivenessBusyButStagnant,
	LivenessIdle,
}

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
