package agentcmd

import (
	"errors"
	"fmt"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestLaunchCause_TheEnvelopeCarriesTheTypedCauseOfTheLaunch(t *testing.T) {
	t.Parallel()
	wrapped := fmt.Errorf("phase selfheal-orchestrator: %w", core.WalkError{CLIs: []string{"claude-tmux"}, Err: core.LaunchCause{Cause: "guard_refused", Err: errors.New("bridge: launch exit=81")}})
	if got := launchCause(wrapped); got != "guard_refused" {
		t.Fatalf("launchCause = %q, want guard_refused", got)
	}
	if got := launchCause(errors.New("bridge down")); got != "" {
		t.Fatalf("an error with no cause gives %q", got)
	}
	if (Envelope{Cause: "guard_refused"}).Cause != "guard_refused" {
		t.Fatal("Envelope.Cause")
	}
}
