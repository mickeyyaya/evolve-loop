package core

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
)

// TestCodeManifestGate_DualRegistered asserts the dedicated manifest-gate code
// is re-exported through core exactly like every other ship code (the
// CodeCommitPrefixGate dual-registration pattern). Consumers import core, not
// shiperr, so a code that exists only in shiperr is unreachable from the ship
// phase and the debugger-routing layer (cycle-1064).
func TestCodeManifestGate_DualRegistered(t *testing.T) {
	if CodeManifestGate != shiperr.CodeManifestGate {
		t.Errorf("core.CodeManifestGate = %q, want the shiperr re-export %q", CodeManifestGate, shiperr.CodeManifestGate)
	}
	if CodeManifestGate == CodeGitStageFailed {
		t.Errorf("core.CodeManifestGate must be distinct from CodeGitStageFailed")
	}
	// Exercise the constructor consumers actually use.
	err := NewShipError(CodeManifestGate, ShipClassPrecondition, StageAtomicShip, "manifest-gate block")
	got, ok := AsShipError(err)
	if !ok || got.Code != CodeManifestGate {
		t.Fatalf("AsShipError(core-built MANIFEST_GATE) = %+v, ok=%v", got, ok)
	}
}
