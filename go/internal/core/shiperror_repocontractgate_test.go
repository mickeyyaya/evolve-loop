package core

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
)

// TestCodeRepoContractGate_DualRegistered mirrors the CodeManifestGate
// dual-registration pin: the ship-time repo-contract scanner pack's code must
// be reachable through core (consumers import core, not shiperr), distinct
// from git-failure codes, and constructible/recoverable through the real
// error machinery.
func TestCodeRepoContractGate_DualRegistered(t *testing.T) {
	if CodeRepoContractGate != shiperr.CodeRepoContractGate {
		t.Errorf("core.CodeRepoContractGate = %q, want the shiperr re-export %q", CodeRepoContractGate, shiperr.CodeRepoContractGate)
	}
	if CodeRepoContractGate == CodeGitStageFailed || CodeRepoContractGate == CodeManifestGate {
		t.Error("core.CodeRepoContractGate must be distinct from git-stage and manifest-gate codes")
	}
	err := NewShipError(CodeRepoContractGate, ShipClassPrecondition, StageAtomicShip, "repo-contract scanner RED")
	got, ok := AsShipError(err)
	if !ok || got.Code != CodeRepoContractGate {
		t.Fatalf("AsShipError(core-built REPO_CONTRACT_GATE) = %+v, ok=%v", got, ok)
	}
}
