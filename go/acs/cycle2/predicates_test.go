//go:build acs

// Package cycle2 materializes the cycle-2 acceptance criteria for the
// committed top_n task:
//
//   - dead-flag-sweep — remove 18 StatusDead flags from flagregistry
//     (registry_table.go), add regression test TestDeadFlagsSweep_Gone in
//     go/internal/flagregistry/registry_deadflags_test.go, and regenerate
//     docs/architecture/control-flags.md (282 → 264 flags).
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	dead-flag-sweep:
//	  AC1  18 dead flags absent from flagregistry.Lookup            → C2_001 (behavioral)
//	  AC2  0 StatusDead rows remain in registry                     → C2_002 (behavioral, adversarial-negative)
//	  AC3  registry row count == 264                                → C2_003 (behavioral, count assertion)
//	  AC4  TestDeadFlagsSweep_Gone passes in flagregistry package   → C2_004 (behavioral, subprocess)
//	  AC5  flagreaders ACS guard exits 0                            → C2_005 (behavioral, subprocess)
//
// Floor binding (R9.3): predicates only for committed top_n task (dead-flag-sweep).
// Deferred tasks (deprecated-no-reader retirement, internal classification) get zero predicates.
package cycle2

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// deadFlags is the full list of 18 StatusDead flags targeted by the sweep.
// Each was verified zero-reader on all surfaces (go/ *.go, .github/, skills/,
// agents/, *.sh) in the cycle-1 and cycle-2 scout cross-surface grep.
var deadFlags = []string{
	"EVOLVE_ANCHOR_EXTRACT",
	"EVOLVE_CARRYOVER_TODO_MAX_UNPICKED",
	"EVOLVE_CONTEXT_DIGEST",
	"EVOLVE_CYCLE_STATE_FILE",
	"EVOLVE_DIR",
	"EVOLVE_DIR_OVERRIDE",
	"EVOLVE_DRY_RUN_PROVISION_WORKTREE",
	"EVOLVE_FAILURE_CLASSIFICATIONS_LOADED",
	"EVOLVE_FANOUT_RETROSPECTIVE",
	"EVOLVE_FANOUT_SCOUT",
	"EVOLVE_INSTINCT_SUMMARY_CAP",
	"EVOLVE_PROFILE_OVERRIDE",
	"EVOLVE_PROMPT_BUDGET_ENFORCE",
	"EVOLVE_RESOLVE_ROOTS_LOADED",
	"EVOLVE_STATE_FILE_OVERRIDE",
	"EVOLVE_STATE_OVERRIDE",
	"EVOLVE_STRICT_FAILURES",
	"EVOLVE_TRIAGE_ENABLED",
}

// goDir returns the go module directory for subprocess calls.
func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

// TestC2_001_DeadFlagsAbsentFromRegistry verifies that all 18 StatusDead flags
// are no longer registered after Builder removes their rows from registry_table.go.
//
// BEHAVIORAL: calls flagregistry.Lookup() directly — the production SSOT
// binary-search function. A source edit alone cannot satisfy this; the flag rows
// must be physically absent for Lookup to return ok=false.
//
// NEGATIVE (AC1): each flag currently has StatusDead and Lookup returns ok=true,
// so the assert-!ok fails.
//
// RED: flagregistry.Lookup returns (flag, true) for all 18 flags — dead rows are
// still registered in registry_table.go.
func TestC2_001_DeadFlagsAbsentFromRegistry(t *testing.T) {
	for _, name := range deadFlags {
		if f, ok := flagregistry.Lookup(name); ok {
			t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — dead flag still registered.\n"+
				"Builder must remove this row from go/internal/flagregistry/registry_table.go.\n"+
				"Current entry: Status=%q Cluster=%q",
				name, f.Status, f.Cluster)
		}
	}
}

