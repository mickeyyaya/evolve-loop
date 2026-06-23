//go:build acs

// Package cycle353 materializes the cycle-353 acceptance criteria for the
// committed top_n task:
//
//	observer-flag-classify — promote 4 Observer cluster flags from
//	StatusInternal to StatusActive in the flagregistry and fix the stale
//	EVOLVE_OBSERVER_NUDGE_S default in runtime-reference.md.
//
// AC map (1:1 with scout-report.md ACs):
//
//	AC1/AC3/AC5  observer spot-checks pass in flagregistry test suite  → C353_001
//	AC4          NUDGE_S default in runtime-reference.md is 300        → C353_002
//	AC6(neg)     STALL_S is no longer StatusInternal in registry_table  → C353_003
//	AC2          evolve flags check exits 0                            → C353_004 (pre-existing GREEN)
//
// Floor binding (R9.3): observer-flag-classify is the sole committed top_n
// task. No predicates for deferred items (cycle-280 lesson).
package cycle353

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/pkg/acsassert"
)

// goDir returns the go module directory for use as -C in subprocess calls.
func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

// TestC353_001_ObserverSpotChecksPass verifies that the 4 Observer cluster
// flags (STALL_S, POLL_S, NUDGE_S, NUDGE_BODY) are all StatusActive in the
// flagregistry by running the existing TestLookup_SpotChecks suite, which
// Builder must extend with 4 new spot-check cases (one per flag).
//
// BEHAVIORAL: runs the actual go test binary against the flagregistry package;
// source-text addition alone cannot make it pass — the registry rows must have
// Status: StatusActive.
//
// RED: the 4 new spot-check cases added to registry_test.go by TDD-engineer
// fail because the current registry has StatusInternal for all 4 flags.
func TestC353_001_ObserverSpotChecksPass(t *testing.T) {
	dir := goDir(t)
	out, errOut, code, err := acsassert.SubprocessOutput(
		"go", "test",
		"-C", dir,
		"-count=1",
		"./internal/flagregistry/...",
		"-run", "TestLookup_SpotChecks",
	)
	combined := out + "\n" + errOut
	if code != 0 || err != nil {
		t.Errorf("RED: go test ./internal/flagregistry/... -run TestLookup_SpotChecks failed (exit=%d).\n"+
			"Builder must update registry_table.go: set Status=StatusActive for\n"+
			"  EVOLVE_OBSERVER_STALL_S, EVOLVE_OBSERVER_POLL_S,\n"+
			"  EVOLVE_OBSERVER_NUDGE_S, EVOLVE_OBSERVER_NUDGE_BODY.\n\nOutput:\n%s",
			code, combined)
	}
}

// TestC353_002_NudgeSDefaultIs300InRuntimeRef verifies that the
// EVOLVE_OBSERVER_NUDGE_S row in docs/operations/runtime-reference.md
// documents the default as 300 (opt-out via =0), not the stale "0 (opt-in)".
//
// BEHAVIORAL (via acs-predicate: config-check waiver): this is a doc-content
// correctness check. The default value in the doc is the observable contract
// operators read; "300" in the row is the criterion.
//
// RED: current runtime-reference.md line 77 has "`0` (opt-in)" which is
// wrong — observer.go defaults DefaultNudgeS=300 and cmd_phase_observer.go
// envchain.Int("EVOLVE_OBSERVER_NUDGE_S", nil, 300).
// acs-predicate: config-check
func TestC353_002_NudgeSDefaultIs300InRuntimeRef(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "docs", "operations", "runtime-reference.md")
	if !acsassert.FileMatchesRegex(t, path, `EVOLVE_OBSERVER_NUDGE_S[^|]*300`) {
		t.Errorf("RED: docs/operations/runtime-reference.md EVOLVE_OBSERVER_NUDGE_S row\n"+
			"does not contain '300'.\n"+
			"Builder must change the default column from '`0` (opt-in)' to\n"+
			"'`300` (opt-out via `=0`)' and update the description accordingly.\n"+
			"File: %s", path)
	}
}

// TestC353_003_ObserverStallSIsNotInternal is the NEGATIVE predicate (AC6):
// asserts that EVOLVE_OBSERVER_STALL_S is no longer annotated as StatusInternal
// in registry_table.go. Failure means the registry entry still reads
// "Status: StatusInternal" — the strongest anti-no-op signal for this task.
//
// Pair with C353_001: C353_001 verifies the flags ARE Active; C353_003 verifies
// they are NOT Internal. Together they prevent a vacuous implementation that
// just removes the flag entry without adding the Active row.
//
// RED: registry_table.go currently contains the string
// `EVOLVE_OBSERVER_STALL_S", Status: StatusInternal` because the flag was
// left as a placeholder from the 2026-06-11 inventory sweep.
// acs-predicate: config-check
func TestC353_003_ObserverStallSIsNotInternal(t *testing.T) {
	root := acsassert.RepoRoot(t)
	path := filepath.Join(root, "go", "internal", "flagregistry", "registry_table.go")
	// Assert that the StatusInternal placeholder for STALL_S is gone.
	// FileNotContains returns true when the substring is ABSENT (correct state).
	if !acsassert.FileNotContains(t, path, `EVOLVE_OBSERVER_STALL_S", Status: StatusInternal`) {
		t.Errorf("RED: registry_table.go still contains StatusInternal for EVOLVE_OBSERVER_STALL_S.\n"+
			"Builder must promote all 4 Observer flags to StatusActive.\n"+
			"File: %s", path)
	}
}

// TestC353_004_FlagsCheckExitsZero verifies that `evolve flags check` exits 0,
// meaning the generated flag index in control-flags.md is in sync with the
// flagregistry after Builder regenerates it with `evolve flags generate`.
//
// NOTE: this predicate is pre-existing GREEN (the index and registry are
// currently in sync, both reflecting StatusInternal). It becomes RED mid-fix
// (after registry update but before index regeneration), then GREEN again after
// Builder runs `evolve flags generate`. Included for completeness (AC2).
//
// The binary must be invoked from the repo root (not the package dir) so it
// finds docs/architecture/control-flags.md at the correct relative path.
func TestC353_004_FlagsCheckExitsZero(t *testing.T) {
	root := acsassert.RepoRoot(t)
	binPath := filepath.Join(root, "go", "bin", "evolve")
	// Run via bash cd to set cwd to repo root — SubprocessOutput inherits the
	// go test runner's package cwd which would be go/acs/cycle353/, not root.
	out, errOut, code, err := acsassert.SubprocessOutput(
		"bash", "-c", "cd "+root+" && "+binPath+" flags check",
	)
	combined := strings.TrimSpace(out + "\n" + errOut)
	if code != 0 || err != nil {
		t.Errorf("evolve flags check exited %d: %v\nOutput:\n%s\n"+
			"Builder must run `evolve flags generate` after updating registry_table.go\n"+
			"to regenerate the flag index in control-flags.md.",
			code, err, combined)
	}
}
