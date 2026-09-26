package panestream

import (
	"regexp"
	"strconv"
	"strings"
)

// LivenessState is the structured liveness verdict; the zero value means not set, and the reviewer pauses on it.
type LivenessState int

const (
	// LivenessIdle is a quiet pane: no busy signal and no new content.
	LivenessIdle LivenessState = iota + 1
	// LivenessBusyButStagnant is a busy pane with no new content, still inside the stall threshold.
	LivenessBusyButStagnant
	// LivenessConverging means new stable content appeared this interval; real output is never stuck.
	LivenessConverging
	// LivenessHung is a busy pane with no new content for the stall threshold of consecutive intervals.
	LivenessHung
	// LivenessExhausted means the pane shows the CLI's quota or rate-limit wall; it overrides every other state.
	LivenessExhausted
)

// LivenessProbe is a stateful per-run detector; call Assess once per review interval.
type LivenessProbe interface {
	Assess(rendered string, profile PaneProfile) (LivenessState, float64)
}

// ExhaustionProbe decorates a LivenessProbe so a match of profile.ExhaustedRegex overrides its verdict.
// See ADR-0070.
type ExhaustionProbe struct {
	inner LivenessProbe
}

// NewExhaustionProbe wraps inner with exhaustion-override detection.
func NewExhaustionProbe(inner LivenessProbe) *ExhaustionProbe {
	return &ExhaustionProbe{inner: inner}
}

// Assess returns (LivenessExhausted, 1.0) when the pane shows the wall, else the inner verdict unchanged.
func (e *ExhaustionProbe) Assess(rendered string, profile PaneProfile) (LivenessState, float64) {
	state, conf := e.inner.Assess(rendered, profile) // always advance the inner cursor
	if matchExhaustedPattern(profile.ExhaustedRegex, rendered) {
		return LivenessExhausted, 1.0
	}
	return state, conf
}

// matchExhaustedPattern is shared by ExhaustionProbe and ExhaustedOf so the checkpoint and fast-poll
// paths cannot disagree. An empty or invalid pattern matches nothing (fail-open).
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

// defaultHungAfter must stay below the bridge's defaultArtifactMaxExtends so Hung fast-fails
// before the extend backstop.
const defaultHungAfter = 3

// DefaultDetector derives liveness from stable-content growth per interval, so it needs no busy affordance.
type DefaultDetector struct {
	delta     PaneDelta
	hungAfter int // consecutive busy-stagnant intervals before LivenessHung
	stalls    int
}

// NewDefaultDetector returns a detector that reports Hung after stallThreshold busy-stagnant intervals; ≤0 uses 3.
func NewDefaultDetector(stallThreshold int) *DefaultDetector {
	if stallThreshold <= 0 {
		stallThreshold = defaultHungAfter
	}
	return &DefaultDetector{hungAfter: stallThreshold}
}

// Assess classifies one snapshot: Converging (0.9), Hung (0.8), BusyButStagnant (0.6) or Idle (0.7).
func (d *DefaultDetector) Assess(rendered string, p PaneProfile) (LivenessState, float64) {
	newLines := d.delta.Next(rendered, p)
	if len(newLines) > 0 {
		d.stalls = 0
		return LivenessConverging, 0.9
	}
	busy := PaneBusy(rendered, p)
	if !busy {
		// A quiet frame breaks the busy-stagnant run toward Hung.
		d.stalls = 0
		return LivenessIdle, 0.7
	}
	d.stalls++
	if d.stalls >= d.hungAfter {
		return LivenessHung, 0.8
	}
	return LivenessBusyButStagnant, 0.6
}

var rxResponseTokens = regexp.MustCompile(`↓\s*([0-9]+(?:\.[0-9]+)?)(k?)\s+tokens`)

// ExtractResponseTokens returns the peak ↓ response-token count in a rendered pane, scaling a k suffix by 1000.
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

// ClaudeDetector layers a rising ↓ response-token counter over DefaultDetector as a stronger Converging signal.
type ClaudeDetector struct {
	base       *DefaultDetector
	lastTokens int
	primed     bool
}

// NewClaudeDetector returns a ClaudeDetector; stallThreshold is forwarded to its DefaultDetector.
func NewClaudeDetector(stallThreshold int) *ClaudeDetector {
	return &ClaudeDetector{base: NewDefaultDetector(stallThreshold)}
}

// Assess returns (LivenessConverging, 0.95) when the token counter rose since the last call; the first call only primes.
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

// ollama's thinking signal is live while the header line is present and the done marker is absent.
const (
	ollamaThinkingHeader = "Thinking..."
	ollamaThinkingDone   = "...done thinking."
)

// containsOllamaThinkingSignal checks the done marker first because the pane accumulates: the header
// stays visible after thinking ends.
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

// OllamaDetector layers ollama's "Thinking..." header over DefaultDetector as a stronger Converging signal.
type OllamaDetector struct {
	base   *DefaultDetector
	primed bool
}

// NewOllamaDetector returns an OllamaDetector; stallThreshold is forwarded to its DefaultDetector.
func NewOllamaDetector(stallThreshold int) *OllamaDetector {
	return &OllamaDetector{base: NewDefaultDetector(stallThreshold)}
}

// Assess returns (LivenessConverging, 0.92) while the thinking header is live; the first call only primes.
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

// agyGeneratingSpinner is the line agy renders while generating; it vanishes in the answer frame.
const agyGeneratingSpinner = "⣯ Generating..."

func containsAgyGeneratingSignal(rendered string) bool {
	for _, line := range strings.Split(rendered, "\n") {
		if strings.TrimSpace(line) == agyGeneratingSpinner {
			return true
		}
	}
	return false
}

// AgyDetector layers agy's "⣯ Generating..." spinner over DefaultDetector as a stronger Converging signal.
type AgyDetector struct {
	base   *DefaultDetector
	primed bool
}

// NewAgyDetector returns an AgyDetector; stallThreshold is forwarded to its DefaultDetector.
func NewAgyDetector(stallThreshold int) *AgyDetector {
	return &AgyDetector{base: NewDefaultDetector(stallThreshold)}
}

// Assess returns (LivenessConverging, 0.92) while the generating spinner shows; the first call only primes.
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

// DetectorFor returns a new LivenessProbe for the profile's CLI; unknown CLIs get a DefaultDetector.
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

// String returns the state's one spelling, the word pane.liveness signals carry in fields.state.
func (s LivenessState) String() string {
	switch s {
	case LivenessIdle:
		return "idle"
	case LivenessBusyButStagnant:
		return "busy-stagnant"
	case LivenessConverging:
		return "converging"
	case LivenessHung:
		return "hung"
	case LivenessExhausted:
		return "exhausted"
	}
	return "unknown"
}
