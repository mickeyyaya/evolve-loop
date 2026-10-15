// Package clicontrol is the CLI-control abstraction layer: the
// pipeline-facing vocabulary that hides each LLM CLI's concrete command
// surface behind a small set of ABSTRACT control events.
package clicontrol

import (
	"context"
	"errors"
)

type Event string

const (
	EventUsage    Event = "usage"
	EventStatus   Event = "status"
	EventCleanCtx Event = "clean_ctx"
)

var ErrUnsupported = errors.New("clicontrol: family does not support event")

type Response struct {
	Family string
	Event  Event
	Pane   string
}

type Controller interface {
	Do(ctx context.Context, family string, ev Event) (Response, error)
}
