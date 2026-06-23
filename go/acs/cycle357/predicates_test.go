//go:build acs

// Package cycle357 materializes the cycle-357 acceptance criteria for the
// committed top_n task:
//
//   - dispatch-bridge-retirement — retire EVOLVE_DISPATCH_STOP_ON_FAIL and
//     EVOLVE_DISPATCH_VERIFY deprecated bridge flags from flagregistry,
//     bridge function, bridge test cases, and skills docs.
//
// AC map (1:1 with triage top_n, scout-report.md ACs):
//
//	dispatch-bridge-retirement:
//	  AC-1 (neg)  EVOLVE_DISPATCH_STOP_ON_FAIL absent from flagregistry.Lookup → C357_001
//	  AC-2 (neg)  EVOLVE_DISPATCH_VERIFY absent from flagregistry.Lookup        → C357_001
//	  AC-3 (neg)  bridge code absent from resolveDispatchPolicy + whole file    → C357_002 + C357_007
//	  AC-4        TestResolveDispatchPolicy: no legacy cases in test output      → C357_003
//	  AC-5 (neg)  bridge test cases absent from cmd_loop_m4_test.go             → C357_004
//	  AC-6 (neg)  EVOLVE_DISPATCH_VERIFY=0 absent from claude-runtime.md        → C357_005
//	  AC-7        evolve flags check exits 0 (no drift)                          → C357_006
//	  AC-8 (neg)  no non-test Go files reference either deprecated flag          → C357_007
//	  AC-9        EVOLVE_DISPATCH_POLICY preserved in resolveDispatchPolicy      → C357_002
//	  AC-10       go test ./internal/flagregistry/... all PASS                   → C357_008
//
// Floor binding (R9.3): predicates only for committed top_n task.
// Deferred tasks (EVOLVE_CHANNEL, EVOLVE_BYPASS_SHIP_VERIFY, etc.) get zero predicates.
package cycle357

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

// TestC357_001_DispatchBridgeFlagsAbsentFromLookup verifies that both
// deprecated dispatch bridge flags are no longer registered after Builder
// removes their rows from go/internal/flagregistry/registry_table.go.
//
// BEHAVIORAL: calls flagregistry.Lookup() directly (the production SSOT
// binary-search function). Source edits alone cannot satisfy this — the flag
// row must be physically absent for Lookup to return ok=false.
//
// NEGATIVE (AC-1, AC-2): before Builder's change both flags have StatusDeprecated
// and Lookup returns ok=true, so the test fails for both.
//
// RED: Lookup("EVOLVE_DISPATCH_STOP_ON_FAIL") and Lookup("EVOLVE_DISPATCH_VERIFY")
// currently return ok=true → assert !ok fails for both.
func TestC357_001_DispatchBridgeFlagsAbsentFromLookup(t *testing.T) {
	for _, name := range []string{"EVOLVE_DISPATCH_STOP_ON_FAIL", "EVOLVE_DISPATCH_VERIFY"} {
		if f, ok := flagregistry.Lookup(name); ok {
			t.Errorf("RED: flagregistry.Lookup(%q) returned (flag, true) — deprecated bridge flag still registered.\n"+
				"Builder must remove this row from go/internal/flagregistry/registry_table.go.\n"+
				"Current entry: Status=%q Cluster=%q Doc=%q",
				name, f.Status, f.Cluster, f.Doc)
		}
	}
}

// TestC357_003_ResolveDispatchPolicyNoLegacyCases verifies that the 4 bridge
// test cases (legacy STOP_ON_FAIL, legacy VERIFY, both-legacy+new, both-legacy)
// are absent from TestResolveDispatchPolicy output after Builder removes them
// from cmd_loop_m4_test.go.
//
// BEHAVIORAL: runs the actual go test binary and asserts on output content.
//
// NEGATIVE (AC-4): before Builder's change, go test -v output contains
// "STOP_ON_FAIL=1_bridges", "VERIFY=0_bridges", and "both_legacy" markers
// in subtest names. After Builder's change, none appear.
//
// RED: currently all 9 cases pass; legacy case subtest names appear in verbose
// output → pattern checks fail.
func TestC357_003_ResolveDispatchPolicyNoLegacyCases(t *testing.T) {
	dir := goDir(t)
	out, errOut, code, err := acsassert.SubprocessOutput(
		"go", "test",
		"-C", dir,
		"-count=1", "-v",
		"./cmd/evolve/...",
		"-run", "TestResolveDispatchPolicy",
	)
	combined := out + "\n" + errOut
	if code != 0 || err != nil {
		t.Fatalf("go test -run TestResolveDispatchPolicy failed (exit=%d): %v\nOutput:\n%s",
			code, err, combined)
	}
	for _, marker := range []string{"STOP_ON_FAIL=1_bridges", "VERIFY=0_bridges", "both_legacy"} {
		if strings.Contains(combined, marker) {
			t.Errorf("RED: go test output contains legacy bridge case marker %q — legacy cases still present.\n"+
				"Builder must remove the 4 legacy bridge test cases from cmd_loop_m4_test.go lines 103-106.",
				marker)
		}
	}
}

