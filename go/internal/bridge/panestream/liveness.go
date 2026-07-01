package panestream

import (
	"regexp"
	"strconv"
	"strings"
)

// liveness.go — Strategy-based liveness detection over a sequence of rendered
// pane snapshots (ADR-0047 projection seam). All detectors are projections of
// ClassifyLine / PaneDelta — the single channel separator — so chrome/affordance
// vocabulary cannot drift between liveness and progress paths.
//
// LivenessProbe is the per-run interface. DefaultDetector uses stable-content
// growth velocity (new content lines / interval via PaneDelta), closing the
// codex weak-signal gap (PaneBusy=false on codex → Idle or Converging from
// growth alone). ClaudeDetector composes over DefaultDetector, layering a
// monotonic ↓ token counter as higher-confidence Converging for claude.
//
// Registry: DetectorFor(profile) is co-located with Profiles (ADR-0047
// single-source-with-projection); the only per-CLI branch is here, never in
// the reviewer (stopreview.go).

// LivenessState is the structured liveness vocabulary replacing the coarse
// Progressed+Busy boolean pair. Zero value (0) means "not set" — the reviewer
// falls back to the legacy boolean path when StopEvent.State == 0.
type LivenessState int

const (
	// LivenessIdle: quiet prompt, no busy signal, no new content. Reviewer: pause.
	LivenessIdle LivenessState = iota + 1
	// LivenessBusyButStagnant: busy affordance present, no new content for fewer
	// than stallThreshold intervals. Reviewer: extend (bounded by maxExtends).
	LivenessBusyButStagnant
	// LivenessConverging: new stable content lines emitted this interval (growth
	// velocity > 0). Reviewer: extend unconditionally — real output is never stuck.
	LivenessConverging
	// LivenessHung: busy but no new content for stallThreshold consecutive
	// intervals. Reviewer: fast-fail before the maxExtends×interval backstop.
	LivenessHung
	// LivenessExhausted: the pane shows a quota/rate-limit WALL (the per-CLI
	// ExhaustedRegex matched). This is an ORTHOGONAL axis to the growth-velocity
	// liveness above — a walled CLI can still look Converging because its error
	// re-prints as "new content" — so ExhaustionProbe returns this state to
	// OVERRIDE the inner verdict. It is terminal: the artifact will never come,
	// so the reviewer/driver must fast-fail to the fallback CLI (exit 85), never
	// extend. Top of the aggregate priority order (a wall dominates).
	LivenessExhausted
)

// LivenessProbe is the per-run liveness detector interface. Implementations are
// stateful across calls (they hold PaneDelta cursors and stall counters). Call
// Assess once per review interval with the current rendered pane snapshot.
type LivenessProbe interface {
	Assess(rendered string, profile PaneProfile) (LivenessState, float64)
}

// ExhaustionProbe is a Decorator over any LivenessProbe that layers the
// orthogonal, DOMINATING exhaustion signal: when the rendered pane matches the
// profile's ExhaustedRegex (the per-CLI quota/rate-limit wall), Assess returns
// LivenessExhausted regardless of the inner liveness verdict. This is what stops
// a re-printed quota error from reading as LivenessConverging ("real output is
// never stuck") and wedging the phase in an extend-forever livelock. The inner
// probe is ALWAYS assessed (advancing its stateful PaneDelta cursor) so its
// verdict stays consistent for any later frame; the override only replaces the
// RESULT. When ExhaustedRegex is empty, uncompilable, or unmatched the decorator
// is transparent — it delegates byte-identically (fail-open: never invents a
// wall). This keeps exhaustion detection single-source (the manifest pattern),
// per-CLI (via PaneProfile), and applied uniformly to every registered strategy.
type ExhaustionProbe struct {
	inner LivenessProbe
}

// NewExhaustionProbe wraps inner with exhaustion-override detection. The
// SignalCenter wraps every per-CLI probe in one of these so exhaustion flows
// through the same abstraction as liveness.
func NewExhaustionProbe(inner LivenessProbe) *ExhaustionProbe {
	return &ExhaustionProbe{inner: inner}
}

