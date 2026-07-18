package bridge

// exhaustion_gate.go — persistence guard for the quota/rate-limit fast-fail.
//
// The exhaustion detectors (the ~2s fast-poll's ExhaustedOf and the 300s
// stop-review checkpoint's Observe) match a regex against the RAW captured
// pane. That pane is not just CLI chrome: an agent doing ordinary work can
// render wall-shaped TEXT into it — a `cat`/`grep`/diff of a file, test
// fixture, or incident report that quotes a provider's "you've reached your …
// limit" message. Fast-failing on that single frame kills a WORKING agent
// (exit 85 → cross-family failover), the cardinal false-FAIL sin
// (cycle-254/255/314/641) the go-review of the per-model regex fix surfaced.
//
// The robust, regex-independent discriminator is PERSISTENCE. A genuine wall is
// the CLI's TERMINAL state — it parks there and the same text is present on the
// next observation too (a re-printing error still shows the wall every frame,
// preserving the agy-hang fix that motivated the override). Wall text merely
// PASSING THROUGH a working agent's pane is gone by the next observation, as the
// agent's fresh output scrolls it off. So: fast-fail only after the wall has
// persisted for `threshold` consecutive observations; a single transient match
// never crosses. This costs one extra observation of latency on a real wall
// (~one fast-poll tick), a trade the fail-over-vs-kill asymmetry makes trivially
// worth it: a missed wall merely fails over (safe); a killed working agent is not.
//
// One gate instance per detection loop (the fast-poll owns one, the checkpoint
// owns one) — they observe at different cadences and must each require their OWN
// consecutive frames, so they never share a streak.
const exhaustionPersistObservations = 2

type exhaustionGate struct {
	threshold int
	streak    int
}

func newExhaustionGate() *exhaustionGate {
	return &exhaustionGate{threshold: exhaustionPersistObservations}
}

// observe records one exhaustion observation and reports whether the wall has
// now persisted for `threshold` consecutive observations (i.e. this call should
// trigger the fast-fail). matched=false resets the streak — the defining
// property that lets a transient wall-text frame in a working agent's pane never
// cross the threshold.
func (g *exhaustionGate) observe(matched bool) bool {
	if !matched {
		g.streak = 0
		return false
	}
	g.streak++
	return g.streak >= g.threshold
}