// TestC357_004_BridgeTestCasesAbsentFromM4Test verifies that both deprecated
// flag names are absent from cmd_loop_m4_test.go after Builder removes the
// 4 bridge test cases and 2 envKeys entries.
//
// acs-predicate: config-check
//
// RED: cmd_loop_m4_test.go currently references both deprecated flags in the
// test table (lines 103-106) and envKeys map (lines 131-132).
func TestC357_004_BridgeTestCasesAbsentFromM4Test(t *testing.T) {
	root := acsassert.RepoRoot(t)
	m4test := filepath.Join(root, "go", "cmd", "evolve", "cmd_loop_m4_test.go")
	for _, name := range []string{"EVOLVE_DISPATCH_STOP_ON_FAIL", "EVOLVE_DISPATCH_VERIFY"} {
		if !acsassert.FileNotContains(t, m4test, name) {
			t.Errorf("RED: cmd_loop_m4_test.go still references %q.\n"+
				"Builder must remove the bridge test cases (lines 103-106) and envKeys entries (lines 131-132).\n"+
				"File: %s", name, m4test)
		}
	}
}

// TestC357_005_ClaudeRuntimeDocNoBridgeFlagReference verifies that
// skills/loop/reference/claude-runtime.md no longer references
// EVOLVE_DISPATCH_VERIFY=0 after Builder updates line 72 to use
// EVOLVE_DISPATCH_POLICY=off.
//
// acs-predicate: config-check
//
// RED: claude-runtime.md:72 currently contains "EVOLVE_DISPATCH_VERIFY=0".
func TestC357_005_ClaudeRuntimeDocNoBridgeFlagReference(t *testing.T) {
	root := acsassert.RepoRoot(t)
	claudeRuntime := filepath.Join(root, "skills", "loop", "reference", "claude-runtime.md")
	if !acsassert.FileNotContains(t, claudeRuntime, "EVOLVE_DISPATCH_VERIFY=0") {
		t.Errorf("RED: skills/loop/reference/claude-runtime.md still contains EVOLVE_DISPATCH_VERIFY=0.\n"+
			"Builder must replace line 72 with EVOLVE_DISPATCH_POLICY=off.\n"+
			"File: %s", claudeRuntime)
	}
}

// TestC357_006_FlagsCheckExitsZero verifies that `evolve flags check` exits 0,
// confirming that docs/architecture/control-flags.md is in sync with the
// flagregistry after Builder removes the 2 deprecated rows and re-runs
// `evolve flags generate`.
//
// BEHAVIORAL: runs the real evolve binary against the worktree.
//
// RED confirmed: control-flags.md in the worktree is already stale vs
// flagregistry (cycle-356 drift). Builder must run `evolve flags generate`
// after removing the 2 deprecated registry rows to reach GREEN.
func TestC357_006_FlagsCheckExitsZero(t *testing.T) {
	root := acsassert.RepoRoot(t)
	binPath := filepath.Join(root, "go", "bin", "evolve")
	out, errOut, code, err := acsassert.SubprocessOutput(
		"bash", "-c", "cd "+root+" && "+binPath+" flags check",
	)
	combined := out + "\n" + errOut
	if code != 0 || err != nil {
		t.Errorf("evolve flags check exited %d: %v\nOutput:\n%s\n"+
			"Builder must run `evolve flags generate` after removing the 2 deprecated registry rows.",
			code, err, combined)
	}
}

// TestC357_007_NoProductionGoCodeReferencesBridgeFlags verifies that no
// non-test Go source files reference the deprecated bridge flag names after
// Builder removes the bridge function body, doc comment, and registry rows.
// Covers both AC-3 (whole-file absence) and AC-8 (no non-test reader).
//
// BEHAVIORAL: runs grep as a subprocess — file-system scan of all non-test
// Go files under go/. The ACS predicate file itself is a _test.go and is
// excluded by the grep -v filter.
//
// RED: cmd_loop_control.go (doc comment lines 34-35 + function body lines 56-65)
// and registry_table.go (lines 101-102) currently contain both flag names →
// grep finds matches → output is non-empty → test fails.
func TestC357_007_NoProductionGoCodeReferencesBridgeFlags(t *testing.T) {
	root := acsassert.RepoRoot(t)
	goSrc := filepath.Join(root, "go")
	for _, flag := range []string{"EVOLVE_DISPATCH_STOP_ON_FAIL", "EVOLVE_DISPATCH_VERIFY"} {
		out, _, _, _ := acsassert.SubprocessOutput("bash", "-c",
			`grep -rl "`+flag+`" "`+goSrc+`" --include="*.go" 2>/dev/null | grep -v "_test.go"; true`)
		if strings.TrimSpace(out) != "" {
			t.Errorf("RED: non-test Go files still reference deprecated bridge flag %q:\n%s\n"+
				"Builder must remove all production references (bridge function body, doc comment, registry row).",
				flag, strings.TrimSpace(out))
		}
	}
}

// TestC357_008_FlagRegistryTestsPass verifies that all flagregistry package
// tests pass after Builder removes the 2 deprecated registry rows.
//
// BEHAVIORAL: runs the actual go test binary against the flagregistry package.
//
// NOTE: pre-existing GREEN (all flagregistry tests pass with both deprecated
// rows currently present). Retained as a regression lock.
func TestC357_008_FlagRegistryTestsPass(t *testing.T) {
	dir := goDir(t)
	out, errOut, code, err := acsassert.SubprocessOutput(
		"go", "test",
		"-C", dir,
		"-count=1",
		"./internal/flagregistry/...",
	)
	combined := out + "\n" + errOut
	if code != 0 || err != nil {
		t.Errorf("RED: go test ./internal/flagregistry/... failed (exit=%d): %v\nOutput:\n%s",
			code, err, combined)
	}
}