// Assess returns (LivenessExhausted, 1.0) when the pane matches
// profile.ExhaustedRegex (a wall is unambiguous — confidence 1.0); otherwise the
// inner probe's verdict, unchanged.
func (e *ExhaustionProbe) Assess(rendered string, profile PaneProfile) (LivenessState, float64) {
	state, conf := e.inner.Assess(rendered, profile) // always advance the inner cursor
	if matchExhaustedPattern(profile.ExhaustedRegex, rendered) {
		return LivenessExhausted, 1.0
	}
	return state, conf
}

// matchExhaustedPattern reports whether rendered shows the quota/rate-limit wall
// described by pattern — the shared matcher for the two panestream exhaustion
// paths (ExhaustionProbe's 300s-checkpoint Observe and SignalCenter.ExhaustedOf's
// ~2s fast poll), so they can never disagree. (bridge/usageclassify.go's
// matchExhausted is its cross-layer sibling — the same predicate, kept separate
// only by the one-way bridge→panestream import boundary; a future modularization
// could host one copy in a shared leaf.) It compiles per call rather than caching
// a *regexp.Regexp: the patterns are small and the per-poll CapturePane IPC
// dominates by orders of magnitude, so a cache is not worth the state. An empty
// or uncompilable pattern matches nothing — fail-open: the gate's own
// misconfiguration must never brick a session.
func matchExhaustedPattern(pattern, rendered string) bool {
	if pattern == "" {
		return false
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}
	return re.MatchString(rendered)
}

// defaultHungAfter is how many consecutive busy-but-stagnant intervals the
// DefaultDetector accumulates before declaring LivenessHung. Must be less than
// defaultArtifactMaxExtends (6 in stopreview.go) so Hung fast-fails BEFORE the
// maxExtends×interval backstop.
const defaultHungAfter = 3

// DefaultDetector derives liveness from stable-content growth velocity: new
// stable content lines per interval (via PaneDelta / ClassifyLine). Works for
// any CLI including codex which has no busy affordance — the content-line
// count alone classifies Converging vs Hung without needing PaneBusy.
type DefaultDetector struct {
	delta     PaneDelta
	hungAfter int // consecutive busy-stagnant intervals before LivenessHung
	stalls    int // current consecutive busy-stagnant interval count
}

// NewDefaultDetector creates a DefaultDetector. stallThreshold controls how many
// consecutive busy-but-no-content intervals trigger LivenessHung; ≤0 uses
// defaultHungAfter (3).
func NewDefaultDetector(stallThreshold int) *DefaultDetector {
	if stallThreshold <= 0 {
		stallThreshold = defaultHungAfter
	}
	return &DefaultDetector{hungAfter: stallThreshold}
}

// Assess evaluates one rendered pane snapshot. Returns (LivenessConverging, 0.9)
// when new stable content lines appeared; (LivenessHung, 0.8) after hungAfter
// consecutive busy-stagnant intervals; (LivenessBusyButStagnant, 0.6) for busy
// panes within the stall budget; (LivenessIdle, 0.7) for quiet panes.
func (d *DefaultDetector) Assess(rendered string, p PaneProfile) (LivenessState, float64) {
	newLines := d.delta.Next(rendered, p)
	if len(newLines) > 0 {
		d.stalls = 0
		return LivenessConverging, 0.9
	}
	busy := PaneBusy(rendered, p)
	if !busy {
		// Idle resets the stall counter — a non-busy quiet frame is not a
		// busy-stagnant accumulation toward Hung.
		d.stalls = 0
		return LivenessIdle, 0.7
	}
	d.stalls++
	if d.stalls >= d.hungAfter {
		return LivenessHung, 0.8
	}
	return LivenessBusyButStagnant, 0.6
}

// rxResponseTokens extracts the peak ↓ response-token count from a rendered
// pane. Handles both "↓ Nk tokens" (k-scaled) and "↓ N tokens" (plain integer)
// so test frames and real capture frames match uniformly.
var rxResponseTokens = regexp.MustCompile(`↓\s*([0-9]+(?:\.[0-9]+)?)(k?)\s+tokens`)

