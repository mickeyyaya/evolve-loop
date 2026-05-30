package recipe

import (
	"fmt"
	"regexp"
	"strings"
)

// AwaitKind is the Strategy selector for how a step decides its work is done
// by inspecting the captured pane.
type AwaitKind string

const (
	// AwaitPromptMarker is satisfied when the REPL prompt marker reappears
	// (the CLI returned to idle). The marker is supplied by the engine from
	// the manifest, so an empty marker can never accidentally satisfy.
	AwaitPromptMarker AwaitKind = "prompt_marker"
	// AwaitRegex is satisfied when Regex matches the pane.
	AwaitRegex AwaitKind = "regex"
	// AwaitAnyOf is satisfied when the pane contains any of Substrs.
	AwaitAnyOf AwaitKind = "any_of"
	// AwaitAllOf is satisfied when the pane contains all of Substrs.
	AwaitAllOf AwaitKind = "all_of"
)

// Await is the declarative wait condition for a step. FailRegex, when set, is
// an early-fail signal checked before the success condition on every tick.
type Await struct {
	Kind      AwaitKind `json:"kind"`
	Regex     string    `json:"regex,omitempty"`
	Substrs   []string  `json:"substrs,omitempty"`
	FailRegex string    `json:"fail_regex,omitempty"`
	TimeoutS  int       `json:"timeout_s"`
	IntervalS int       `json:"interval_s,omitempty"`
}

// matchOutcome is the per-tick evaluation result.
type matchOutcome int

const (
	matchPending   matchOutcome = iota // keep polling
	matchSatisfied                     // advance to next step
	matchFailed                        // fail_regex tripped — abort step
)

// compiledAwait is an Await with its regexes pre-compiled. Splitting compile
// (which can error) from eval (pure, total) keeps the per-tick hot path free
// of recompilation and lets validate() reject bad regexes at load time.
type compiledAwait struct {
	kind    AwaitKind
	re      *regexp.Regexp // non-nil only for AwaitRegex
	failRE  *regexp.Regexp // non-nil only when FailRegex set
	substrs []string
}

// compile validates the Await and pre-compiles its regexes.
func (a Await) compile() (compiledAwait, error) {
	c := compiledAwait{kind: a.Kind, substrs: a.Substrs}
	switch a.Kind {
	case AwaitPromptMarker:
		// no extra fields required
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

// eval is the pure, total per-tick decision. fail_regex is checked first so a
// definitive failure beats a coincidental success-substring on the same pane.
func (c compiledAwait) eval(pane, promptMarker string) matchOutcome {
	if c.failRE != nil && c.failRE.MatchString(pane) {
		return matchFailed
	}
	switch c.kind {
	case AwaitPromptMarker:
		// An empty marker (manifest miss) can never confirm idle — stay
		// pending rather than satisfy on strings.Contains(pane, "")==true.
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
