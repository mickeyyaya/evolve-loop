package profiles_test

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func TestGateInputInventory_NoAgentWritableInputIsAuthority(t *testing.T) {
	for _, finding := range profiles.RealWritableAuthority(t, guards.IsProtectedSurface) {
		t.Errorf("%s: verify the input or deny the write", finding)
	}
}
