//go:build acs

// Package cycle358 materializes the cycle-358 acceptance criteria for the
// committed top_n task:
//
//   - channel-bridge-retirement — retire the EVOLVE_CHANNEL deprecated bridge
//     flag from flagregistry, simplify channel.Enabled() from 2-param to
//     1-param, remove all production readers in tmux_inject.go and
//     core_adapter.go, update 5 test files, and regenerate control-flags.md
//     (286 → 285 flags).
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	channel-bridge-retirement:
//	  AC-1  EVOLVE_CHANNEL absent from flagregistry.Lookup          → C358_001
//	  AC-2  explicitChannel param removed from Enabled()            → C358_002
//	  AC-3  no production Go reader of EVOLVE_CHANNEL               → C358_003
//	  AC-4  bridge files fully cleaned (subset of AC-3 surface)     → C358_003
//	  AC-5  channel tests pass (TestEnabled_UsesStageOnly)          → C358_004
//	  AC-6  bridge + observer tests pass                            → C358_005
//	  AC-7  flagregistry tests pass                                 → C358_006
//	  AC-8  shadow/off → channel off (covered by channel tests)     → C358_004
//	  AC-9  control-flags.md regenerated (EVOLVE_CHANNEL row gone)  → C358_007
//	  AC-10 ACS cycle-358 guard passes (meta — this file)           → this package
//
// Floor binding (R9.3): predicates only for committed top_n task.
// Deferred tasks (internal-flag-classification, test-seam-relocation, etc.) get zero predicates.
package cycle358

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolveloop/go/pkg/acsassert"
)

// goDir returns the go module directory for subprocess calls.
func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

// TestC358_001_ChannelFlagAbsentFromLookup verifies that EVOLVE_CHANNEL is no
// longer registered after Builder removes its row from registry_table.go.
//
// BEHAVIORAL: calls flagregistry.Lookup() directly — the production SSOT
// binary-search function. A source edit alone cannot satisfy this: the flag
// row must be physically absent for Lookup to return ok=false.
//
// NEGATIVE (AC-1): before Builder's change the flag has StatusDeprecated and
// Lookup returns ok=true, so the assert-!ok fails.
//
// RED: flagregistry.Lookup("EVOLVE_CHANNEL") currently returns (flag, true)
// at registry_table.go:70 — the deprecated row is still registered.
func TestC358_001_ChannelFlagAbsentFromLookup(t *testing.T) {
	if f, ok := flagregistry.Lookup("EVOLVE_CHANNEL"); ok {
		t.Errorf("RED: flagregistry.Lookup(\"EVOLVE_CHANNEL\") returned (flag, true) — deprecated bridge flag still registered.\n"+
			"Builder must remove this row from go/internal/flagregistry/registry_table.go.\n"+
			"Current entry: Status=%q Cluster=%q RemoveIn=%q Doc=%q",
			f.Status, f.Cluster, f.RemoveIn, f.Doc)
	}
}

