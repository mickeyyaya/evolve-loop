//go:build acs

package cycle12

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func TestC12_001_PhaseRecoveryFlagAbsentFromRegistry(t *testing.T) {
	const name = "EVOLVE_PHASE_RECOVERY"
	if f, ok := flagregistry.Lookup(name); ok {
		t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — registry row still present.\n"+
			"Builder must remove the EVOLVE_PHASE_RECOVERY row from registry_table.go.\n"+
			"Cycle-10 (w2-phaserecovery-ipc) shipped 35→35 by converting readers without\n"+
			"deleting this row; that pattern is now blocked by flagprogress guard (PR #212).\n"+
			"Current entry: Status=%q Cluster=%q",
			name, f.Status, f.Cluster)
	}
}

func TestC12_002_RegistryRowCountDroppedTo34(t *testing.T) {
	const target = 34
	if got := len(flagregistry.All); got != target {
		t.Errorf("RED: len(flagregistry.All) = %d, want exactly %d.\n"+
			"Builder must remove exactly the EVOLVE_PHASE_RECOVERY row from registry_table.go.\n"+
			"HEAD count: 35. Target after deletion: %d.\n"+
			"Removing additional rows would also fail this exact-count check.",
			got, target, target)
	}
}

func TestC12_003_NoBarePhaseRecoveryEnvReadsInObserverAndPhasecmd(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)

	coreAdapterFile := filepath.Join(root, "go", "internal", "adapters", "observer", "core_adapter.go")
	if !acsassert.FileNotContains(t, coreAdapterFile, `envGet("EVOLVE_PHASE_RECOVERY")`) {
		t.Errorf("RED: core_adapter.go still reads EVOLVE_PHASE_RECOVERY via envGet.\n"+
			"Builder must add RecoveryStage string field to CoreAdapter and replace\n"+
			"a.envGet(\"EVOLVE_PHASE_RECOVERY\") at line 144 with a.RecoveryStage.\n"+
			"Wire at wireOrchestratorDeps: RecoveryStage: string(cfg.PhaseRecovery).\n"+
			"File: %s", coreAdapterFile)
	}

	phaseObserverFile := filepath.Join(root, "go", "internal", "cli", "phasecmd", "phase_observer.go")
	if !acsassert.FileNotContains(t, phaseObserverFile, `os.Getenv("EVOLVE_PHASE_RECOVERY")`) {
		t.Errorf("RED: phasecmd/phase_observer.go still reads EVOLVE_PHASE_RECOVERY via os.Getenv.\n"+
			"Builder must define an IPC const (e.g. envIPCPhaseRecoveryStage =\n"+
			"\"EVOLVE_\"+\"PHASE_RECOVERY_STAGE\" // SSOT IPC-protocol-allowed) and replace\n"+
			"os.Getenv(\"EVOLVE_PHASE_RECOVERY\") in stallPolicyFromEnv() with the IPC const read.\n"+
			"File: %s", phaseObserverFile)
	}
}

// TestC12_004_StallPolicyUsesIPCConst verifies that phase_observer.go defines and
// uses the new IPC const (EVOLVE_PHASE_RECOVERY_STAGE) rather than the retired
// env-var name.
//
// AC6: stallPolicyFromEnv() must read the IPC const, not os.Getenv("EVOLVE_PHASE_RECOVERY").
// The scout specifies the split-const form to prevent the flagreaders guard from
// picking up the key as a live unregistered read:
//
//	const envIPCPhaseRecoveryStage = "EVOLVE_" + "PHASE_RECOVERY_STAGE" // SSOT IPC-protocol-allowed
//
// The parent (wireOrchestratorDeps) injects the resolved stage under the new key;
// the subprocess reads it via envIPCPhaseRecoveryStage. If the parent doesn't inject
// the key, stallPolicyFromEnv defaults to nil-policy (same as shadow/off — fail-safe).
//
// // acs-predicate: config-check — the IPC const PRESENCE (new key suffix "PHASE_RECOVERY_STAGE")
// is the structural contract that the subprocess protocol switched to the new env key.
//
// RED: phase_observer.go currently uses "EVOLVE_PHASE_RECOVERY" directly;
// no "PHASE_RECOVERY_STAGE" suffix is present in the file.
func TestC12_004_StallPolicyUsesIPCConst(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	phaseObserverFile := filepath.Join(root, "go", "internal", "cli", "phasecmd", "phase_observer.go")
	if !acsassert.FileContains(t, phaseObserverFile, "PHASE_RECOVERY_STAGE") {
		t.Errorf("RED: phasecmd/phase_observer.go has no IPC const for the new env key.\n"+
			"Builder must add:\n"+
			"  const envIPCPhaseRecoveryStage = \"EVOLVE_\" + \"PHASE_RECOVERY_STAGE\" // SSOT IPC-protocol-allowed\n"+
			"and update stallPolicyFromEnv() to read os.Getenv(envIPCPhaseRecoveryStage).\n"+
			"The parent wireup (wireOrchestratorDeps) must inject the resolved stage under the new key.\n"+
			"File: %s", phaseObserverFile)
	}
}

func TestC12_005_CoreAdapterHasRecoveryStageField(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	coreAdapterFile := filepath.Join(root, "go", "internal", "adapters", "observer", "core_adapter.go")
	if !acsassert.FileContains(t, coreAdapterFile, "RecoveryStage") {
		t.Errorf("RED: core_adapter.go has no RecoveryStage field.\n"+
			"Builder must add RecoveryStage string to the CoreAdapter struct:\n"+
			"  RecoveryStage string\n"+
			"then replace a.envGet(\"EVOLVE_PHASE_RECOVERY\") at line 144 with a.RecoveryStage,\n"+
			"and wire RecoveryStage: string(cfg.PhaseRecovery) in wireOrchestratorDeps.\n"+
			"File: %s", coreAdapterFile)
	}
}

func TestC12_006_ControlFlagsMdHasNoPhaseRecoveryEntry(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	controlFlags := filepath.Join(root, "docs", "architecture", "control-flags.md")
	if !acsassert.FileNotContains(t, controlFlags, "EVOLVE_PHASE_RECOVERY") {
		t.Errorf("RED: control-flags.md still lists EVOLVE_PHASE_RECOVERY.\n"+
			"Builder must:\n"+
			"  1. Remove the EVOLVE_PHASE_RECOVERY row from registry_table.go\n"+
			"  2. Regenerate the doc: cd go && go run ./cmd/evolve flags generate\n"+
			"File: %s", controlFlags)
	}
}
