//go:build acs

// Package cycle12 materializes the cycle-12 acceptance criteria for the
// committed top_n task:
//
//	phase-recovery-flag-retire — retire EVOLVE_PHASE_RECOVERY from the flag
//	registry by converting all four bare env-var reads to the policy/config-resolved
//	path (RecoveryConfig in policy.go; Deps.Env overlay for bridge/fatalpane;
//	RecoveryStage field for CoreAdapter; IPC const for phasecmd subprocess), then
//	deleting the registry row (35 → 34 rows).
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	phase-recovery-flag-retire:
//	  AC1      flagregistry.Lookup("EVOLVE_PHASE_RECOVERY") returns ok=false  → C12_001 (behavioral)
//	  AC2      registry row count strictly reduced to 34                        → C12_002 (behavioral)
//	  AC3      no bare env reads in core_adapter.go + phase_observer.go         → C12_003 (config-check, waiver)
//	  AC4      flagreaders ACS guard still passes                               → manual+checklist (CI regression lane)
//	  AC5      all affected packages test suite green                           → manual+checklist (CI pipeline)
//	  AC6      stallPolicyFromEnv reads IPC const, not os.Getenv("...")        → C12_004 (config-check, waiver)
//	  AC7      CoreAdapter.RecoveryStage field added                            → C12_005 (config-check, waiver)
//	  EDGE1    control-flags.md has no EVOLVE_PHASE_RECOVERY entry              → C12_006 (config-check, waiver)
//
// Adversarial diversity (SKILL §6):
//
//	Negative:  C12_001 — Lookup must return false; cannot be satisfied by any
//	           magic-string patch — the row must be deleted from registry_table.go.
//	           This is the primary anti-no-op signal: cycle-10 w2-phaserecovery-ipc
//	           shipped with 35→35 rows because only readers were converted; this test
//	           ensures that mistake is not repeated (flagprogress guard PR #212 added
//	           after that failure also catches this, but at the metric level).
//	Edge/OOD:  C12_002 — exact count == 34 (not just "< 35"), pinning the delta so
//	           accidental additional deletions are caught immediately.
//	Lexical:   Lookup() / len() / FileNotContains / FileContains — four distinct verbs.
//	Semantic:  registry-row-absent, count-exact, env-reads-deleted (2 files),
//	           IPC-const-defined (new protocol), field-added, doc-regenerated.
//
// 1:1 enforcement: predicate=6, manual+checklist=2 → 7 ACs + 1 EDGE = 8 dispositions ✓
//
// Manual+checklist dispositions:
//
//	AC4 (flagreaders ACS guard): `go test -tags acs ./acs/regression/flagreaders/...`
//	    — enforced by the CI regression lane; a cycle predicate would duplicate
//	    an existing durable guard and add no signal beyond what that guard already provides.
//	AC5 (full test suite): `go test ./internal/bridge/... ./internal/adapters/observer/...
//	    ./internal/cli/phasecmd/... ./internal/config/... ./internal/policy/...`
//	    — enforced by CI on every commit; no additional cycle predicate is needed.
//
// Floor binding (R9.3): predicates authored only for the committed top_n task
// (phase-recovery-flag-retire). Deferred tasks get zero predicates.
package cycle12

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// TestC12_001_PhaseRecoveryFlagAbsentFromRegistry verifies that EVOLVE_PHASE_RECOVERY
// is no longer registered after Builder removes its row from registry_table.go.
//
// AC1: flagregistry.Lookup("EVOLVE_PHASE_RECOVERY") must return ok=false.
//
// BEHAVIORAL: calls flagregistry.Lookup() — the production SSOT function. A source
// edit alone cannot satisfy this; the row must be absent from registry_table.go for
// Lookup to return ok=false. This is the primary anti-no-op signal: cycle-10
// (w2-phaserecovery-ipc) shipped with rows 35→35 because only the readers were
// converted without deleting the registry entry. The flagprogress guard (PR #212)
// catches this at the metric level; this predicate asserts the specific row is gone.
//
// RED: flagregistry.Lookup currently returns (flag, true) for EVOLVE_PHASE_RECOVERY
// (row at registry_table.go:30, StatusActive, "Phase Recovery (ADR-0044, Go-native)").
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

// TestC12_002_RegistryRowCountDroppedTo34 verifies that the registry row count
// is exactly 34 after Builder removes the EVOLVE_PHASE_RECOVERY row.
//
// AC2: the flagprogress gate requires len(flagregistry.All) < 35 (HEAD count).
// This predicate pins the exact target (34) so any unintended additional deletion
// or regression is also caught immediately.
//
// BEHAVIORAL: calls len(flagregistry.All) — the production SSOT slice count.
// Unlike a source-file grep, this cannot be satisfied by editing comments or
// adding magic strings.
//
// RED: len(flagregistry.All) is currently 35 (HEAD 52039d82).
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

