package recipe

import (
	"fmt"
	"regexp"
	"strings"
)

type AwaitKind string

const (
	AwaitPromptMarker AwaitKind = "prompt_marker"
	AwaitRegex        AwaitKind = "regex"
	AwaitAnyOf        AwaitKind = "any_of"
	AwaitAllOf        AwaitKind = "all_of"
)

type Await struct {
	Kind      AwaitKind `json:"kind"`
	Regex     string    `json:"regex,omitempty"`
	Substrs   []string  `json:"substrs,omitempty"`
	FailRegex string    `json:"fail_regex,omitempty"`
	TimeoutS  int       `json:"timeout_s"`
	IntervalS int       `json:"interval_s,omitempty"`
}

type matchOutcome int

const (
	matchPending matchOutcome = iota
	matchSatisfied
	matchFailed
)

type compiledAwait struct {
	kind    AwaitKind
	re      *regexp.Regexp
	failRE  *regexp.Regexp
	substrs []string
}

func (a Await) compile() (compiledAwait, error) {
	c := compiledAwait{kind: a.Kind, substrs: a.Substrs}
	switch a.Kind {
	case AwaitPromptMarker:
	case AwaitRegex:
		if a.Regex == "" {
			return compiledAwait{}, fmt.Errorf("%w: await kind=regex requires a regex", ErrInvalidRecipe)
		}
		re, err := regexp.Compile(a.Regex)
		if err != nil {
			return compiledAwait{}, fmt.Errorf("%w: await regex %q: %v", ErrInvalidRecipe, a.Regex, err)
		}
		c.re = re
	case AwaitAnyOf, AwaitAllOf:
		if len(a.Substrs) == 0 {
			return compiledAwait{}, fmt.Errorf("%w: await kind=%s requires substrs", ErrInvalidRecipe, a.Kind)
		}
	default:
		return compiledAwait{}, fmt.Errorf("%w: unknown await kind %q", ErrInvalidRecipe, a.Kind)
	}
	if a.FailRegex != "" {
		re, err := regexp.Compile(a.FailRegex)
		if err != nil {
			return compiledAwait{}, fmt.Errorf("%w: await fail_regex %q: %v", ErrInvalidRecipe, a.FailRegex, err)
		}
		c.failRE = re
	}
	return c, nil
}

func (c compiledAwait) eval(pane, promptMarker string) matchOutcome {
	if c.failRE != nil && c.failRE.MatchString(pane) {
		return matchFailed
	}
	switch c.kind {
	case AwaitPromptMarker:
		if promptMarker != "" && strings.Contains(pane, promptMarker) {
			return matchSatisfied
		}
	case AwaitRegex:
		if c.re != nil && c.re.MatchString(pane) {
			return matchSatisfied
		}
	case AwaitAnyOf:
		for _, s := range c.substrs {
			if strings.Contains(pane, s) {
				return matchSatisfied
			}
		}
	case AwaitAllOf:
		all := true
		for _, s := range c.substrs {
			if !strings.Contains(pane, s) {
				all = false
				break
			}
		}
		if all {
			return matchSatisfied
		}
	}
	return matchPending
}
