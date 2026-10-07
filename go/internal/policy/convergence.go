package policy

import (
	"cmp"
	"fmt"
	"slices"
)

const (
	defaultConvergenceStage            = "shadow"
	defaultConvergenceMaxFixRounds     = 3
	defaultConvergenceBaseBar          = "MEDIUM"
	defaultConvergenceRaisedBar        = "HIGH"
	defaultConvergenceThreshold        = 0.6
	defaultConvergenceWindow           = 2
	defaultConvergenceMinFindings      = 5
	defaultConvergenceMaxBackwardEdges = 3
	convergenceKeyPrefix               = "workflow.convergence."
)

var (
	convergenceStages = []string{"shadow", "enforce"}
	convergenceBars   = []string{"CRITICAL", "HIGH", "MEDIUM"}
)

type ConvergencePolicy struct {
	Stage                    string   `json:"stage,omitempty"`
	MaxFixRounds             *int     `json:"max_fix_rounds,omitempty"`
	BaseBlockingBar          string   `json:"base_blocking_bar,omitempty"`
	RaisedBlockingBar        string   `json:"raised_blocking_bar,omitempty"`
	ConcentrationThreshold   *float64 `json:"concentration_threshold,omitempty"`
	ConcentrationWindow      *int     `json:"concentration_window,omitempty"`
	ConcentrationMinFindings *int     `json:"concentration_min_findings,omitempty"`
	MaxBackwardEdges         *int     `json:"max_backward_edges,omitempty"`
}

type ConvergenceConfig struct {
	Stage                    string
	MaxFixRounds             int
	BaseBlockingBar          string
	RaisedBlockingBar        string
	ConcentrationThreshold   float64
	ConcentrationWindow      int
	ConcentrationMinFindings int
	MaxBackwardEdges         int
	Warnings                 []string
}

func (b *ConvergencePolicy) UnmarshalJSON(raw []byte) error {
	type block ConvergencePolicy
	var decoded block
	if err := decodeStrict(raw, &decoded); err != nil {
		return fmt.Errorf("convergence: %w", err)
	}
	parsed := ConvergencePolicy(decoded)
	if err := parsed.validate(); err != nil {
		return fmt.Errorf("convergence: %w", err)
	}
	*b = parsed
	return nil
}

func (b ConvergencePolicy) validate() error {
	return b.resolve().Validate()
}

func (c ConvergenceConfig) Validate() error {
	for _, count := range []struct {
		key   string
		value int
	}{
		{"max_fix_rounds", c.MaxFixRounds},
		{"concentration_window", c.ConcentrationWindow},
		{"concentration_min_findings", c.ConcentrationMinFindings},
		{"max_backward_edges", c.MaxBackwardEdges},
	} {
		if count.value < 1 {
			return fmt.Errorf("%s must be at least 1, got %d", count.key, count.value)
		}
	}
	if t := c.ConcentrationThreshold; t <= 0 || t > 1 {
		return fmt.Errorf("concentration_threshold must be above 0 and at most 1, got %v", t)
	}
	for _, word := range []convergenceWord{
		{key: "stage", named: c.Stage, known: convergenceStages},
		{key: "base_blocking_bar", named: c.BaseBlockingBar, known: convergenceBars},
		{key: "raised_blocking_bar", named: c.RaisedBlockingBar, known: convergenceBars},
	} {
		if !slices.Contains(word.known, word.named) {
			return fmt.Errorf("%s %q is not one of %v", word.key, word.named, word.known)
		}
	}
	return nil
}

func defaultConvergence() ConvergenceConfig {
	return ConvergenceConfig{
		Stage:                    defaultConvergenceStage,
		MaxFixRounds:             defaultConvergenceMaxFixRounds,
		BaseBlockingBar:          defaultConvergenceBaseBar,
		RaisedBlockingBar:        defaultConvergenceRaisedBar,
		ConcentrationThreshold:   defaultConvergenceThreshold,
		ConcentrationWindow:      defaultConvergenceWindow,
		ConcentrationMinFindings: defaultConvergenceMinFindings,
		MaxBackwardEdges:         defaultConvergenceMaxBackwardEdges,
	}
}

func (p Policy) ConvergenceConfig() ConvergenceConfig {
	if p.Workflow == nil || p.Workflow.Convergence == nil {
		return defaultConvergence()
	}
	return p.Workflow.Convergence.resolve()
}

func (b ConvergencePolicy) resolve() ConvergenceConfig {
	c := defaultConvergence()
	c.Stage = c.resolveWord(convergenceWord{key: "stage", named: b.Stage, known: convergenceStages, fallback: c.Stage})
	c.BaseBlockingBar = c.resolveWord(convergenceWord{key: "base_blocking_bar", named: b.BaseBlockingBar, known: convergenceBars, fallback: c.BaseBlockingBar})
	c.RaisedBlockingBar = c.resolveWord(convergenceWord{key: "raised_blocking_bar", named: b.RaisedBlockingBar, known: convergenceBars, fallback: c.RaisedBlockingBar})
	c.holdRaisedBarAtOrAboveBase()
	c.MaxFixRounds = valueOr(b.MaxFixRounds, c.MaxFixRounds)
	c.ConcentrationThreshold = valueOr(b.ConcentrationThreshold, c.ConcentrationThreshold)
	c.ConcentrationWindow = valueOr(b.ConcentrationWindow, c.ConcentrationWindow)
	c.ConcentrationMinFindings = valueOr(b.ConcentrationMinFindings, c.ConcentrationMinFindings)
	c.MaxBackwardEdges = valueOr(b.MaxBackwardEdges, c.MaxBackwardEdges)
	return c
}

type convergenceWord struct {
	key      string
	named    string
	known    []string
	fallback string
}

func (c *ConvergenceConfig) resolveWord(w convergenceWord) string {
	if w.named == "" || slices.Contains(w.known, w.named) {
		return cmp.Or(w.named, w.fallback)
	}
	c.Warnings = append(c.Warnings, fmt.Sprintf("%s%s: unknown value %q, falling back to %q", convergenceKeyPrefix, w.key, w.named, w.fallback))
	return w.fallback
}

func (c *ConvergenceConfig) holdRaisedBarAtOrAboveBase() {
	if slices.Index(convergenceBars, c.RaisedBlockingBar) <= slices.Index(convergenceBars, c.BaseBlockingBar) {
		return
	}
	c.Warnings = append(c.Warnings, fmt.Sprintf("%sraised_blocking_bar: %q is below base_blocking_bar %q, holding %q",
		convergenceKeyPrefix, c.RaisedBlockingBar, c.BaseBlockingBar, c.BaseBlockingBar))
	c.RaisedBlockingBar = c.BaseBlockingBar
}
