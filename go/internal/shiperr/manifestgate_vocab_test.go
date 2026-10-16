package shiperr

import (
	"strings"
	"testing"
)

func TestManifestGateCode_WireStringAndDistinctness(t *testing.T) {
	if got, want := string(CodeManifestGate), "MANIFEST_GATE"; got != want {
		t.Errorf("CodeManifestGate = %q, want %q", got, want)
	}
	if CodeManifestGate == CodeGitStageFailed {
		t.Errorf("CodeManifestGate must not alias CodeGitStageFailed (%q)", CodeGitStageFailed)
	}
}

func TestManifestGateCode_RoundTripsThroughShipError(t *testing.T) {
	err := NewShipError(CodeManifestGate, ShipClassPrecondition, StageAtomicShip,
		"ship: manifest-gate (enforce): refusing to commit 1 path(s)", "out_of_manifest", "go/internal/foreign/leak_test.go")
	got, ok := AsShipError(error(err))
	if !ok {
		t.Fatalf("AsShipError failed for a MANIFEST_GATE ShipError")
	}
	if got.Code != CodeManifestGate {
		t.Errorf("round-tripped Code = %q, want %q", got.Code, CodeManifestGate)
	}
	if got.Class != ShipClassPrecondition || got.Stage != StageAtomicShip {
		t.Errorf("class/stage = %q/%q, want %q/%q", got.Class, got.Stage, ShipClassPrecondition, StageAtomicShip)
	}
	if s := got.Error(); !strings.Contains(s, "MANIFEST_GATE") || !strings.Contains(s, "manifest-gate") {
		t.Errorf("Error() = %q, want it to render the MANIFEST_GATE code", s)
	}
	if got.Debug["out_of_manifest"] != "go/internal/foreign/leak_test.go" {
		t.Errorf("Debug[out_of_manifest] = %q, want the leaked path", got.Debug["out_of_manifest"])
	}
}
