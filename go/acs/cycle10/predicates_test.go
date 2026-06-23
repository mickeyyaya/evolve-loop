//go:build acs

// Package cycle10 materializes the cycle-10 acceptance criteria for two
// committed top_n tasks (flag-reduction wave-1):
//
//	w1-dead-flags — delete 5 StatusActive/StatusInternal flags with 0 readers
//	  (PROMPT_MAX_TOKENS, REAP_ORPHANS, TESTING, SANDBOX_FALLBACK_ON_EPERM,
//	   WORKTREE_PATH) and clean all surface refs.
//
//	w1-tombstones-and-compose — delete 8 StatusDeprecated tombstones
//	  (ANTHROPIC_BASE_URL, HANG_CLASSIFIER, MODELCATALOG_AUTOREFRESH,
//	   MARKETPLACE_DIR, ADVISOR_DEPTH, DISABLE_WORKSPACE_GUARD, POLICY_BYPASS,
//	   PLATFORM) + convert EVOLVE_COMPOSE_PHASES from env signal to DI/policy.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	w1-dead-flags:
//	  AC1  5 dead flag names absent from flagregistry         → C10_001 (behavioral)
//	  AC2  go test ./internal/flagregistry/... PASS           → manual+checklist (CI)
//	  AC3  No new ACS failures after WORKTREE_PATH guards     → manual+checklist (CI)
//	  AC4  FlagCeiling=48 unchanged                           → C10_002 (config-check, pre-existing GREEN)
//	  AC5  LiveFeatureFlagCeiling=21 unchanged                → C10_003 (config-check, pre-existing GREEN)
//
//	w1-tombstones-and-compose:
//	  AC1  9 tombstone names absent from flagregistry         → C10_004 (behavioral)
//	  AC2  No EVOLVE_COMPOSE_PHASES in cmd_compose.go         → C10_005 (behavioral, absence)
//	  AC3  No env bridge reads in cmd_cycle.go / cmd_loop.go  → C10_006 (behavioral, absence)
//	  AC4  go test ./cmd/evolve/... ./internal/core/... PASS  → manual+checklist (CI)
//	  AC5  FlagCeiling/LiveFeatureFlagCeiling UNCHANGED       → covered by C10_002/C10_003
//
// Floor binding (R9.3): predicates authored only for committed top_n tasks.
// Deferred items (none this cycle) get zero predicates.
package cycle10

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolveloop/go/pkg/acsassert"
)

// TestC10_001_DeadFlagsAbsentFromRegistry verifies that 5 confirmed-dead flags
// are absent from the live flagregistry after Builder removes their rows in
// w1-dead-flags.
//
// BEHAVIORAL: calls flagregistry.Lookup() — the production SSOT function that
// binary-searches the live All slice. If Lookup returns (flag, true), the row
// was not deleted. A magic-string patch in a doc cannot satisfy this; the row
// must be genuinely removed from registry_table.go.
//
// Negative: if any Lookup returns ok=true the test fails.
// RED: all 5 rows are currently present in registry_table.go; Lookup returns ok=true.
func TestC10_001_DeadFlagsAbsentFromRegistry(t *testing.T) {
	deadFlags := []string{
		"EVOLVE_PROMPT_MAX_TOKENS",
		"EVOLVE_REAP_ORPHANS",
		"EVOLVE_TESTING",
		"EVOLVE_SANDBOX_FALLBACK_ON_EPERM",
		"EVOLVE_WORKTREE_PATH",
	}
	for _, name := range deadFlags {
		if f, ok := flagregistry.Lookup(name); ok {
			t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — dead flag still registered.\n"+
				"Builder must delete this row from registry_table.go (w1-dead-flags).\n"+
				"Current entry: Status=%q Cluster=%q",
				name, f.Status, f.Cluster)
		}
	}
}

// C10_002/C10_003 (FlagCeiling/LiveFeatureFlagCeiling "unchanged at 48/21")
// retired at integration: the operator ratchets both consts when banking a cycle
// to main (the ceiling-decoupling rule), so a per-cycle exact-value predicate
// here directly conflicts with that ratchet and re-reddens as the campaign
// reduces. The durable SSOT ratchets — TestRegistry_FlagCeiling and
// TestRegistry_LiveFeatureFlagCeiling in internal/flagregistry — are the real
// guards. (Same retired anti-pattern as the cycle-N FlagCeilingConstIsN sweep,
// PR #162.)