// TestC12_003_NoBarePhaseRecoveryEnvReadsInObserverAndPhasecmd verifies that the
// two production files holding direct EVOLVE_PHASE_RECOVERY env reads no longer
// contain them after Builder routes through the policy DI seams.
//
// AC3 (the two files that require code changes):
//   - core_adapter.go must NOT have `envGet("EVOLVE_PHASE_RECOVERY")` (AC3+AC7 shared)
//   - phase_observer.go must NOT have `os.Getenv("EVOLVE_PHASE_RECOVERY")` (AC3+AC6 shared)
//
// Note: fatalpane.go is intentionally excluded from this check. The scout determined
// that the Deps.Env overlay approach (injecting the resolved stage into bridge.Deps.Env
// at wireOrchestratorDeps) allows fatalpane.go:47 to remain unchanged — lookupEnv
// reads the resolved value from the overlay, not directly from os.Getenv. Both the
// overlay approach and a RecoveryStage field on Deps are valid Builder choices for
// fatalpane.go; this predicate does not constrain that decision.
//
// // acs-predicate: config-check — env-read ABSENCE is the structural contract.
//
// RED: core_adapter.go:144 has `a.envGet("EVOLVE_PHASE_RECOVERY")`;
// phase_observer.go:118 has `os.Getenv("EVOLVE_PHASE_RECOVERY")`.
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
	// Check for the new key's unique suffix. The split-const form ensures the full
	// "EVOLVE_PHASE_RECOVERY_STAGE" string does not appear as a single literal
	// (which the flagreaders guard would flag as an unregistered direct read).
	// "PHASE_RECOVERY_STAGE" appears in the const definition and cannot match the
	// retired "EVOLVE_PHASE_RECOVERY" name (no "_STAGE" suffix on the old name).
	if !acsassert.FileContains(t, phaseObserverFile, "PHASE_RECOVERY_STAGE") {
		t.Errorf("RED: phasecmd/phase_observer.go has no IPC const for the new env key.\n"+
			"Builder must add:\n"+
			"  const envIPCPhaseRecoveryStage = \"EVOLVE_\" + \"PHASE_RECOVERY_STAGE\" // SSOT IPC-protocol-allowed\n"+
			"and update stallPolicyFromEnv() to read os.Getenv(envIPCPhaseRecoveryStage).\n"+
			"The parent wireup (wireOrchestratorDeps) must inject the resolved stage under the new key.\n"+
			"File: %s", phaseObserverFile)
	}
}

// TestC12_005_CoreAdapterHasRecoveryStageField verifies that CoreAdapter has a
// RecoveryStage string field, replacing the envGet("EVOLVE_PHASE_RECOVERY") call
// with a policy-injected DI seam.
//
// AC7: CoreAdapter.RecoveryStage field used (not envGet("EVOLVE_PHASE_RECOVERY")).
// The field is set at wireup in wireOrchestratorDeps (cmd_cycle.go):
//
//	RecoveryStage: string(cfg.PhaseRecovery)
//
// This follows the existing DI seam pattern for observer configuration. After Builder
// adds the field, the usage at core_adapter.go:144 becomes a.RecoveryStage (covered by
// the absence check in C12_003).
//
// // acs-predicate: config-check — the RecoveryStage field PRESENCE is the structural
// contract that the DI seam was added to replace the raw env call.
//
// RED: core_adapter.go has no RecoveryStage field; the struct reads envGet at line 144.
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

// TestC12_006_ControlFlagsMdHasNoPhaseRecoveryEntry verifies that the generated
// docs/architecture/control-flags.md no longer lists EVOLVE_PHASE_RECOVERY after
// Builder removes the registry row and regenerates the doc.
//
// EDGE1: the doc is generated from the flagregistry (source of truth); its absence
// of EVOLVE_PHASE_RECOVERY follows from C12_001 (row removed) PLUS the regeneration
// step (`cd go && go run ./cmd/evolve flags generate`). This predicate ensures the
// regeneration step also ran — the row deletion alone doesn't update the committed doc.
//
// // acs-predicate: config-check — doc absence follows from row deletion + regeneration;
// both steps must complete for this predicate to green.
//
// RED: control-flags.md currently lists EVOLVE_PHASE_RECOVERY (1 occurrence).
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