// ExtractResponseTokens returns the peak ↓ response-token count from a rendered
// pane. It is the single-source token extractor for the signal-center campaign
// (S1, ADR-0047): both ClaudeDetector and the stopreview callsite use this
// instead of maintaining separate per-package extractors.
func ExtractResponseTokens(pane string) int {
	peak := 0
	for _, m := range rxResponseTokens.FindAllStringSubmatch(pane, -1) {
		if len(m) < 3 {
			continue
		}
		v, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			continue
		}
		var n int
		if m[2] == "k" {
			n = int(v*1000 + 0.5)
		} else {
			n = int(v + 0.5)
		}
		if n > peak {
			peak = n
		}
	}
	return peak
}

// ClaudeDetector composes over DefaultDetector, layering the monotonic ↓
// response-token counter as a higher-confidence Converging signal for claude.
// A strictly-increasing ↓ N tokens line across intervals confirms the model is
// still generating, even when content lines are being streamed as chrome.
// When the counter is absent or static, it falls back to the DefaultDetector
// verdict — byte-identical on non-claude frames. Non-claude CLIs receive
// DefaultDetector via DetectorFor.
type ClaudeDetector struct {
	base       *DefaultDetector
	lastTokens int  // highest ↓ token count seen so far; 0 before first non-prime call
	primed     bool // true after the first Assess call (prime / baseline)
}

// NewClaudeDetector creates a ClaudeDetector. stallThreshold is forwarded to
// the inner DefaultDetector; ≤0 uses defaultHungAfter.
func NewClaudeDetector(stallThreshold int) *ClaudeDetector {
	return &ClaudeDetector{base: NewDefaultDetector(stallThreshold)}
}

// Assess evaluates one rendered pane snapshot. On the prime (first) call,
// it always returns the base DefaultDetector verdict (the baseline is being
// established, not compared). On subsequent calls, a strictly-increasing ↓
// token counter overrides to (LivenessConverging, 0.95) — higher confidence
// than the default's growth-velocity path. Static or absent counters fall
// through to the base verdict unchanged.
func (c *ClaudeDetector) Assess(rendered string, p PaneProfile) (LivenessState, float64) {
	base, baseConf := c.base.Assess(rendered, p)
	tokens := ExtractResponseTokens(rendered)
	if !c.primed {
		c.primed = true
		c.lastTokens = tokens
		return base, baseConf
	}
	if tokens > c.lastTokens {
		c.lastTokens = tokens
		return LivenessConverging, 0.95
	}
	return base, baseConf
}

// ollamaThinkingHeader is the exact line text that ollama renders while its
// chain-of-thought model is streaming. ollamaThinkingDone is the terminator
// line that appears when the thinking phase is complete. The signal is active
// only while the header is present AND the done-marker is absent.
const (
	ollamaThinkingHeader = "Thinking..."
	ollamaThinkingDone   = "...done thinking."
)

// containsOllamaThinkingSignal returns true when the rendered pane contains
// the ollama thinking header as a complete trimmed line AND does NOT yet
// contain the thinking-done terminator. The pane accumulates cumulatively —
// after the thinking phase completes, "Thinking..." stays visible but
// "...done thinking." appears, ending the active-generation signal.
func containsOllamaThinkingSignal(rendered string) bool {
	if strings.Contains(rendered, ollamaThinkingDone) {
		return false
	}
	for _, line := range strings.Split(rendered, "\n") {
		if strings.TrimSpace(line) == ollamaThinkingHeader {
			return true
		}
	}
	return false
}

// OllamaDetector composes over DefaultDetector, layering ollama's "Thinking..."
// header as a higher-confidence Converging signal. When the header is present as
// a complete line, the model is streaming its chain-of-thought: the detector
// returns (LivenessConverging, 0.92) even when no new stable content lines
// appeared in the interval (a frame DefaultDetector would classify as
// BusyButStagnant). When the header is absent, falls back to DefaultDetector
// byte-identical. Non-ollama CLIs receive DefaultDetector via DetectorFor.
type OllamaDetector struct {
	base   *DefaultDetector
	primed bool
}