// TestC358_002_EnabledFunctionNoExplicitChannelParam verifies that the
// explicitChannel parameter and deprecated bool return have been removed from
// channel.Enabled() (AC-2), and that the channel package still compiles and
// its tests pass (behavioral companion).
//
// acs-predicate: source-structure
//
// The assertion "Enabled no longer has an explicitChannel parameter" is
// inherently structural — it verifies the API contract of the function. A
// pure subprocess test cannot make this RED because the existing 2-param tests
// all pass today. CountInGoFunc is the correct tool: it asserts a property of
// the function declaration itself (signature + body), and the behavioral
// companion (go test ./internal/bridge/channel/...) ensures the simplified
// function also satisfies its test suite post-refactor.
//
// RED: CountInGoFunc currently returns ≥1 because "explicitChannel" and
// "deprecated bool" appear in the function declaration at enablement.go:45.
func TestC358_002_EnabledFunctionNoExplicitChannelParam(t *testing.T) {
	root := acsassert.RepoRoot(t)
	enablement := filepath.Join(root, "go", "internal", "bridge", "channel", "enablement.go")

	// Primary assertion: explicitChannel and deprecated bool absent from Enabled.
	count, err := acsassert.CountInGoFunc(enablement, "Enabled", "explicitChannel", "deprecated bool")
	if err != nil {
		t.Fatalf("CountInGoFunc(Enabled, explicit+deprecated): %v", err)
	}
	if count != 0 {
		t.Errorf("RED: channel.Enabled still has %d line(s) containing \"explicitChannel\" or \"deprecated bool\".\n"+
			"Builder must simplify the signature to: func Enabled(stage string) bool { return stage == \"enforce\" }\n"+
			"File: %s", count, enablement)
	}

	// Behavioral companion: channel package tests must compile and pass.
	// Pre-existing GREEN today; regression lock for post-refactor correctness.
	out, errOut, code, err := acsassert.SubprocessOutput(
		"go", "test",
		"-C", goDir(t),
		"-count=1",
		"./internal/bridge/channel/...",
	)
	combined := out + "\n" + errOut
	if code != 0 || err != nil {
		t.Errorf("RED: go test ./internal/bridge/channel/... failed (exit=%d): %v\nOutput:\n%s",
			code, err, combined)
	}
}

// TestC358_003_NoProductionGoReferencesChannelFlag verifies that no non-test
// Go source files reference EVOLVE_CHANNEL after Builder removes the bridge
// lookup from tmux_inject.go and core_adapter.go. Covers both AC-3 (all
// non-test Go) and AC-4 (specific bridge files).
//
// BEHAVIORAL: runs grep as a subprocess — filesystem scan of all non-test
// Go files under go/. The ACS predicate file itself is a _test.go and is
// excluded by the grep filter.
//
// RED: tmux_inject.go (lines 20,33,37,40) and core_adapter.go (line 136,140,142)
// currently contain EVOLVE_CHANNEL references → grep finds matches → non-empty output.
func TestC358_003_NoProductionGoReferencesChannelFlag(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goSrc := filepath.Join(root, "go")
	out, _, _, _ := acsassert.SubprocessOutput("bash", "-c",
		`grep -rl "EVOLVE_CHANNEL" "`+goSrc+`" --include="*.go" 2>/dev/null | grep -v "_test.go"; true`)
	if strings.TrimSpace(out) != "" {
		t.Errorf("RED: non-test Go files still reference deprecated bridge flag EVOLVE_CHANNEL:\n%s\n"+
			"Builder must remove all production references:\n"+
			"  - go/internal/bridge/tmux_inject.go: remove lookupEnv call + WARN block\n"+
			"  - go/internal/adapters/observer/core_adapter.go: remove envGet arg + WARN block\n"+
			"  - go/internal/bridge/channel/enablement.go: simplify func (doc comment + body)",
			strings.TrimSpace(out))
	}
}

// TestC358_004_ChannelTestsPass verifies that the channel package tests pass
// after Builder rewrites TestEnabled_FoldsFlagIntoStage → TestEnabled_UsesStageOnly
// with 3 cases (shadow→off, off→off, enforce→on). Covers AC-5 and AC-8.
//
// BEHAVIORAL: runs the actual go test binary against channel package.
//
// Pre-existing GREEN in current state (TestEnabled_FoldsFlagIntoStage all pass).
// After Builder rewrites the test file and simplifies Enabled(), the renamed test
// must also pass. A compile failure after the signature change would cause RED here.
//
// NOTE: pre-existing GREEN — retained as regression lock; ensures that Builder's
// signature simplification + test rewrite leave the package in a passing state.
func TestC358_004_ChannelTestsPass(t *testing.T) {
	out, errOut, code, err := acsassert.SubprocessOutput(
		"go", "test",
		"-C", goDir(t),
		"-count=1",
		"./internal/bridge/channel/...",
	)
	combined := out + "\n" + errOut
	if code != 0 || err != nil {
		t.Errorf("go test ./internal/bridge/channel/... failed (exit=%d): %v\nOutput:\n%s",
			code, err, combined)
	}
}