// TestC2_002_NoStatusDeadRowsRemain verifies that flagregistry.All contains
// zero entries with Status == StatusDead after the sweep.
//
// BEHAVIORAL: iterates flagregistry.All (the production slice, populated by
// registry_table.go). A no-op implementation cannot satisfy this — the rows must
// be deleted.
//
// ADVERSARIAL-NEGATIVE: this is the strongest anti-no-op signal: even if only
// some dead flags are removed, any remaining StatusDead entry fails the test.
//
// RED: flagregistry.All currently has 18 StatusDead entries.
func TestC2_002_NoStatusDeadRowsRemain(t *testing.T) {
	var deadRemaining []string
	for _, f := range flagregistry.All {
		if f.Status == flagregistry.StatusDead {
			deadRemaining = append(deadRemaining, f.Name)
		}
	}
	if len(deadRemaining) != 0 {
		t.Errorf("RED: %d StatusDead rows remain in flagregistry.All — Builder must remove all dead rows.\n"+
			"Remaining: %v", len(deadRemaining), deadRemaining)
	}
}

// TestC2_003_RegistryRowCountIs264 verifies that after removing 18 dead flags
// from the 282-row registry, the row count is exactly 264.
//
// BEHAVIORAL: asserts len(flagregistry.All) == 264. The value 264 = 282 - 18
// (cross-verified by the cycle-1 and cycle-2 scouts). Over-removal fails this test.
//
// RED: len(flagregistry.All) is currently 282.
func TestC2_003_RegistryRowCountIs264(t *testing.T) {
	const want = 264
	if got := len(flagregistry.All); got != want {
		t.Errorf("RED: len(flagregistry.All) = %d, want %d.\n"+
			"Builder must remove exactly 18 StatusDead rows (282 → 264). "+
			"Over-removal or under-removal both fail this predicate.", got, want)
	}
}

// TestC2_004_DeadFlagsSweepGoneUnitTestPasses verifies that the regression test
// TestDeadFlagsSweep_Gone in go/internal/flagregistry/registry_deadflags_test.go
// (authored by TDD-engineer for Builder to make GREEN) passes after the sweep.
//
// BEHAVIORAL: runs `go test -run TestDeadFlagsSweep_Gone ./internal/flagregistry/`
// via SubprocessOutput — exercises the actual registry binary, not just source text.
//
// RED: TestDeadFlagsSweep_Gone currently fails because all 18 dead flags are still
// registered (Lookup returns ok=true for each, and the test asserts ok=false).
func TestC2_004_DeadFlagsSweepGoneUnitTestPasses(t *testing.T) {
	out, errOut, code, err := acsassert.SubprocessOutput(
		"go", "test",
		"-C", goDir(t),
		"-count=1",
		"-run", "TestDeadFlagsSweep_Gone",
		"./internal/flagregistry/",
	)
	combined := out + "\n" + errOut
	if code != 0 || err != nil {
		t.Errorf("RED: go test -run TestDeadFlagsSweep_Gone ./internal/flagregistry/ failed (exit=%d): %v\n"+
			"Builder must delete all 18 StatusDead rows from registry_table.go to make this GREEN.\n"+
			"Output:\n%s", code, err, combined)
	}
}

// TestC2_005_FlagreadersACSGuardPasses verifies that the flagreaders regression
// ACS guard exits 0 after the dead-flag sweep, confirming no live production
// reader references any of the 18 removed flags.
//
// BEHAVIORAL: runs the real go test binary against the flagreaders ACS package.
// Source edits alone cannot satisfy this — the guard scans the actual filesystem.
//
// RED: this test is expected pre-existing GREEN (the flagreaders guard passes
// because all 18 flags have 0 readers already). Marked RED-candidate: if Builder
// accidentally removes a flag that HAS a reader, this test will catch it.
func TestC2_005_FlagreadersACSGuardPasses(t *testing.T) {
	out, errOut, code, err := acsassert.SubprocessOutput(
		"go", "test",
		"-C", goDir(t),
		"-tags", "acs",
		"-count=1",
		"./acs/regression/flagreaders/...",
	)
	combined := out + "\n" + errOut
	if code != 0 || err != nil {
		t.Errorf("flagreaders ACS guard failed (exit=%d): %v\n"+
			"This means a removed flag still has a production reader.\n"+
			"Output:\n%s", code, err, combined)
	}
}