// NewOllamaDetector creates an OllamaDetector. stallThreshold is forwarded to
// the inner DefaultDetector; ≤0 uses defaultHungAfter.
func NewOllamaDetector(stallThreshold int) *OllamaDetector {
	return &OllamaDetector{base: NewDefaultDetector(stallThreshold)}
}

// Assess evaluates one rendered pane snapshot. On the prime (first) call it
// always returns the base DefaultDetector verdict (establishing baseline). On
// subsequent calls, presence of the "Thinking..." header as a complete line
// overrides to (LivenessConverging, 0.92) — higher confidence than DefaultDetector's
// BusyButStagnant (0.6) or Hung (0.8), confirming the model is live without
// requiring a content-line delta. Absent/partial header falls through to base.
func (d *OllamaDetector) Assess(rendered string, p PaneProfile) (LivenessState, float64) {
	base, baseConf := d.base.Assess(rendered, p)
	if !d.primed {
		d.primed = true
		return base, baseConf
	}
	if containsOllamaThinkingSignal(rendered) {
		return LivenessConverging, 0.92
	}
	return base, baseConf
}

// agyGeneratingSpinner is the exact spinner text that agy renders while the
// model is generating a response. It disappears when the answer is complete.
const agyGeneratingSpinner = "⣯ Generating..."

// containsAgyGeneratingSignal returns true when the rendered pane contains
// agy's generating spinner as a complete trimmed line. The spinner vanishes in
// the answer frame, so its presence confirms the model is actively generating.
func containsAgyGeneratingSignal(rendered string) bool {
	for _, line := range strings.Split(rendered, "\n") {
		if strings.TrimSpace(line) == agyGeneratingSpinner {
			return true
		}
	}
	return false
}

// AgyDetector composes over DefaultDetector, layering agy's "⣯ Generating..."
// spinner affordance as a higher-confidence Converging signal. When the spinner
// is present, the model is actively generating: returns (LivenessConverging, 0.92)
// even when no new content lines appeared in the interval. When the spinner is
// absent (answer frame), falls back to DefaultDetector byte-identical.
// Non-agy CLIs receive DefaultDetector via DetectorFor.
type AgyDetector struct {
	base   *DefaultDetector
	primed bool
}

// NewAgyDetector creates an AgyDetector. stallThreshold is forwarded to
// the inner DefaultDetector; ≤0 uses defaultHungAfter.
func NewAgyDetector(stallThreshold int) *AgyDetector {
	return &AgyDetector{base: NewDefaultDetector(stallThreshold)}
}

// Assess evaluates one rendered pane snapshot. On the prime (first) call it
// always returns the base DefaultDetector verdict (establishing baseline). On
// subsequent calls, presence of "⣯ Generating..." as a complete line overrides
// to (LivenessConverging, 0.92) — higher confidence than BusyButStagnant (0.6),
// confirming the model is live without requiring a content-line delta. Absent
// spinner falls through to the base DefaultDetector verdict unchanged.
func (d *AgyDetector) Assess(rendered string, p PaneProfile) (LivenessState, float64) {
	base, baseConf := d.base.Assess(rendered, p)
	if !d.primed {
		d.primed = true
		return base, baseConf
	}
	if containsAgyGeneratingSignal(rendered) {
		return LivenessConverging, 0.92
	}
	return base, baseConf
}

// DetectorFor returns a new LivenessProbe for the given pane profile.
// Co-located with Profiles (ADR-0047 single-source-with-projection): the
// per-CLI strategy selection lives here, never in the reviewer.
// claude routes to ClaudeDetector; ollama routes to OllamaDetector;
// agy routes to AgyDetector; all others receive DefaultDetector.
func DetectorFor(p PaneProfile) LivenessProbe {
	switch p.Name {
	case "claude":
		return NewClaudeDetector(0)
	case "ollama":
		return NewOllamaDetector(0)
	case "agy":
		return NewAgyDetector(0)
	default:
		return NewDefaultDetector(0)
	}
}
