// Package recipe runs scripted multi-step slash-command sequences, such as a plugin install,
// through the tmux REPL of an LLM CLI. See docs/architecture/packages/internal-bridge-recipe.md.
package recipe

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	ErrUnsupportedCLI        = errors.New("recipe: no steps for cli")
	ErrMissingParam          = errors.New("recipe: missing required parameter")
	ErrUnknownParam          = errors.New("recipe: unresolved parameter token")
	ErrStepTimeout           = errors.New("recipe: step await timed out")
	ErrAwaitFailRegex        = errors.New("recipe: step fail_regex matched")
	ErrAutoRespondEscalation = errors.New("recipe: auto-respond escalated")
	ErrInvalidRecipe         = errors.New("recipe: invalid")
)

type SendKind string

const (
	KindCommand SendKind = "command"
	KindKeys    SendKind = "keys"
)

type OnTimeout string

const (
	OnTimeoutAbort    OnTimeout = "abort"
	OnTimeoutContinue OnTimeout = "continue"
)

type ParamDecl struct {
	Name     string `json:"name"`
	Required bool   `json:"required"`
	Default  string `json:"default,omitempty"`
}

type Send struct {
	Kind SendKind `json:"kind"`
	Body string   `json:"body"`
}

type Step struct {
	Name      string    `json:"name"`
	Send      Send      `json:"send"`
	Await     Await     `json:"await"`
	OnTimeout OnTimeout `json:"on_timeout,omitempty"`
}

type Recipe struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Params      []ParamDecl       `json:"params,omitempty"`
	Steps       []Step            `json:"steps,omitempty"`
	PerCLI      map[string][]Step `json:"per_cli,omitempty"`
}

type Params map[string]string

func (r Recipe) stepsFor(cli string) ([]Step, error) {
	if s, ok := r.PerCLI[cli]; ok {
		return s, nil
	}
	if len(r.Steps) > 0 {
		return r.Steps, nil
	}
	return nil, fmt.Errorf("%w: cli=%s recipe=%s", ErrUnsupportedCLI, cli, r.Name)
}

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

var paramRE = regexp.MustCompile(`\{\{\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*\}\}`)

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
