package guards

import "testing"

func TestContractPathsAreProtectedSurface(t *testing.T) {
	paths := []string{
		"/wt/go/internal/stageevidence/recorder.go",
		"/wt/go/internal/stageevidence/flip.go",
		"/wt/go/internal/stageevidence/flip_test.go",
		"/wt/go/internal/core/stage_evidence.go",
		"/wt/go/internal/core/stage_evidence_test.go",
		"/wt/go/internal/phases/ship/stage_evidence.go",
		"/wt/go/internal/phases/ship/stage_evidence_test.go",
		"/wt/go/internal/policy/stage_keys.go",
		"/wt/go/internal/policy/stage_keys_test.go",
		"/wt/go/cmd/evolve/cmd_stage.go",
		"/wt/go/cmd/evolve/cmd_stage_chain.go",
		"/wt/go/cmd/evolve/cmd_stage_migrate.go",
		"/wt/go/cmd/evolve/cmd_stage_migrate_test.go",
		"/wt/go/internal/adapters/ledger/stage_evidence.go",
		"/wt/go/internal/adapters/ledger/stage_flip.go",
		"/wt/go/internal/adapters/ledger/stage_flip_test.go",
		"/wt/go/internal/looppreflight/stage_contract.go",
		"/wt/go/internal/looppreflight/stage_contract_test.go",
		"/wt/go/internal/gateregistry/stage_contract_ratchet.json",
		"/wt/go/internal/gateregistry/stage_contract_ratchet.go",
		"/wt/go/internal/gateregistry/stage_conformance_test.go",
	}
	for _, p := range paths {
		if !IsProtectedSurface(p) {
			t.Errorf("IsProtectedSurface(%q) = false, want true (the shadow stage contract must not weaken silently)", p)
		}
		if ClassOf(p) == "" {
			t.Errorf("ClassOf(%q) is empty, want a guarantee class", p)
		}
	}
}

func TestContractPathsAreProtectedSurface_LeavesTheNeighboursOpen(t *testing.T) {
	for _, p := range []string{
		"/wt/go/internal/core/stage_other.go",
		"/wt/go/internal/policy/stage_names.go",
		"/wt/go/cmd/evolve/cmd_models_stage.go",
	} {
		if IsProtectedSurface(p) {
			t.Errorf("IsProtectedSurface(%q) = true, want false", p)
		}
	}
}
