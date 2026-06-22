// Package clicontrol is the CLI-control abstraction layer: the pipeline-facing
// vocabulary that hides each LLM CLI's concrete command surface behind a small
// set of ABSTRACT control events. A consumer names an event (usage, status,
// clean_ctx, …) and a family (claude, codex, agy, ollama); a Controller
// resolves the family's per-CLI mapping table to the concrete slash command and
// drives it over the tmux bridge. The pipeline only ever chooses model + family
// — it never names a slash command, so swapping a CLI's command surface is a
// config edit (the manifest `controls` table), not a code change (Adapter +
// Strategy).
//
// This is a stdlib-only LEAF: it owns the Event vocabulary, the Controller port,
// and the ErrUnsupported sentinel, but NOT the tmux implementation (that lives
// in the bridge package, which depends on this — keeping the dependency arrow
// one-directional and letting consumers depend on the abstraction alone).
package clicontrol

import (
	"context"
	"errors"
)

// Event is one abstract CLI-control intent. Its concrete per-CLI command is
// resolved from the target family's manifest `controls` table.
type Event string

const (
	// EventUsage queries the CLI's quota / usage / rate-limit standing.
	EventUsage Event = "usage"
	// EventStatus queries the CLI's session / model / auth status.
	EventStatus Event = "status"
	// EventCleanCtx clears the CLI's conversation context (mapped, e.g.,
	// /clear for claude, /new for codex). Defined for the general vocabulary;
	// a consumer wires it independently.
	EventCleanCtx Event = "clean_ctx"
)

// ErrUnsupported reports that a family declares no mapping for an event (or no
// controls table at all) — the honest no-op (e.g. ollama has no usage command).
// Callers branch with errors.Is.
var ErrUnsupported = errors.New("clicontrol: family does not support event")

// Response is the captured outcome of a control event.
type Response struct {
	Family string
	Event  Event
	Pane   string // the captured REPL pane after the command settled
}

// Controller executes an abstract control event against a family's REPL and
// returns the captured response. Implementations MUST be safe for concurrent
// use: the per-cycle prober fans Do out across all families at once.
type Controller interface {
	Do(ctx context.Context, family string, ev Event) (Response, error)
}