// TestC358_005_BridgeAndObserverTestsPass verifies that the full bridge and
// observer adapter test suites pass after Builder swaps EVOLVE_CHANNEL env
// vars and updates callers. Covers AC-6.
//
// BEHAVIORAL: runs the actual go test binary.
//
// Pre-existing GREEN in current state. After Builder changes the Enabled()
// signature, tests in driver_tmux_repl_panelive_test.go and
// channel_e2e_test.go that use EVOLVE_CHANNEL: "1" will fail to compile
// (if not updated by Builder). This predicate catches that failure.
func TestC358_005_BridgeAndObserverTestsPass(t *testing.T) {
	out, errOut, code, err := acsassert.SubprocessOutput(
		"go", "test",
		"-C", goDir(t),
		"-count=1",
		"./internal/bridge/...",
		"./internal/adapters/observer/...",
	)
	combined := out + "\n" + errOut
	if code != 0 || err != nil {
		t.Errorf("go test ./internal/bridge/... ./internal/adapters/observer/... failed (exit=%d): %v\nOutput:\n%s",
			code, err, combined)
	}
}

// TestC358_006_FlagRegistryTestsPass verifies that the flagregistry package
// tests pass after Builder removes the EVOLVE_CHANNEL deprecated row and
// its corresponding test table entry in registry_test.go. Covers AC-7.
//
// BEHAVIORAL: runs the actual go test binary.
//
// Pre-existing GREEN in current state. After Builder removes the registry
// row, the registry_test.go table entry {"EVOLVE_CHANNEL", StatusDeprecated}
// must also be removed — otherwise the test fails (expects a status that the
// registry no longer carries).
func TestC358_006_FlagRegistryTestsPass(t *testing.T) {
	out, errOut, code, err := acsassert.SubprocessOutput(
		"go", "test",
		"-C", goDir(t),
		"-count=1",
		"./internal/flagregistry/...",
	)
	combined := out + "\n" + errOut
	if code != 0 || err != nil {
		t.Errorf("go test ./internal/flagregistry/... failed (exit=%d): %v\nOutput:\n%s",
			code, err, combined)
	}
}

// TestC358_007_ControlFlagsDocNoChannelRow verifies that the dedicated
// EVOLVE_CHANNEL table row has been removed from docs/architecture/control-flags.md
// after Builder runs `evolve flags generate`. Covers AC-9.
//
// acs-predicate: config-check
//
// The pattern "| `EVOLVE_CHANNEL` |" uniquely identifies the EVOLVE_CHANNEL
// table rows (there are two: one in the full flag table, one in the deprecated
// section). It does NOT match the EVOLVE_PHASE_RECOVERY description, which
// mentions EVOLVE_CHANNEL inline but does not start a row with that name.
//
// The worktree root (EVOLVE_WORKTREE_ROOT) is the correct path for this
// generated-doc check per the dual-root pattern (ACS README §generated-from-source).
// acsassert.RepoRoot(t) resolves to the worktree when running in the worktree.
//
// RED: control-flags.md currently contains
// "| `EVOLVE_CHANNEL` | **DEPRECATED**" and
// "| `EVOLVE_CHANNEL` | deprecated |" (4 occurrences total) because
// the flag row has not been removed yet.
func TestC358_007_ControlFlagsDocNoChannelRow(t *testing.T) {
	root := acsassert.RepoRoot(t)
	controlFlags := filepath.Join(root, "docs", "architecture", "control-flags.md")

	if !acsassert.FileExists(t, controlFlags) {
		t.Fatalf("docs/architecture/control-flags.md missing — cannot verify AC-9")
	}
	// The dedicated EVOLVE_CHANNEL row pattern: starts with the flag name as
	// the first column of a markdown table row.
	if !acsassert.FileNotContains(t, controlFlags, "| `EVOLVE_CHANNEL` |") {
		t.Errorf("RED: docs/architecture/control-flags.md still contains the EVOLVE_CHANNEL table row.\n"+
			"Builder must run `evolve flags generate` after removing the registry entry to drop this row.\n"+
			"File: %s", controlFlags)
	}
}
