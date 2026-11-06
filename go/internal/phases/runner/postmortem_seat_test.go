package runner

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/launchoutcome"
)

func TestEndsOutsideTheAgentSession_ARefusedSeatEndsOutside(t *testing.T) {
	t.Parallel()
	if !endsOutsideTheAgentSession[launchoutcome.ExitSeatTokenRefused] {
		t.Fatal("a refused seat reads as a session that ended inside")
	}
}
