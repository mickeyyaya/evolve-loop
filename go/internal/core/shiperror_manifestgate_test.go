package core

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
)

func TestCodeManifestGate_DualRegistered(t *testing.T) {
	if CodeManifestGate != shiperr.CodeManifestGate {
		t.Errorf("core.CodeManifestGate = %q, want the shiperr re-export %q", CodeManifestGate, shiperr.CodeManifestGate)
	}
	if CodeManifestGate == CodeGitStageFailed {
		t.Errorf("core.CodeManifestGate must be distinct from CodeGitStageFailed")
	}
	err := NewShipError(CodeManifestGate, ShipClassPrecondition, StageAtomicShip, "manifest-gate block")
	got, ok := AsShipError(err)
	if !ok || got.Code != CodeManifestGate {
		t.Fatalf("AsShipError(core-built MANIFEST_GATE) = %+v, ok=%v", got, ok)
	}
}