// TestC10_004_TombstoneFlagsAbsentFromRegistry verifies that 8 tombstone /
// internal-env-signal rows are absent from flagregistry after Builder removes
// them in w1-tombstones-and-compose (POLICY_BYPASS deferred — see below).
//
// BEHAVIORAL: calls flagregistry.Lookup() for each flag — same SSOT function
// as C10_001. Returning ok=true means the row was not removed.
// Negative: each Lookup returning ok=true is an individual failure.
//
// RED: all 9 rows are currently present in registry_table.go.
func TestC10_004_TombstoneFlagsAbsentFromRegistry(t *testing.T) {
	tombstones := []string{
		// Fully migrated tombstones (no live env reads, text-surface cleanup only)
		"EVOLVE_ANTHROPIC_BASE_URL",
		"EVOLVE_HANG_CLASSIFIER",
		"EVOLVE_MODELCATALOG_AUTOREFRESH",
		"EVOLVE_MARKETPLACE_DIR",
		// Tombstones with live env bridges (bridge must be removed before row deletion)
		"EVOLVE_ADVISOR_DEPTH",
		"EVOLVE_DISABLE_WORKSPACE_GUARD",
		// EVOLVE_POLICY_BYPASS bridge converted to --bypass-policy CLI flag in cycle-15;
		// row deleted in that cycle (bypass-policy-flag task).
		"EVOLVE_POLICY_BYPASS",
		"EVOLVE_PLATFORM",
		// Internal env signal converted to DI (ComposePhases bool in PhaseRequest)
		"EVOLVE_COMPOSE_PHASES",
	}
	for _, name := range tombstones {
		if f, ok := flagregistry.Lookup(name); ok {
			t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — tombstone still registered.\n"+
				"Builder must delete this row from registry_table.go (w1-tombstones-and-compose).\n"+
				"Current entry: Status=%q Cluster=%q",
				name, f.Status, f.Cluster)
		}
	}
}

// TestC10_005_ComposePhasesEnvRefsAbsent verifies that cmd_compose.go contains
// no references to EVOLVE_COMPOSE_PHASES after Builder converts the env signal
// to a DI boolean (ComposePhases bool in PhaseRequest / CycleRequest).
//
// BEHAVIORAL (absence): acsassert.FileNotContains fails only when the string IS
// present — meaning old env calls remain. Builder cannot game this by renaming
// only the comment; the os.Getenv/os.Setenv literal strings must be removed.
//
// RED: cmd_compose.go currently has os.Getenv + os.Setenv + os.Unsetenv for
// "EVOLVE_COMPOSE_PHASES" at lines 92–98.
func TestC10_005_ComposePhasesEnvRefsAbsent(t *testing.T) {
	root := acsassert.RepoRoot(t)
	composeFile := filepath.Join(root, "go", "cmd", "evolve", "cmd_compose.go")
	if !acsassert.FileNotContains(t, composeFile, "EVOLVE_COMPOSE_PHASES") {
		t.Errorf("RED: cmd_compose.go still references EVOLVE_COMPOSE_PHASES.\n"+
			"Builder must remove all os.Getenv/os.Setenv/os.Unsetenv calls for\n"+
			"EVOLVE_COMPOSE_PHASES and replace with ComposePhases bool in PhaseRequest.\n"+
			"File: %s", composeFile)
	}
}

// TestC10_006_EnvBridgesRemovedFromCmdFiles verifies that cmd_cycle.go and
// cmd_loop.go no longer contain env bridge reads for EVOLVE_DISABLE_WORKSPACE_GUARD
// or EVOLVE_POLICY_BYPASS after Builder removes the deprecated bridges in
// w1-tombstones-and-compose (DISABLE_WORKSPACE_GUARD) and cycle-15
// bypass-policy-flag (POLICY_BYPASS).
//
// BEHAVIORAL (absence): acsassert.FileNotContains fails only when the flag name
// IS present in the file. The DI replacement fields already exist on CycleRequest
// (DisableWorkspaceGuard bool) and PhaseRequest (BypassPolicy bool); this predicate
// verifies the bridge env read is gone, not just that the DI field exists.
//
// RED: cmd_cycle.go:190 and cmd_loop.go:186,303 still contain
// cycleEnv["EVOLVE_POLICY_BYPASS"] (DISABLE_WORKSPACE_GUARD was already removed).
func TestC10_006_EnvBridgesRemovedFromCmdFiles(t *testing.T) {
	root := acsassert.RepoRoot(t)
	// EVOLVE_POLICY_BYPASS bridge converted to --bypass-policy CLI flag in cycle-15;
	// now included alongside DISABLE_WORKSPACE_GUARD.
	checks := []struct {
		file string
		flag string
	}{
		{filepath.Join(root, "go", "cmd", "evolve", "cmd_cycle.go"), "EVOLVE_DISABLE_WORKSPACE_GUARD"},
		{filepath.Join(root, "go", "cmd", "evolve", "cmd_loop.go"), "EVOLVE_DISABLE_WORKSPACE_GUARD"},
		{filepath.Join(root, "go", "cmd", "evolve", "cmd_cycle.go"), "EVOLVE_POLICY_BYPASS"},
		{filepath.Join(root, "go", "cmd", "evolve", "cmd_loop.go"), "EVOLVE_POLICY_BYPASS"},
	}
	for _, tc := range checks {
		if !acsassert.FileNotContains(t, tc.file, tc.flag) {
			t.Errorf("RED: %s still contains env bridge read for %s.\n"+
				"Builder must remove the cycleEnv[%q] bridge read.\n"+
				"File: %s",
				filepath.Base(tc.file), tc.flag, tc.flag, tc.file)
		}
	}
}
