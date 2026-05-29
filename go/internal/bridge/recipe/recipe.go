// Package recipe drives scripted, multi-step interactive slash-command
// sequences through an LLM CLI's tmux REPL — the canonical example being a
// plugin install (`/plugin marketplace add` → `/plugin install` →
// `/reload-plugins`), where each step must wait for a pane condition before
// the next is sent.
//
// The package is deliberately self-contained: it OWNS the small ports it
// depends on (SessionDriver, Clock — see executor.go) rather than importing
// the parent bridge package. This keeps the dependency arrow one-directional
// (bridge → recipe), breaks the import cycle, and makes the engine a pure
// state machine over (pane-snapshots-in, key-tokens-out) that is trivially
// unit-testable with a scripted fake driver.
//
// Design patterns: Repository (LoadRecipe, loader.go), Template Method
// (Engine.Run, executor.go), Strategy (Await kinds, condition.go; Send.Kind),
// Command (each Step is a self-describing unit of work).
package recipe

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Sentinel errors. Callers use errors.Is to branch; the CLI surface maps
// them to exit codes.
var (
	// ErrUnsupportedCLI is returned when a recipe has no step list for the
	// requested CLI (e.g. plugin-install against ollama, which has no plugin
	// system). Failing loudly beats a silent no-op.
	ErrUnsupportedCLI = errors.New("recipe: no steps for cli")
	// ErrMissingParam is returned when a required parameter has neither a
	// caller-supplied value nor a default.
	ErrMissingParam = errors.New("recipe: missing required parameter")
	// ErrUnknownParam is returned when a step body references a {{token}}
	// that is not among the merged parameters — substitution refuses to emit
	// a body with an empty or stray placeholder.
	ErrUnknownParam = errors.New("recipe: unresolved parameter token")
	// ErrStepTimeout is returned when a step's await condition never matched
	// within its timeout and OnTimeout is abort (the default).
	ErrStepTimeout = errors.New("recipe: step await timed out")
	// ErrAwaitFailRegex is returned when a step's fail_regex matched the pane
	// — an early, definitive failure signal (e.g. "not found").
	ErrAwaitFailRegex = errors.New("recipe: step fail_regex matched")
	// ErrAutoRespondEscalation is returned when the auto-responder escalated
	// (policy=escalate or loop-guard tripped) while waiting between steps.
	ErrAutoRespondEscalation = errors.New("recipe: auto-respond escalated")
	// ErrInvalidRecipe is returned by validate() for a malformed recipe.
	ErrInvalidRecipe = errors.New("recipe: invalid")
)

// SendKind selects the INJECT transport for a step's body.
type SendKind string

const (
	// KindCommand pastes the body via the paste-buffer then presses Enter —
	// the idiom for slash commands and multi-line text (survives newlines).
	KindCommand SendKind = "command"
	// KindKeys sends the body as raw tmux key tokens (e.g. "Down Down Enter")
	// with no trailing Enter — for menu navigation and modal control.
	KindKeys SendKind = "keys"
)

// OnTimeout decides what happens when a step's await condition times out.
type OnTimeout string

const (
	// OnTimeoutAbort (default, also the zero value via abortIfEmpty) fails the
	// whole recipe when a step times out.
	OnTimeoutAbort OnTimeout = "abort"
	// OnTimeoutContinue records the timeout and proceeds to the next step —
	// for best-effort steps where a missing confirmation is non-fatal.
	OnTimeoutContinue OnTimeout = "continue"
)

// ParamDecl declares one recipe parameter.
type ParamDecl struct {
	Name     string `json:"name"`
	Required bool   `json:"required"`
	Default  string `json:"default,omitempty"`
}

// Send is what a step injects into the REPL.
type Send struct {
	Kind SendKind `json:"kind"`
	Body string   `json:"body"`
}

// Step is one command-and-wait unit of a recipe.
type Step struct {
	Name      string    `json:"name"`
	Send      Send      `json:"send"`
	Await     Await     `json:"await"`
	OnTimeout OnTimeout `json:"on_timeout,omitempty"`
}

// Recipe is a declarative interaction script. A recipe is either CLI-agnostic
// (Steps) or per-CLI (PerCLI) — the latter wins when an arm exists for the
// target CLI, modelling flows whose mechanics differ per CLI (Claude's
// `/plugin ...` one-liners vs Codex's menu-driven `/plugins` TUI).
type Recipe struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Params      []ParamDecl       `json:"params,omitempty"`
	Steps       []Step            `json:"steps,omitempty"`
	PerCLI      map[string][]Step `json:"per_cli,omitempty"`
}

// Params is the resolved name→value map a recipe is executed with.
type Params map[string]string

// stepsFor resolves the step list for cli: the per-CLI arm if present, else
// the CLI-agnostic Steps, else ErrUnsupportedCLI.
func (r Recipe) stepsFor(cli string) ([]Step, error) {
	if s, ok := r.PerCLI[cli]; ok {
		return s, nil
	}
	if len(r.Steps) > 0 {
		return r.Steps, nil
	}
	return nil, fmt.Errorf("%w: cli=%s recipe=%s", ErrUnsupportedCLI, cli, r.Name)
}

// mergeParams overlays caller-supplied values onto declared defaults and
// errors on any required parameter left unset. Only declared parameters are
// carried forward — undeclared caller keys are ignored so a stray --param
// cannot smuggle an unvalidated token into a body.
func (r Recipe) mergeParams(in Params) (Params, error) {
	out := make(Params, len(r.Params))
	for _, d := range r.Params {
		if v, ok := in[d.Name]; ok {
			out[d.Name] = v
			continue
		}
		if d.Default != "" {
			out[d.Name] = d.Default
			continue
		}
		if d.Required {
			return nil, fmt.Errorf("%w: %s", ErrMissingParam, d.Name)
		}
	}
	return out, nil
}

// paramRE matches a {{token}} placeholder (optional surrounding whitespace).
var paramRE = regexp.MustCompile(`\{\{\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*\}\}`)

// substitute replaces every {{token}} in body with its parameter value and
// errors if any token is unresolved — never emitting a body with an empty or
// leftover placeholder (which would, e.g., send "/plugin install @" and hang).
func substitute(body string, params Params) (string, error) {
	var missing []string
	out := paramRE.ReplaceAllStringFunc(body, func(m string) string {
		name := paramRE.FindStringSubmatch(m)[1]
		if v, ok := params[name]; ok {
			return v
		}
		missing = append(missing, name)
		return m
	})
	if len(missing) > 0 {
		return "", fmt.Errorf("%w: %s", ErrUnknownParam, strings.Join(missing, ", "))
	}
	return out, nil
}
