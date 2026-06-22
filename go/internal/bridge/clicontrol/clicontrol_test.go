package clicontrol

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

// TestEventWireValues locks the abstract event identifiers to their wire
// strings. These ARE the keys in each manifest's `controls` table, so a rename
// here silently breaks every mapping — the contract must be pinned.
func TestEventWireValues(t *testing.T) {
	cases := map[Event]string{
		EventUsage:    "usage",
		EventStatus:   "status",
		EventCleanCtx: "clean_ctx",
	}
	for ev, want := range cases {
		if string(ev) != want {
			t.Errorf("Event %v wire value = %q, want %q", ev, string(ev), want)
		}
	}
}

// TestErrUnsupportedIsMatchable verifies ErrUnsupported survives wrapping so
// consumers can branch on it (the prober skips unsupported families silently).
func TestErrUnsupportedIsMatchable(t *testing.T) {
	wrapped := fmt.Errorf("family=ollama event=usage: %w", ErrUnsupported)
	if !errors.Is(wrapped, ErrUnsupported) {
		t.Fatal("wrapped ErrUnsupported not matchable via errors.Is")
	}
}

// stubController is a no-op Controller used only to pin the interface shape.
type stubController struct{}

func (stubController) Do(context.Context, string, Event) (Response, error) {
	return Response{}, ErrUnsupported
}

// TestControllerInterface pins the Controller port: a minimal implementation
// satisfies it and returns the abstract types, so the pipeline-facing contract
// stays stable.
func TestControllerInterface(t *testing.T) {
	var c Controller = stubController{}
	resp, err := c.Do(context.Background(), "claude", EventUsage)
	if !errors.Is(err, ErrUnsupported) || resp.Pane != "" {
		t.Fatalf("stub Do = (%+v, %v)", resp, err)
	}
}

// TestResponseCarriesContext exercises the Response value type (the carrier the
// Controller returns) so its fields are part of the package's covered surface.
func TestResponseCarriesContext(t *testing.T) {
	r := Response{Family: "claude", Event: EventUsage, Pane: "12% used"}
	if r.Family != "claude" || r.Event != EventUsage || r.Pane == "" {
		t.Errorf("Response round-trip failed: %+v", r)
	}
}
