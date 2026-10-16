package panestream

import (
	"regexp"
	"strconv"
	"strings"
)

type LivenessState int

const (
	LivenessIdle LivenessState = iota + 1
	LivenessBusyButStagnant
	LivenessConverging
	LivenessHung
	LivenessExhausted
)

type LivenessProbe interface {
	Assess(rendered string, profile PaneProfile) (LivenessState, float64)
}

type ExhaustionProbe struct {
	inner LivenessProbe
}

func NewExhaustionProbe(inner LivenessProbe) *ExhaustionProbe {
	return &ExhaustionProbe{inner: inner}
}

func (e *ExhaustionProbe) Assess(rendered string, profile PaneProfile) (LivenessState, float64) {
	state, conf := e.inner.Assess(rendered, profile)
	if matchExhaustedPattern(profile.ExhaustedRegex, rendered) {
		return LivenessExhausted, 1.0
	}
	return state, conf
}

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

const defaultHungAfter = 3

type DefaultDetector struct {
	delta     PaneDelta
	hungAfter int
	stalls    int
}

func NewDefaultDetector(stallThreshold int) *DefaultDetector {
	if stallThreshold <= 0 {
		stallThreshold = defaultHungAfter
	}
	return &DefaultDetector{hungAfter: stallThreshold}
}

func (d *DefaultDetector) Assess(rendered string, p PaneProfile) (LivenessState, float64) {
	newLines := d.delta.Next(rendered, p)
	if len(newLines) > 0 {
		d.stalls = 0
		return LivenessConverging, 0.9
	}
	busy := PaneBusy(rendered, p)
	if !busy {
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

type ClaudeDetector struct {
	base       *DefaultDetector
	lastTokens int
	primed     bool
}

func NewClaudeDetector(stallThreshold int) *ClaudeDetector {
	return &ClaudeDetector{base: NewDefaultDetector(stallThreshold)}
}

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

const (
	ollamaThinkingHeader = "Thinking..."
	ollamaThinkingDone   = "...done thinking."
)

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

type OllamaDetector struct {
	base   *DefaultDetector
	primed bool
}

func NewOllamaDetector(stallThreshold int) *OllamaDetector {
	return &OllamaDetector{base: NewDefaultDetector(stallThreshold)}
}

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

type AgyDetector struct {
	base      *DefaultDetector
	lastBlock string
	primed    bool
}

func NewAgyDetector(stallThreshold int) *AgyDetector {
	return &AgyDetector{base: NewDefaultDetector(stallThreshold)}
}

func (d *AgyDetector) Assess(rendered string, p PaneProfile) (LivenessState, float64) {
	base, baseConf := d.base.Assess(rendered, p)
	block := LastTokenLine(rendered, p.TokenLineRegex)
	if !d.primed {
		d.primed, d.lastBlock = true, block
		return base, baseConf
	}
	if block == "" || block == d.lastBlock {
		return base, baseConf
	}
	d.lastBlock = block
	return LivenessConverging, 0.95
}

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
