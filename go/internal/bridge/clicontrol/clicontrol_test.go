package clicontrol

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

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

func TestErrUnsupportedIsMatchable(t *testing.T) {
	wrapped := fmt.Errorf("family=ollama event=usage: %w", ErrUnsupported)
	if !errors.Is(wrapped, ErrUnsupported) {
		t.Fatal("wrapped ErrUnsupported not matchable via errors.Is")
	}
}

type stubController struct{}

func (stubController) Do(context.Context, string, Event) (Response, error) {
	return Response{}, ErrUnsupported
}

func TestControllerInterface(t *testing.T) {
	var c Controller = stubController{}
	resp, err := c.Do(context.Background(), "claude", EventUsage)
	if !errors.Is(err, ErrUnsupported) || resp.Pane != "" {
		t.Fatalf("stub Do = (%+v, %v)", resp, err)
	}
}

func TestResponseCarriesContext(t *testing.T) {
	r := Response{Family: "claude", Event: EventUsage, Pane: "12% used"}
	if r.Family != "claude" || r.Event != EventUsage || r.Pane == "" {
		t.Errorf("Response round-trip failed: %+v", r)
	}
}
