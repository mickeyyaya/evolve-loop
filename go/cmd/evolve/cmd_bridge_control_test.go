package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/bridge/clicontrol"
)

// TestEmitControl_Outcomes verifies the control command maps each Controller
// outcome to the right exit code + output: a captured pane prints on success,
// an unsupported event is a clean distinct exit (3), any other error is exit 1.
func TestEmitControl_Outcomes(t *testing.T) {
	t.Run("success prints pane", func(t *testing.T) {
		var out, errb bytes.Buffer
		do := func(_ context.Context, family string, ev clicontrol.Event) (clicontrol.Response, error) {
			return clicontrol.Response{Family: family, Event: ev, Pane: "5h limit: 100% left (resets 14:39)"}, nil
		}
		if rc := emitControl(do, "codex", "usage", &out, &errb); rc != 0 {
			t.Fatalf("rc=%d, want 0", rc)
		}
		if !strings.Contains(out.String(), "100% left") {
			t.Errorf("stdout=%q, want the captured pane", out.String())
		}
	})

	t.Run("unsupported is exit 3", func(t *testing.T) {
		var out, errb bytes.Buffer
		do := func(_ context.Context, _ string, _ clicontrol.Event) (clicontrol.Response, error) {
			return clicontrol.Response{}, fmt.Errorf("wrap: %w", clicontrol.ErrUnsupported)
		}
		if rc := emitControl(do, "ollama", "usage", &out, &errb); rc != 3 {
			t.Fatalf("rc=%d, want 3", rc)
		}
		if !strings.Contains(errb.String(), "ollama") {
			t.Errorf("stderr=%q, want it to name the family", errb.String())
		}
	})

	t.Run("other error is exit 1", func(t *testing.T) {
		var out, errb bytes.Buffer
		do := func(_ context.Context, _ string, _ clicontrol.Event) (clicontrol.Response, error) {
			return clicontrol.Response{}, errors.New("boot failed")
		}
		if rc := emitControl(do, "claude", "usage", &out, &errb); rc != 1 {
			t.Fatalf("rc=%d, want 1", rc)
		}
	})
}

// TestRunBridgeControl_ArgValidation covers the flag/positional parsing without
// touching tmux: wrong arity, a missing workspace, help, and an unknown flag.
func TestRunBridgeControl_ArgValidation(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want int
	}{
		{"no args", nil, 10},
		{"one positional", []string{"claude"}, 10},
		{"missing workspace", []string{"claude", "usage"}, 10},
		{"unknown flag", []string{"claude", "usage", "--nope"}, 10},
		{"help", []string{"--help"}, 0},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var out, errb bytes.Buffer
			if rc := runBridgeControl(tc.args, &out, &errb); rc != tc.want {
				t.Errorf("runBridgeControl(%v)=%d, want %d (stderr=%q)", tc.args, rc, tc.want, errb.String())
			}
		})
	}
}
